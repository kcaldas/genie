package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

const (
	defaultTaskTimeout        = 10 * time.Minute
	minTaskTimeout            = 5 * time.Second
	maxTaskTimeout            = 30 * time.Minute
	defaultTaskMaxOutputChars = 60000
	maxTaskMaxOutputChars     = 200000
	defaultTaskMaxLogEntries  = 200
	defaultMaxConcurrentTasks = 4
	defaultTaskDedupeWindow   = 2 * time.Minute
)

var errTaskExecutorNotConfigured = errors.New("task executor is not configured")

// TaskStatus is the lifecycle state for an async Task invocation.
type TaskStatus string

const (
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusTimedOut  TaskStatus = "timed_out"
)

// TaskRequest is the immutable input passed to a task executor.
type TaskRequest struct {
	TaskID         string
	Summary        string
	Prompt         string
	Workspace      string
	Persona        string
	Timeout        time.Duration
	MaxOutputChars int
	CreatedAt      time.Time
	Metadata       map[string]string
}

// TaskResult is the executor's final output.
type TaskResult struct {
	Output          string
	Error           string
	OutputTruncated bool
}

// TaskSnapshot is a point-in-time view of a task.
type TaskSnapshot struct {
	TaskID          string
	Summary         string
	Status          TaskStatus
	StartedAt       time.Time
	CompletedAt     time.Time
	Timeout         time.Duration
	Result          string
	Error           string
	OutputTruncated bool
	Logs            []string
}

// TaskReporter lets an executor append bounded diagnostic messages.
type TaskReporter interface {
	Log(message string)
}

// TaskExecutor runs the task body. The task manager owns lifecycle,
// cancellation, status transitions, and completion callbacks.
type TaskExecutor interface {
	RunTask(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error)
}

// TaskExecutorFunc adapts a function to TaskExecutor.
type TaskExecutorFunc func(context.Context, TaskRequest, TaskReporter) (TaskResult, error)

func (f TaskExecutorFunc) RunTask(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error) {
	return f(ctx, request, reporter)
}

type unconfiguredTaskExecutor struct{}

func newUnconfiguredTaskExecutor() TaskExecutor {
	return unconfiguredTaskExecutor{}
}

func (unconfiguredTaskExecutor) RunTask(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error) {
	return TaskResult{Error: errTaskExecutorNotConfigured.Error()}, errTaskExecutorNotConfigured
}

// TaskCompletionHandler observes terminal task snapshots.
type TaskCompletionHandler func(TaskSnapshot)

// TaskManagerOption configures a TaskManager.
type TaskManagerOption func(*TaskManager)

// WithTaskExecutor configures the executor used by a task manager or registry.
func WithTaskExecutor(executor TaskExecutor) TaskManagerOption {
	return func(manager *TaskManager) {
		if executor != nil {
			manager.executor = executor
		}
	}
}

// WithTaskCompletionHandler configures a terminal task callback.
func WithTaskCompletionHandler(handler TaskCompletionHandler) TaskManagerOption {
	return func(manager *TaskManager) {
		manager.onComplete = handler
	}
}

// WithTaskLimits configures manager execution limits.
func WithTaskLimits(maxConcurrent int, maxLogEntries int) TaskManagerOption {
	return func(manager *TaskManager) {
		if maxConcurrent > 0 {
			manager.maxConcurrent = maxConcurrent
		}
		if maxLogEntries > 0 {
			manager.maxLogEntries = maxLogEntries
		}
	}
}

// TaskManager owns async task execution and bounded task state.
type TaskManager struct {
	executor      TaskExecutor
	onComplete    TaskCompletionHandler
	maxConcurrent int
	maxLogEntries int
	dedupeWindow  time.Duration

	mu          sync.RWMutex
	tasks       map[string]*taskRecord
	dedupeIndex map[string]string
	active      int
}

type taskRecord struct {
	request TaskRequest
	dedupe  string
	status  TaskStatus
	result  string
	err     string
	trunc   bool
	logs    []string
	doneAt  time.Time
	cancel  context.CancelFunc
}

