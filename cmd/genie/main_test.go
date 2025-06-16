package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandExists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}
}

func TestRootCommandExecute(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{})
	
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	
	output := buf.String()
	if !strings.Contains(output, "Genie CLI tool") {
		t.Errorf("Expected output to contain 'Genie CLI tool', got: %s", output)
	}
}

func TestVersionFlag(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--version"})
	
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() with --version failed: %v", err)
	}
	
	output := buf.String()
	if !strings.Contains(output, "genie version") {
		t.Errorf("Expected output to contain 'genie version', got: %s", output)
	}
}
