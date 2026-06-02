package tools

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

func TestTaskToolStartIsAsyncAndCompletionCallbackFires(t *testing.T) {
	release := make(chan struct{})
	completed := make(chan TaskSnapshot, 1)

	tool := NewTaskTool(nil,
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error) {
			reporter.Log("child task started")
			select {
			case <-release:
				return TaskResult{Output: "final answer"}, nil
			case <-ctx.Done():
				return TaskResult{Error: ctx.Err().Error()}, ctx.Err()
			}
		})),
		WithTaskCompletionHandler(func(snapshot TaskSnapshot) {
			completed <- snapshot
		}),
	)

	result, err := tool.Handler()(context.Background(), map[string]any{
		"action":     "start",
		"summary":    "Summarize recent owner conversations",
		"prompt":     "Read the relevant conversations and return a concise summary.",
		"timeout_ms": float64((30 * time.Second) / time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Task start failed: %v", err)
	}

	taskID, _ := result["task_id"].(string)
	if taskID == "" {
		t.Fatalf("Task start did not return a task id: %#v", result)
	}
	if result["status"] != string(TaskStatusRunning) {
		t.Fatalf("Task status = %v, want running", result["status"])
	}

	status, err := tool.Handler()(context.Background(), map[string]any{
		"action":  "status",
		"task_id": taskID,
	})
	if err != nil {
		t.Fatalf("Task status failed: %v", err)
	}
	if status["status"] != string(TaskStatusRunning) {
		t.Fatalf("Task status = %v, want running", status["status"])
	}

	close(release)

	var snapshot TaskSnapshot
	select {
	case snapshot = <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for task completion callback")
	}

	if snapshot.TaskID != taskID {
		t.Fatalf("Completion task id = %s, want %s", snapshot.TaskID, taskID)
	}
	if snapshot.Status != TaskStatusCompleted {
		t.Fatalf("Completion status = %s, want completed", snapshot.Status)
	}
	if snapshot.Result != "final answer" {
		t.Fatalf("Completion result = %q, want final answer", snapshot.Result)
	}
	if len(snapshot.Logs) != 1 || snapshot.Logs[0] != "child task started" {
		t.Fatalf("Completion logs = %#v", snapshot.Logs)
	}
}

func TestTaskToolCancelRequestsCancellation(t *testing.T) {
	completed := make(chan TaskSnapshot, 1)

	tool := NewTaskTool(nil,
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error) {
			<-ctx.Done()
			return TaskResult{Error: ctx.Err().Error()}, ctx.Err()
		})),
		WithTaskCompletionHandler(func(snapshot TaskSnapshot) {
			completed <- snapshot
		}),
	)

	result, err := tool.Handler()(context.Background(), map[string]any{
		"action":     "start",
		"summary":    "Cancelable background task",
		"prompt":     "Wait until cancelled.",
		"timeout_ms": float64((30 * time.Second) / time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Task start failed: %v", err)
	}
	taskID := result["task_id"].(string)

	_, err = tool.Handler()(context.Background(), map[string]any{
		"action":  "cancel",
		"task_id": taskID,
	})
	if err != nil {
		t.Fatalf("Task cancel failed: %v", err)
	}

	select {
	case snapshot := <-completed:
		if snapshot.Status != TaskStatusCancelled {
			t.Fatalf("Completion status = %s, want cancelled", snapshot.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for cancellation")
	}
}

func TestTaskToolDuplicateStartsReuseExistingTask(t *testing.T) {
	release := make(chan struct{})
	var runs int32

	tool := NewTaskTool(nil,
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error) {
			atomic.AddInt32(&runs, 1)
			select {
			case <-release:
				return TaskResult{Output: "done"}, nil
			case <-ctx.Done():
				return TaskResult{Error: ctx.Err().Error()}, ctx.Err()
			}
		})),
	)
	defer close(release)

	params := map[string]any{
		"action":     "start",
		"summary":    "Summarize recent owner conversations",
		"prompt":     "Read the relevant conversations and return a concise summary.",
		"timeout_ms": float64((30 * time.Second) / time.Millisecond),
	}
	first, err := tool.Handler()(context.Background(), params)
	if err != nil {
		t.Fatalf("First task start failed: %v", err)
	}
	second, err := tool.Handler()(context.Background(), map[string]any{
		"action":     "start",
		"summary":    "  summarize   recent OWNER conversations ",
		"prompt":     "Read the relevant conversations and return a concise summary.",
		"timeout_ms": float64((30 * time.Second) / time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Second task start failed: %v", err)
	}

	if first["task_id"] == "" || first["task_id"] != second["task_id"] {
		t.Fatalf("Duplicate task ids = %v and %v, want same non-empty id", first["task_id"], second["task_id"])
	}
	waitForTaskRuns(t, &runs, 1)
	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("Executor runs = %d, want 1", got)
	}
}

func TestTaskManagerWithoutExecutorFailsTaskInertly(t *testing.T) {
	completed := make(chan TaskSnapshot, 1)
	manager := NewTaskManager(
		WithTaskCompletionHandler(func(snapshot TaskSnapshot) {
			completed <- snapshot
		}),
	)

	snapshot, err := manager.Start(TaskRequest{
		Summary: "Unconfigured executor test",
		Prompt:  "This task should fail because no executor is configured.",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Task start failed: %v", err)
	}
	if snapshot.Status != TaskStatusRunning {
		t.Fatalf("Start status = %s, want running", snapshot.Status)
	}

	select {
	case done := <-completed:
		if done.Status != TaskStatusFailed {
			t.Fatalf("Completion status = %s, want failed", done.Status)
		}
		if !strings.Contains(done.Error, "not configured") {
			t.Fatalf("Completion error = %q, want not configured", done.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for task completion")
	}
}

func waitForTaskRuns(t *testing.T, runs *int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(runs) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Timed out waiting for executor runs >= %d; got %d", want, atomic.LoadInt32(runs))
}

func TestTaskDeclarationArraySchemasHaveItems(t *testing.T) {
	decl := NewTaskTool(nil).Declaration()
	if decl.Response == nil {
		t.Fatal("Task response schema is nil")
	}

	for _, name := range []string{"logs", "tasks"} {
		schema := decl.Response.Properties[name]
		if schema == nil {
			t.Fatalf("Missing response schema for %s", name)
		}
		if schema.Type != ai.TypeArray {
			t.Fatalf("%s schema type = %v, want array", name, schema.Type)
		}
		if schema.Items == nil {
			t.Fatalf("%s schema is missing Items", name)
		}
	}
}

func TestTaskManagerClampsOutput(t *testing.T) {
	completed := make(chan TaskSnapshot, 1)
	manager := NewTaskManager(
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, request TaskRequest, reporter TaskReporter) (TaskResult, error) {
			return TaskResult{Output: strings.Repeat("x", 100)}, nil
		})),
		WithTaskCompletionHandler(func(snapshot TaskSnapshot) {
			completed <- snapshot
		}),
	)

	snapshot, err := manager.Start(TaskRequest{
		Summary:        "Clamp output test",
		Prompt:         "Return long output.",
		Timeout:        30 * time.Second,
		MaxOutputChars: 25,
	})
	if err != nil {
		t.Fatalf("Task start failed: %v", err)
	}
	if snapshot.Status != TaskStatusRunning {
		t.Fatalf("Start status = %s, want running", snapshot.Status)
	}

	select {
	case done := <-completed:
		if !done.OutputTruncated {
			t.Fatal("Expected output to be truncated")
		}
		if len(done.Result) <= 25 {
			t.Fatalf("Expected truncation marker, got %q", done.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for completion")
	}
}