type taskReporter struct {
	manager *TaskManager
	taskID  string
}

func (r taskReporter) Log(message string) {
	r.manager.appendLog(r.taskID, message)
}

var taskIDCounter uint64

// NewTaskManager creates a task manager.
func NewTaskManager(options ...TaskManagerOption) *TaskManager {
	manager := &TaskManager{
		executor:      newUnconfiguredTaskExecutor(),
		maxConcurrent: defaultMaxConcurrentTasks,
		maxLogEntries: defaultTaskMaxLogEntries,
		dedupeWindow:  defaultTaskDedupeWindow,
		tasks:         make(map[string]*taskRecord),
		dedupeIndex:   make(map[string]string),
	}
	for _, option := range options {
		if option != nil {
			option(manager)
		}
	}
	return manager
}

// SetExecutorIfUnconfigured installs an executor only when the manager still
// has its inert fallback. Hosts can use this to wire a native default while
// preserving explicit WithTaskExecutor overrides.
func (m *TaskManager) SetExecutorIfUnconfigured(executor TaskExecutor) bool {
	if executor == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !isUnconfiguredTaskExecutor(m.executor) {
		return false
	}
	m.executor = executor
	return true
}

// HasConfiguredExecutor reports whether Task has a real executor installed.
func (m *TaskManager) HasConfiguredExecutor() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !isUnconfiguredTaskExecutor(m.executor)
}

// Start begins a task and returns immediately.
func (m *TaskManager) Start(request TaskRequest) (TaskSnapshot, error) {
	if strings.TrimSpace(request.Summary) == "" {
		return TaskSnapshot{}, fmt.Errorf("summary is required")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return TaskSnapshot{}, fmt.Errorf("prompt is required")
	}
	if request.Timeout <= 0 {
		request.Timeout = defaultTaskTimeout
	}
	if request.Timeout < minTaskTimeout {
		return TaskSnapshot{}, fmt.Errorf("timeout must be at least %s", minTaskTimeout)
	}
	if request.Timeout > maxTaskTimeout {
		return TaskSnapshot{}, fmt.Errorf("timeout must be at most %s", maxTaskTimeout)
	}
	if request.MaxOutputChars <= 0 {
		request.MaxOutputChars = defaultTaskMaxOutputChars
	}
	if request.MaxOutputChars > maxTaskMaxOutputChars {
		return TaskSnapshot{}, fmt.Errorf("max_output_chars must be at most %d", maxTaskMaxOutputChars)
	}
	if request.CreatedAt.IsZero() {
		request.CreatedAt = time.Now()
	}
	if request.TaskID == "" {
		request.TaskID = nextTaskID()
	}
	dedupe := taskDedupeKey(request)

	ctx, cancel := context.WithTimeout(context.Background(), request.Timeout)

	record := &taskRecord{
		request: request,
		dedupe:  dedupe,
		status:  TaskStatusRunning,
		cancel:  cancel,
	}

	m.mu.Lock()
	if existingID := m.dedupeIndex[dedupe]; existingID != "" {
		if existing := m.tasks[existingID]; existing != nil && !m.dedupeExpired(existing) {
			snapshot := snapshotFor(existing)
			m.mu.Unlock()
			cancel()
			return snapshot, nil
		}
		delete(m.dedupeIndex, dedupe)
	}
	if m.maxConcurrent > 0 && m.active >= m.maxConcurrent {
		m.mu.Unlock()
		cancel()
		return TaskSnapshot{}, fmt.Errorf("too many active tasks (%d max)", m.maxConcurrent)
	}
	if _, exists := m.tasks[request.TaskID]; exists {
		m.mu.Unlock()
		cancel()
		return TaskSnapshot{}, fmt.Errorf("task id already exists: %s", request.TaskID)
	}
	m.tasks[request.TaskID] = record
	m.dedupeIndex[dedupe] = request.TaskID
	m.active++
	snapshot := snapshotFor(record)
	m.mu.Unlock()

	go m.run(ctx, request, taskReporter{manager: m, taskID: request.TaskID})

	return snapshot, nil
}

