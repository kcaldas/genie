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

// keepDotDirs are dot-directories worth referencing: the tool's own
// configuration and CI.
var keepDotDirs = map[string]bool{".genie": true, ".github": true, ".claude": true}

// List returns workspace-relative file paths under root, sorted, skipping
// VCS and dependency directories, dot-directories other than keepDotDirs,
// and anything the root .gitignore excludes. It stops after limit entries
// (DefaultLimit if 0) and reports whether it did.
func List(root string, limit int) (paths []string, truncated bool) {
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
			hidden := strings.HasPrefix(name, ".") && !keepDotDirs[name]
			if skipDirs[name] || hidden || ignored(rel+"/", ignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignored(rel, ignore) {
			return nil
		}
		if len(out) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, truncated
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
