package files

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListSkipsNoiseAndHonoursGitignore(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"main.go", "cmd/app/app.go", "docs/readme.md", ".genie/personas/p.yaml", ".github/workflows/ci.yml",
		".git/HEAD", "node_modules/x/index.js", ".hidden/secret", "vendor/lib.go",
		"build/out.bin", "logs/a.log", "coverage.out", "tmp/scratch.txt",
	} {
		write(t, root, f)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# comment\n*.log\ncoverage.out\n/tmp/\n!keep.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, truncated := List(root, 0)
	want := []string{".genie/personas/p.yaml", ".github/workflows/ci.yml", ".gitignore", "cmd/app/app.go", "docs/readme.md", "main.go"}
	if !reflect.DeepEqual(got, want) || truncated {
		t.Fatalf("List = %v (truncated=%v), want %v", got, truncated, want)
	}
}

func TestListRespectsLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		write(t, root, filepath.Join("d", string(rune('a'+i))+".txt"))
	}
	if got, truncated := List(root, 5); len(got) != 5 || !truncated {
		t.Fatalf("limit: %d paths, truncated=%v", len(got), truncated)
	}
}

func TestListMissingRootIsEmpty(t *testing.T) {
	if got, _ := List(filepath.Join(t.TempDir(), "nope"), 0); len(got) != 0 {
		t.Fatalf("missing root = %v", got)
	}
}