// Get returns a task snapshot by id.
func (m *TaskManager) Get(taskID string) (TaskSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return TaskSnapshot{}, false
	}
	return snapshotFor(record), true
}

// List returns all known task snapshots.
func (m *TaskManager) List() []TaskSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshots := make([]TaskSnapshot, 0, len(m.tasks))
	for _, record := range m.tasks {
		snapshots = append(snapshots, snapshotFor(record))
	}
	return snapshots
}

// Cancel requests cancellation for a running task.
func (m *TaskManager) Cancel(taskID string) (TaskSnapshot, bool) {
	m.mu.RLock()
	record, ok := m.tasks[taskID]
	if !ok {
		m.mu.RUnlock()
		return TaskSnapshot{}, false
	}
	cancel := record.cancel
	m.mu.RUnlock()

	if cancel != nil {
		cancel()
	}
	snapshot, _ := m.Get(taskID)
	return snapshot, true
}

func (m *TaskManager) run(ctx context.Context, request TaskRequest, reporter TaskReporter) {
	result, err := m.executor.RunTask(ctx, request, reporter)
	status := TaskStatusCompleted
	errText := strings.TrimSpace(result.Error)

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		status = TaskStatusTimedOut
		if errText == "" {
			errText = fmt.Sprintf("task timed out after %s", request.Timeout.Round(time.Second))
		}
	case ctx.Err() == context.Canceled:
		status = TaskStatusCancelled
		if errText == "" {
			errText = "task was cancelled"
		}
	case err != nil:
		status = TaskStatusFailed
		if errText == "" {
			errText = err.Error()
		}
	case errText != "":
		status = TaskStatusFailed
	}

	output, truncated := clampText(result.Output, request.MaxOutputChars)
	if result.OutputTruncated {
		truncated = true
	}

	m.mu.Lock()
	record := m.tasks[request.TaskID]
	if record != nil {
		record.status = status
		record.result = output
		record.err = errText
		record.trunc = truncated
		record.doneAt = time.Now()
		record.cancel = nil
	}
	if m.active > 0 {
		m.active--
	}
	var snapshot TaskSnapshot
	if record != nil {
		snapshot = snapshotFor(record)
	}
	onComplete := m.onComplete
	m.mu.Unlock()

	if onComplete != nil && snapshot.TaskID != "" {
		onComplete(snapshot)
	}
}

func (m *TaskManager) dedupeExpired(record *taskRecord) bool {
	if m.dedupeWindow <= 0 {
		return false
	}
	if record.status == TaskStatusRunning {
		return false
	}
	completed := record.doneAt
	if completed.IsZero() {
		completed = record.request.CreatedAt
	}
	return time.Since(completed) > m.dedupeWindow
}

func (m *TaskManager) appendLog(taskID string, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.tasks[taskID]
	if !ok {
		return
	}
	record.logs = append(record.logs, message)
	if m.maxLogEntries > 0 && len(record.logs) > m.maxLogEntries {
		record.logs = record.logs[len(record.logs)-m.maxLogEntries:]
	}
}

func snapshotFor(record *taskRecord) TaskSnapshot {
	logs := make([]string, len(record.logs))
	copy(logs, record.logs)
	return TaskSnapshot{
		TaskID:          record.request.TaskID,
		Summary:         record.request.Summary,
		Status:          record.status,
		StartedAt:       record.request.CreatedAt,
		CompletedAt:     record.doneAt,
		Timeout:         record.request.Timeout,
		Result:          record.result,
		Error:           record.err,
		OutputTruncated: record.trunc,
		Logs:            logs,
	}
}

func nextTaskID() string {
	n := atomic.AddUint64(&taskIDCounter, 1)
	return fmt.Sprintf("task_%x_%d", time.Now().UnixNano(), n)
}

