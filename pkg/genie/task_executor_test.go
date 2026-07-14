package genie_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/kcaldas/genie/pkg/tools"
)

func TestStartWiresNativeTaskExecutor(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	session := fixture.StartAndGetSession(genie.WithChatHistory(genie.ChatHistoryTurn{
		User:      "Earlier parent-only question",
		Assistant: "Earlier parent-only answer",
	}))
	taskTool := taskToolFromFixture(t, fixture)

	if !taskTool.HasConfiguredExecutor() {
		t.Fatal("Task tool should have a configured native executor after Start")
	}

	childPrompt := genie.NativeTaskPromptForTest("Inspect the repository and summarize what matters.")
	fixture.ExpectSimpleMessage(childPrompt, "child task result")

	taskCtx := genie.ApplySessionContextForTest(context.Background(), session)
	start, err := taskTool.Handler()(taskCtx, map[string]any{
		"action":     "start",
		"summary":    "Inspect repository",
		"prompt":     "Inspect the repository and summarize what matters.",
		"timeout_ms": float64((30 * time.Second) / time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Task start failed: %v", err)
	}

	taskID, _ := start["task_id"].(string)
	if taskID == "" {
		t.Fatalf("Task start did not return a task id: %#v", start)
	}

	done := waitForTaskStatus(t, taskTool, taskID, 2*time.Second)
	if done["status"] != string(tools.TaskStatusCompleted) {
		t.Fatalf("Task status = %v, want completed; result=%#v", done["status"], done)
	}
	if done["result"] != "child task result" {
		t.Fatalf("Task result = %q, want child task result", done["result"])
	}

	captured := fixture.MockPromptRunner.CapturedData()
	if len(captured) == 0 {
		t.Fatal("Expected child prompt data to be captured")
	}
	chat := captured[len(captured)-1]["chat"]
	if strings.Contains(chat, "Earlier parent-only") {
		t.Fatalf("Child task inherited parent chat history: %q", chat)
	}
}

func TestNativeTaskChildRegistryOmitsTask(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	session := fixture.StartAndGetSession()

	child, _, err := genie.NewChildGenieForTest(fixture.Genie)
	if err != nil {
		t.Fatalf("newChildGenie failed: %v", err)
	}

	workspace := session.GetWorkingDirectory()
	if _, err := child.Start(&workspace, nil); err != nil {
		t.Fatalf("child.Start failed: %v", err)
	}
	registry, err := child.GetToolsRegistry()
	if err != nil {
		t.Fatalf("child.GetToolsRegistry failed: %v", err)
	}
	if _, exists := registry.Get("Task"); exists {
		t.Fatal("child registry should not include Task")
	}
	if _, exists := registry.Get("readFile"); !exists {
		t.Fatal("child registry should still include regular tools")
	}
}

func taskToolFromFixture(t *testing.T, fixture *genietest.TestFixture) *tools.TaskTool {
	t.Helper()

	registry, err := fixture.Genie.GetToolsRegistry()
	if err != nil {
		t.Fatalf("GetToolsRegistry failed: %v", err)
	}
	tool, exists := registry.Get("Task")
	if !exists {
		t.Fatal("Task tool not registered")
	}
	taskTool, ok := tool.(*tools.TaskTool)
	if !ok {
		t.Fatalf("Task tool type = %T, want *tools.TaskTool", tool)
	}
	return taskTool
}

func waitForTaskStatus(t *testing.T, taskTool *tools.TaskTool, taskID string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := taskTool.Handler()(context.Background(), map[string]any{
			"action":  "status",
			"task_id": taskID,
		})
		if err != nil {
			t.Fatalf("Task status failed: %v", err)
		}
		status, _ := result["status"].(string)
		if status != string(tools.TaskStatusRunning) {
			return result
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Timed out waiting for task %s", taskID)
	return nil
}
