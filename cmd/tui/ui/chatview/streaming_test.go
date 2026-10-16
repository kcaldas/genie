package chatview

import (
	"strings"
	"testing"
)

func TestSplitStableAtParagraphBreak(t *testing.T) {
	para := strings.Repeat("words ", 60) + "end"
	content := para + "\n\n" + para + "\n\nstreaming tail"
	stable, tail := splitStable(content)
	if stable != para+"\n\n"+para || tail != "streaming tail" {
		t.Fatalf("stable=%q... tail=%q", stable[:20], tail)
	}
}

func TestSplitStableFallsBackToLineBreaks(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		sb.WriteString("- list item number ")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteByte('\n')
	}
	sb.WriteString("- partial")
	stable, tail := splitStable(sb.String())
	if tail != "- partial" || !strings.HasSuffix(stable, "xxxxxxxxxx") {
		t.Fatalf("line-break fallback: tail=%q stable ends %q", tail, stable[len(stable)-12:])
	}
}

func TestSplitStableShortContentIsAllTail(t *testing.T) {
	stable, tail := splitStable("short\n\ntext")
	if stable != "" || tail != "short\n\ntext" {
		t.Fatalf("stable=%q tail=%q", stable, tail)
	}
}

func TestSplitStableNeverCutsInsideAFence(t *testing.T) {
	intro := strings.Repeat("intro text ", 30)
	content := intro + "\n\n```go\nfunc a() {}\n\nfunc b() {}\n\nfunc c() {"
	stable, tail := splitStable(content)
	if stable != intro {
		t.Fatalf("stable should stop before the fence, got %q", stable)
	}
	if !strings.HasPrefix(tail, "```go") {
		t.Fatalf("tail must start at the fence, got %q", tail)
	}
}

func TestSplitStableAfterClosedFence(t *testing.T) {
	intro := strings.Repeat("x ", 150)
	content := intro + "\n\n```\ncode\n```\n\nmore"
	stable, tail := splitStable(content)
	if !strings.HasSuffix(stable, "```") || tail != "more" {
		t.Fatalf("closed fence should be stable: stable ends %q tail=%q", stable[len(stable)-5:], tail)
	}
}

func TestInsideFence(t *testing.T) {
	if insideFence("a\n```\nb") != true || insideFence("a\n```\nb\n```") != false {
		t.Fatal("fence parity wrong")
	}
	if insideFence("  ~~~\nx") != true {
		t.Fatal("indented tilde fence not detected")
	}
	if insideFence("text with ``` inline") != false {
		t.Fatal("inline backticks are not a fence")
	}
}