func taskDedupeKey(request TaskRequest) string {
	normalized := strings.Join([]string{
		normalizeTaskDedupeText(request.Summary),
		normalizeTaskDedupeText(request.Prompt),
		strings.TrimSpace(request.Workspace),
		strings.TrimSpace(request.Persona),
	}, "\n")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func normalizeTaskDedupeText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func isUnconfiguredTaskExecutor(executor TaskExecutor) bool {
	_, ok := executor.(unconfiguredTaskExecutor)
	return ok
}

// TaskTool starts and manages isolated async task sessions.
type TaskTool struct {
	publisher events.Publisher
	manager   *TaskManager
}

// NewTaskTool creates a new task tool.
func NewTaskTool(publisher events.Publisher, options ...TaskManagerOption) Tool {
	return &TaskTool{
		publisher: publisher,
		manager:   NewTaskManager(options...),
	}
}

// SetExecutorIfUnconfigured installs an executor only when no explicit
// executor was configured for this Task tool.
func (t *TaskTool) SetExecutorIfUnconfigured(executor TaskExecutor) bool {
	if t == nil || t.manager == nil {
		return false
	}
	return t.manager.SetExecutorIfUnconfigured(executor)
}

// HasConfiguredExecutor reports whether this Task tool has a real executor.
func (t *TaskTool) HasConfiguredExecutor() bool {
	if t == nil || t.manager == nil {
		return false
	}
	return t.manager.HasConfiguredExecutor()
}

func (t *TaskTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "Task",
		Description: "Start and inspect isolated async Genie tasks for complex research, analysis, summarization, or multi-step work without loading the full working context into the current conversation. Requires a configured task executor. Use start to launch work, then status/list/log/cancel to inspect it.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for async task management",
			Properties: map[string]*ai.Schema{
				"action": {
					Type:        ai.TypeString,
					Description: "Task action: start, status, list, log, or cancel. Defaults to start.",
				},
				"summary": {
					Type:        ai.TypeString,
					Description: "Brief user-visible summary of what the task will accomplish. Required for start.",
					MinLength:   10,
					MaxLength:   240,
				},
				"prompt": {
					Type:        ai.TypeString,
					Description: "Detailed instructions for the isolated task. Be specific about what to inspect, which tools to use, and the desired response format. Required for start.",
					MinLength:   10,
					MaxLength:   16000,
				},
				"task_id": {
					Type:        ai.TypeString,
					Description: "Task id for status, log, or cancel.",
				},
				"timeout_ms": {
					Type:        ai.TypeInteger,
					Description: "Optional timeout in milliseconds for start. Default is 10 minutes; max is 30 minutes.",
					Minimum:     float64(minTaskTimeout / time.Millisecond),
					Maximum:     float64(maxTaskTimeout / time.Millisecond),
				},
				"max_output_chars": {
					Type:        ai.TypeInteger,
					Description: "Maximum result characters retained for this task. Default is 60000; max is 200000.",
					Minimum:     1000,
					Maximum:     maxTaskMaxOutputChars,
				},
			},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Async task state or result",
			Properties: map[string]*ai.Schema{
				"success":          {Type: ai.TypeBoolean, Description: "Whether the requested task action succeeded"},
				"action":           {Type: ai.TypeString, Description: "Action that was executed"},
				"task_id":          {Type: ai.TypeString, Description: "Task id"},
				"status":           {Type: ai.TypeString, Description: "Task status"},
				"summary":          {Type: ai.TypeString, Description: "Task summary"},
				"message":          {Type: ai.TypeString, Description: "Short status message"},
				"result":           {Type: ai.TypeString, Description: "Task result when completed"},
				"error":            {Type: ai.TypeString, Description: "Task error when failed"},
				"output_truncated": {Type: ai.TypeBoolean, Description: "Whether the result was truncated"},
				"logs": {
					Type:        ai.TypeArray,
					Description: "Recent task log messages",
					Items:       &ai.Schema{Type: ai.TypeString},
				},
				"tasks": {
					Type:        ai.TypeArray,
					Description: "Known tasks",
					Items:       taskSnapshotSchema(),
				},
			},
			Required: []string{"success", "action"},
		},
	}
}

