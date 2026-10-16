package chatview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// countingFormatter renders "role: content" and counts calls, so tests can
// assert what the cache saved.
type countingFormatter struct{ calls int }

func (f *countingFormatter) FormatMessageWithWidth(msg types.Message, _ int) string {
	f.calls++
	return msg.Role + ": " + msg.Content + "\n\n"
}

func newModel(t *testing.T, maxMsgs int) (*Model, *state.ChatState, *countingFormatter) {
	t.Helper()
	chat := state.NewChatState(maxMsgs)
	f := &countingFormatter{}
	m := New(chat, f)
	m.SetSize(40, 5)
	return m, chat, f
}

func flush(m *Model) { m.Update(flushMsg{}) }

func TestSyncRendersEachMessageOnce(t *testing.T) {
	m, chat, f := newModel(t, 100)
	chat.AddMessage(types.Message{Role: "user", Content: "hi"})
	chat.AddMessage(types.Message{Role: "assistant", Content: "hello"})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if f.calls != 2 {
		t.Fatalf("renders = %d, want 2", f.calls)
	}
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if f.calls != 2 {
		t.Fatalf("unchanged messages re-rendered: %d", f.calls)
	}
	if got := ansi.Strip(m.View()); got != "user: hi\n\nassistant: hello\n" {
		t.Fatalf("view = %q", got)
	}
}

func TestStreamingReRendersOnlyTheLiveMessage(t *testing.T) {
	m, chat, f := newModel(t, 100)
	chat.AddMessage(types.Message{Role: "user", Content: "q"})
	id := chat.AddMessage(types.Message{Role: "assistant", Content: "a"})
	m.Update(uimsg.ChatStreaming{ID: id})
	flush(m)
	base := f.calls
	for i := 0; i < 5; i++ {
		chat.UpdateMessageByID(id, func(msg *types.Message) { msg.Content += "b" })
		m.Update(uimsg.ChatStreaming{ID: id})
		flush(m)
	}
	if f.calls != base+5 {
		t.Fatalf("renders during streaming = %d, want %d (one per flush, live only)", f.calls-base, 5)
	}
	// Completion: same content, no longer live, rendered once more.
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if f.calls != base+6 {
		t.Fatalf("completion renders = %d, want 1", f.calls-base-5)
	}
	if m.cache.size() > 2*2 {
		t.Fatalf("cache kept %d renderings for 2 messages", m.cache.size())
	}
}

func TestChunkBurstCoalescesIntoOneFlush(t *testing.T) {
	m, chat, _ := newModel(t, 100)
	id := chat.AddMessage(types.Message{Role: "assistant", Content: "x"})
	var cmds int
	for i := 0; i < 50; i++ {
		if cmd := m.Update(uimsg.ChatStreaming{ID: id}); cmd != nil {
			cmds++
		}
	}
	if cmds != 1 {
		t.Fatalf("flush ticks scheduled = %d, want 1", cmds)
	}
	flush(m)
	if cmd := m.Update(uimsg.ChatStreaming{ID: id}); cmd == nil {
		t.Fatal("after a flush the next chunk must arm a new tick")
	}
}

func TestFollowTailOnlyWhenAtBottom(t *testing.T) {
	m, chat, _ := newModel(t, 100)
	for i := 0; i < 10; i++ {
		chat.AddMessage(types.Message{Role: "user", Content: fmt.Sprint(i)})
	}
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if !m.Following() || m.Offset() != m.LineCount()-5 {
		t.Fatalf("should follow the tail: following=%v offset=%d total=%d", m.Following(), m.Offset(), m.LineCount())
	}
	m.ScrollBy(-3)
	if m.Following() {
		t.Fatal("scrolling up must stop following")
	}
	off := m.Offset()
	chat.AddMessage(types.Message{Role: "user", Content: "new"})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if m.Offset() != off {
		t.Fatalf("offset moved while scrolled up: %d -> %d", off, m.Offset())
	}
	m.Bottom()
	if !m.Following() || !strings.Contains(ansi.Strip(m.View()), "new") {
		t.Fatalf("Bottom must resume following and show the new line: %q", m.View())
	}
	m.Top()
	if m.Offset() != 0 || m.Following() {
		t.Fatal("Top must go to line 0 and stop following")
	}
	m.PageDown()
	if m.Offset() != 5 {
		t.Fatalf("PageDown offset = %d, want 5", m.Offset())
	}
}

