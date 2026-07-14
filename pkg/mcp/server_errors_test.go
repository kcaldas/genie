package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRecordsServerConnectErrors(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `{
  "mcpServers": {
    "broken": {
      "command": "definitely-not-a-real-binary-xyz",
      "args": []
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	client := NewClient(nil)
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("Init must not fail on a broken server, got: %v", err)
	}

	errs := client.ServerErrors()
	if len(errs) != 1 {
		t.Fatalf("Expected 1 server error, got %d: %v", len(errs), errs)
	}
	msg, ok := errs["broken"]
	if !ok {
		t.Fatalf("Expected error recorded for server %q, got %v", "broken", errs)
	}
	if !strings.Contains(msg, "definitely-not-a-real-binary-xyz") {
		t.Errorf("Expected error message to name the missing binary, got %q", msg)
	}

	if len(client.GetTools()) != 0 {
		t.Errorf("Expected no tools from a broken server, got %d", len(client.GetTools()))
	}
}

func TestInitNoConfigNoServerErrors(t *testing.T) {
	client := NewClient(nil)
	if err := client.Init(t.TempDir()); err != nil {
		t.Fatalf("Init without config must succeed, got: %v", err)
	}
	if errs := client.ServerErrors(); len(errs) != 0 {
		t.Fatalf("Expected no server errors without config, got %v", errs)
	}
}