func (t *TaskTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		action := strings.ToLower(strings.TrimSpace(stringParam(params, "action")))
		if action == "" {
			action = "start"
		}

		switch action {
		case "start":
			return t.handleStart(ctx, params)
		case "status":
			return t.handleStatus(params)
		case "list":
			return t.handleList()
		case "log":
			return t.handleLog(params)
		case "cancel":
			return t.handleCancel(params)
		default:
			return nil, fmt.Errorf("unsupported task action: %s", action)
		}
	}
}

func (t *TaskTool) handleStart(ctx context.Context, params map[string]any) (map[string]any, error) {
	summary := strings.TrimSpace(stringParam(params, "summary"))
	if summary == "" {
		return nil, fmt.Errorf("summary parameter is required for start")
	}
	prompt := strings.TrimSpace(stringParam(params, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("prompt parameter is required for start")
	}

	timeout := durationParam(params, "timeout_ms", defaultTaskTimeout)
	maxOutput := intParam(params, "max_output_chars", defaultTaskMaxOutputChars)
	workspace, err := taskWorkspaceFromParams(ctx, params)
	if err != nil {
		return nil, err
	}

	request := TaskRequest{
		Summary:        summary,
		Prompt:         prompt,
		Workspace:      workspace,
		Persona:        contextString(ctx, "persona", stringParam(params, "persona")),
		Timeout:        timeout,
		MaxOutputChars: maxOutput,
		CreatedAt:      time.Now(),
	}

	snapshot, err := t.manager.Start(request)
	if err != nil {
		return nil, err
	}

	if t.publisher != nil {
		startMessage := fmt.Sprintf("Starting background task %s: %s", snapshot.TaskID, summary)
		t.publisher.Publish(events.ToolCallMessageEvent{}.Topic(), events.ToolCallMessageEvent{
			ToolName: "Task",
			Message:  startMessage,
		})
		t.publisher.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
			Message:     startMessage,
			Role:        "assistant",
			ContentType: "text",
		})
	}

	return map[string]any{
		"success": true,
		"action":  "start",
		"task_id": snapshot.TaskID,
		"status":  string(snapshot.Status),
		"summary": snapshot.Summary,
		"message": fmt.Sprintf("Background task %s started. I will report back when it completes.", snapshot.TaskID),
	}, nil
}

func taskWorkspaceFromParams(ctx context.Context, params map[string]any) (string, error) {
	workspace := strings.TrimSpace(stringParam(params, "workspace"))
	if workspace == "" {
		return WorkingDirectoryFromContext(ctx), nil
	}

	resolved, valid := ResolvePathWithWorkingDirectory(ctx, workspace)
	if !valid {
		return "", FormatPathOutsideWorkspaceError(ctx, workspace)
	}
	if err := CheckPathPolicy(ctx, resolved, IntentRead); err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("workspace %q is not accessible: %w", workspace, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory", workspace)
	}
	return resolved, nil
}

func (t *TaskTool) handleStatus(params map[string]any) (map[string]any, error) {
	taskID := strings.TrimSpace(stringParam(params, "task_id"))
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required for status")
	}
	snapshot, ok := t.manager.Get(taskID)
	if !ok {
		return map[string]any{"success": false, "action": "status", "task_id": taskID, "error": "task not found"}, nil
	}
	return snapshotResult("status", snapshot, true), nil
}

func (t *TaskTool) handleList() (map[string]any, error) {
	snapshots := t.manager.List()
	tasks := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		tasks = append(tasks, compactSnapshotResult(snapshot))
	}
	return map[string]any{
		"success": true,
		"action":  "list",
		"tasks":   tasks,
	}, nil
}

func (t *TaskTool) handleLog(params map[string]any) (map[string]any, error) {
	taskID := strings.TrimSpace(stringParam(params, "task_id"))
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required for log")
	}
	snapshot, ok := t.manager.Get(taskID)
	if !ok {
		return map[string]any{"success": false, "action": "log", "task_id": taskID, "error": "task not found"}, nil
	}
	result := snapshotResult("log", snapshot, true)
	result["logs"] = snapshot.Logs
	return result, nil
}