func TestResizeReusesCachePerWidth(t *testing.T) {
	m, chat, f := newModel(t, 100)
	chat.AddMessage(types.Message{Role: "user", Content: "hi"})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	m.SetSize(30, 5)
	m.SetSize(40, 5)
	m.SetSize(30, 5)
	if f.calls != 2 {
		t.Fatalf("renders across 40/30/40/30 widths = %d, want 2", f.calls)
	}
}

func TestEvictedMessagesLeaveTheCache(t *testing.T) {
	m, chat, _ := newModel(t, 3)
	for i := 0; i < 6; i++ {
		chat.AddMessage(types.Message{Role: "user", Content: fmt.Sprint(i)})
		m.Update(uimsg.ChatChanged{})
		flush(m)
	}
	if n := len(m.cache.byID); n != 3 {
		t.Fatalf("cache holds %d messages, state holds 3", n)
	}
}

func TestLongLinesAreHardWrapped(t *testing.T) {
	m, chat, _ := newModel(t, 10)
	chat.AddMessage(types.Message{Role: "user", Content: strings.Repeat("x", 100)})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	for _, l := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(l) > 40 {
			t.Fatalf("line wider than view: %q", l)
		}
	}
}

func TestThemeChangeInvalidatesCache(t *testing.T) {
	m, chat, f := newModel(t, 10)
	chat.AddMessage(types.Message{Role: "user", Content: "hi"})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	g := &countingFormatter{}
	m.SetFormatter(g)
	if g.calls != 1 || f.calls != 1 {
		t.Fatalf("new formatter calls=%d old=%d", g.calls, f.calls)
	}
}

func TestViewBeforeSizeIsEmpty(t *testing.T) {
	chat := state.NewChatState(10)
	chat.AddMessage(types.Message{Role: "user", Content: "hi"})
	m := New(chat, &countingFormatter{})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if m.View() != "" {
		t.Fatal("unsized view must render nothing")
	}
}

// BenchmarkStreamingFlush measures one coalesced re-render of a long
// streaming reply through the real glamour formatter.
func BenchmarkStreamingFlush(b *testing.B) {
	chat := state.NewChatState(100)
	cfg := &types.Config{Theme: "default", GlamourTheme: "auto", MarkdownRendering: "enabled", WrapMessages: "enabled", UserLabel: "○", AssistantLabel: "●"}
	f, err := NewFormatter(cfg, &types.Theme{})
	if err != nil {
		b.Fatal(err)
	}
	m := New(chat, f)
	m.SetSize(100, 40)
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb, "line %d of a long streamed reply with `code` and **bold**\n", i)
	}
	id := chat.AddMessage(types.Message{Role: "assistant", Content: sb.String(), ContentType: "markdown"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chat.UpdateMessageByID(id, func(msg *types.Message) { msg.Content += "." })
		m.Update(uimsg.ChatStreaming{ID: id})
		flush(m)
	}
}

// TestStreamingFlushBudget guards the incremental rendering: appending to
// a 5k-line markdown reply must not re-render the whole reply.
func TestStreamingFlushBudget(t *testing.T) {
	chat := state.NewChatState(100)
	cfg := &types.Config{Theme: "default", GlamourTheme: "auto", MarkdownRendering: "enabled", WrapMessages: "enabled", UserLabel: "○", AssistantLabel: "●"}
	f, err := NewFormatter(cfg, &types.Theme{})
	if err != nil {
		t.Fatal(err)
	}
	m := New(chat, f)
	m.SetSize(100, 40)
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb, "line %d of a long streamed reply with `code` and **bold**\n\n", i)
	}
	id := chat.AddMessage(types.Message{Role: "assistant", Content: sb.String(), ContentType: "markdown"})
	m.Update(uimsg.ChatStreaming{ID: id})
	flush(m) // pays for the stable prefix once
	start := time.Now()
	const flushes = 10
	for i := 0; i < flushes; i++ {
		chat.UpdateMessageByID(id, func(msg *types.Message) { msg.Content += "." })
		m.Update(uimsg.ChatStreaming{ID: id})
		flush(m)
	}
	if per := time.Since(start) / flushes; per > 40*time.Millisecond {
		t.Fatalf("streaming flush took %v, budget 40ms (whole-message render is ~100ms)", per)
	}
}

func TestViewIsCheap(t *testing.T) {
	m, chat, _ := newModel(t, 1000)
	for i := 0; i < 500; i++ {
		chat.AddMessage(types.Message{Role: "user", Content: strings.Repeat("word ", 30)})
	}
	m.Update(uimsg.ChatChanged{})
	flush(m)
	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = m.View()
	}
	if per := time.Since(start) / 100; per > 2*time.Millisecond {
		t.Fatalf("View took %v per frame, budget 2ms", per)
	}
}

var _ tea.Msg = flushMsg{}
