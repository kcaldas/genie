// Package files lists the workspace files the @ picker offers.
package files

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kcaldas/genie/pkg/tools"
)

// DefaultLimit caps how many paths List returns, so a huge tree still
// opens the picker quickly.
const DefaultLimit = 5000

// skipDirs are never descended into, whatever .gitignore says.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".idea": true, ".vscode": true, "__pycache__": true, ".next": true,
}

// List returns workspace-relative file paths under root, sorted, skipping
// VCS and dependency directories, dot-directories, and anything the root
// .gitignore excludes. It stops after limit entries (DefaultLimit if 0).
func List(root string, limit int) []string {
	if limit <= 0 {
		limit = DefaultLimit
	}
	ignore := gitignorePatterns(root)
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if d.IsDir() {
			if skipDirs[name] || strings.HasPrefix(name, ".") || ignored(rel+"/", ignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignored(rel, ignore) {
			return nil
		}
		out = append(out, rel)
		if len(out) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// gitignorePatterns reads the simple (non-negated) patterns from the root
// .gitignore. Negations and nested ignore files are not honoured; the
// picker only needs to keep obvious noise out.
func gitignorePatterns(root string) []string {
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var patterns []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		patterns = append(patterns, strings.TrimPrefix(line, "/"))
	}
	return patterns
}

// ignored reports whether rel (with a trailing slash for directories)
// matches any pattern.
func ignored(rel string, patterns []string) bool {
	for _, p := range patterns {
		dirOnly := strings.HasSuffix(p, "/")
		p = strings.TrimSuffix(p, "/")
		isDir := strings.HasSuffix(rel, "/")
		target := strings.TrimSuffix(rel, "/")
		if dirOnly && !isDir {
			continue
		}
		if tools.MatchGlob(p, target) || strings.HasPrefix(target, p+"/") {
			return true
		}
	}
	return false
}