func (t *TaskTool) handleCancel(params map[string]any) (map[string]any, error) {
	taskID := strings.TrimSpace(stringParam(params, "task_id"))
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required for cancel")
	}
	snapshot, ok := t.manager.Cancel(taskID)
	if !ok {
		return map[string]any{"success": false, "action": "cancel", "task_id": taskID, "error": "task not found"}, nil
	}
	result := snapshotResult("cancel", snapshot, true)
	result["message"] = fmt.Sprintf("Cancellation requested for task %s.", taskID)
	return result, nil
}

func (t *TaskTool) FormatOutput(result map[string]interface{}) string {
	success, _ := result["success"].(bool)
	action, _ := result["action"].(string)
	taskID, _ := result["task_id"].(string)
	status, _ := result["status"].(string)
	message, _ := result["message"].(string)
	errText, _ := result["error"].(string)
	output, _ := result["result"].(string)

	if !success {
		if errText != "" {
			return fmt.Sprintf("**Task %s failed:** %s", action, errText)
		}
		return fmt.Sprintf("**Task %s failed**", action)
	}
	if message != "" {
		return message
	}
	if action == "list" {
		tasks, _ := result["tasks"].([]map[string]any)
		return fmt.Sprintf("**Tasks:** %d known", len(tasks))
	}
	if output != "" {
		return fmt.Sprintf("**Task %s (%s)**\n\n%s", taskID, status, output)
	}
	if taskID != "" {
		return fmt.Sprintf("**Task %s:** %s", taskID, status)
	}
	return "**Task action completed**"
}

func snapshotResult(action string, snapshot TaskSnapshot, includeResult bool) map[string]any {
	result := compactSnapshotResult(snapshot)
	result["success"] = true
	result["action"] = action
	if includeResult {
		result["result"] = snapshot.Result
		result["error"] = snapshot.Error
		result["output_truncated"] = snapshot.OutputTruncated
	}
	return result
}

func compactSnapshotResult(snapshot TaskSnapshot) map[string]any {
	result := map[string]any{
		"task_id": snapshot.TaskID,
		"summary": snapshot.Summary,
		"status":  string(snapshot.Status),
	}
	if !snapshot.StartedAt.IsZero() {
		result["started_at"] = snapshot.StartedAt.Format(time.RFC3339)
	}
	if !snapshot.CompletedAt.IsZero() {
		result["completed_at"] = snapshot.CompletedAt.Format(time.RFC3339)
	}
	return result
}

func taskSnapshotSchema() *ai.Schema {
	return &ai.Schema{
		Type: ai.TypeObject,
		Properties: map[string]*ai.Schema{
			"task_id":      {Type: ai.TypeString},
			"summary":      {Type: ai.TypeString},
			"status":       {Type: ai.TypeString},
			"started_at":   {Type: ai.TypeString},
			"completed_at": {Type: ai.TypeString},
		},
	}
}

func contextString(ctx context.Context, key string, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	value := ctx.Value(key)
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func stringParam(params map[string]any, key string) string {
	value, ok := params[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func intParam(params map[string]any, key string, fallback int) int {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return parsed
		}
	}
	return fallback
}

func durationParam(params map[string]any, key string, fallback time.Duration) time.Duration {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}
	var millis int64
	switch typed := value.(type) {
	case int:
		millis = int64(typed)
	case int64:
		millis = typed
	case float64:
		millis = int64(typed)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return fallback
		}
		millis = parsed
	default:
		return fallback
	}
	if millis <= 0 {
		return fallback
	}
	return time.Duration(millis) * time.Millisecond
}

func clampText(text string, maxChars int) (string, bool) {
	if maxChars <= 0 || len(text) <= maxChars {
		return text, false
	}
	if maxChars <= 20 {
		return text[:maxChars], true
	}
	return text[:maxChars] + "\n\n[output truncated]", true
}
