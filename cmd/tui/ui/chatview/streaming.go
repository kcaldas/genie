package chatview

import "strings"

// minStableBytes is the smallest prefix worth rendering separately; below
// it the whole message is re-rendered each flush.
const minStableBytes = 256

// splitStable divides streaming markdown into a prefix that will not
// change shape as more text arrives and the tail that may. The split is at
// the last paragraph break that is not inside a fenced code block, so the
// prefix renders the same on its own as it will in the finished message.
// A long run without paragraph breaks (a big list, say) falls back to the
// last line break instead of re-rendering everything on every chunk.
func splitStable(content string) (stable, tail string) {
	if stable, tail, ok := splitAt(content, "\n\n"); ok {
		return stable, tail
	}
	if stable, tail, ok := splitAt(content, "\n"); ok {
		return stable, tail
	}
	return "", content
}

// splitAt finds the last sep outside any fence with at least
// minStableBytes before it.
func splitAt(content, sep string) (stable, tail string, ok bool) {
	cut := len(content)
	for {
		idx := strings.LastIndex(content[:cut], sep)
		if idx < minStableBytes {
			return "", "", false
		}
		if !insideFence(content[:idx]) {
			return content[:idx], content[idx+len(sep):], true
		}
		cut = idx
	}
}

// insideFence reports whether text ends inside an unclosed ``` or ~~~
// block. Fence markers are counted at line starts only.
func insideFence(text string) bool {
	open := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimLeft(line, " ")
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			open = !open
		}
	}
	return open
}
