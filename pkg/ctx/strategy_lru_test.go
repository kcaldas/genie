package ctx

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func formatFileEntry(f FileEntry) string {
	return fmt.Sprintf("File: %s\n```\n%s\n```", f.Path, f.Content)
}

func TestLRUStrategy_Name(t *testing.T) {
	s := NewLRUStrategy(10)
	assert.Equal(t, "lru", s.Name())
}

func TestLRUStrategy_EmptyItems(t *testing.T) {
	s := NewLRUStrategy(10)

	kept, tokens := s.ApplyToCollection(nil, 1000, formatFileEntry)

	assert.Nil(t, kept)
	assert.Equal(t, 0, tokens)
}

func TestLRUStrategy_ZeroBudget(t *testing.T) {
	s := NewLRUStrategy(10)
	items := []FileEntry{{Path: "a.go", Content: "package a"}}

	kept, tokens := s.ApplyToCollection(items, 0, formatFileEntry)

	assert.Nil(t, kept)
	assert.Equal(t, 0, tokens)
}

func TestLRUStrategy_AllFitWithinLimit(t *testing.T) {
	s := NewLRUStrategy(10)
	items := []FileEntry{
		{Path: "a.go", Content: "package a"},
		{Path: "b.go", Content: "package b"},
	}

	kept, tokens := s.ApplyToCollection(items, 10000, formatFileEntry)

	assert.Equal(t, 2, len(kept))
	assert.Equal(t, "a.go", kept[0].Path)
	assert.Equal(t, "b.go", kept[1].Path)
	assert.Greater(t, tokens, 0)
}

func TestLRUStrategy_ItemLimitEnforced(t *testing.T) {
	s := NewLRUStrategy(2)
	items := []FileEntry{
		{Path: "a.go", Content: "package a"},
		{Path: "b.go", Content: "package b"},
		{Path: "c.go", Content: "package c"},
		{Path: "d.go", Content: "package d"},
	}

	kept, _ := s.ApplyToCollection(items, 100000, formatFileEntry)

	// Should only keep first 2 (most recent)
	assert.Equal(t, 2, len(kept))
	assert.Equal(t, "a.go", kept[0].Path)
	assert.Equal(t, "b.go", kept[1].Path)
}

func TestLRUStrategy_BudgetLimitEnforced(t *testing.T) {
	s := NewLRUStrategy(0) // no item limit
	items := []FileEntry{
		{Path: "a.go", Content: "package a"},
		{Path: "b.go", Content: "package b"},
		{Path: "c.go", Content: strings.Repeat("x", 10000)},
	}

	// Budget for roughly 2 small files
	singleTokens := EstimateTokens(formatFileEntry(items[0]))
	budget := singleTokens * 2

	kept, tokens := s.ApplyToCollection(items, budget, formatFileEntry)

	assert.Equal(t, 2, len(kept))
	assert.LessOrEqual(t, tokens, budget)
}

func TestLRUStrategy_SingleLargeItemExceedsBudget(t *testing.T) {
	s := NewLRUStrategy(10)
	items := []FileEntry{
		{Path: "huge.go", Content: strings.Repeat("x", 10000)},
	}

	kept, tokens := s.ApplyToCollection(items, 10, formatFileEntry)

	assert.Nil(t, kept)
	assert.Equal(t, 0, tokens)
}

func TestLRUStrategy_NoItemLimit(t *testing.T) {
	s := NewLRUStrategy(0) // 0 means no item limit
	items := make([]FileEntry, 50)
	for i := range items {
		items[i] = FileEntry{Path: fmt.Sprintf("file%d.go", i), Content: "x"}
	}

	kept, _ := s.ApplyToCollection(items, 100000, formatFileEntry)

	// All 50 should be kept since there's no item limit and budget is huge
	assert.Equal(t, 50, len(kept))
}

func TestLRUStrategy_PreservesOrder(t *testing.T) {
	s := NewLRUStrategy(3)
	items := []FileEntry{
		{Path: "most-recent.go", Content: "1"},
		{Path: "middle.go", Content: "2"},
		{Path: "oldest.go", Content: "3"},
		{Path: "dropped.go", Content: "4"},
	}

	kept, _ := s.ApplyToCollection(items, 100000, formatFileEntry)

	assert.Equal(t, 3, len(kept))
	assert.Equal(t, "most-recent.go", kept[0].Path)
	assert.Equal(t, "middle.go", kept[1].Path)
	assert.Equal(t, "oldest.go", kept[2].Path)
}
