package component

import (
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/gdamore/tcell/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/shell"
)

type InputComponent struct {
	*BaseComponent
	commandEventBus *events.CommandEventBus
	history         history.ChatHistory
	clipboard       *helpers.Clipboard
	lastContent     string
	shellEditor     shell.Shell
	completer       *shell.Completer
}

func NewInputComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, clipboard *helpers.Clipboard, historyManager history.ChatHistory, commandSuggester *shell.CommandSuggester, slashCommandSuggester *shell.SlashCommandSuggester) *InputComponent {
	completer := shell.NewCompleter()

	shellEditor := shell.NewBasicShell(completer, historyManager)

	ctx := &InputComponent{
		BaseComponent:   NewBaseComponent("input", "input", gui, configManager),
		commandEventBus: commandEventBus,
		history:         historyManager,
		clipboard:       clipboard,
		shellEditor:     shellEditor,
		completer:       completer,
	}

	if err := ctx.LoadHistory(); err != nil {
		// Don't fail startup if history loading fails
	}

	ctx.SetTitle("")
	ctx.SetWindowProperties(types.WindowProperties{
		Focusable:  true,
		Editable:   true,
		Wrap:       false,
		Autoscroll: true,
		Highlight:  false,
		Frame:      true,
		Subtitle:   "F4/Ctrl+V Expand",
	})

	ctx.SetOnFocus(func() error {
		if v := ctx.GetView(); v != nil {
			//v.SelFgColor = gocui.ColorBlack
		}
		return nil
	})

	ctx.SetOnFocusLost(func() error {
		if v := ctx.GetView(); v != nil {
			//v.Highlight = false
		}
		return nil
	})

	commandEventBus.Subscribe("theme.changed", func(e interface{}) {
		ctx.gui.PostUIUpdate(func() {
			ctx.RefreshThemeColors()
		})
	})

	ctx.RegisterSuggester(commandSuggester)
	ctx.RegisterSuggester(slashCommandSuggester)

	return ctx
}

// RegisterSuggester adds a suggester to the input component's completer
func (c *InputComponent) RegisterSuggester(suggester shell.Suggester) {
	c.completer.RegisterSuggester(suggester)
}

func (c *InputComponent) SetView(view *gocui.View) {
	c.BaseComponent.SetView(view)

	if view != nil {
		view.Editor = c.shellEditor
	}
}

func (c *InputComponent) GetKeybindings() []*types.KeyBinding {
	return []*types.KeyBinding{
		{
			View:    c.viewName,
			Key:     gocui.KeyEnter,
			Handler: c.handleSubmit,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowUp,
			Handler: c.navigateHistoryUp,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowDown,
			Handler: c.navigateHistoryDown,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlC,
			Handler: c.clearInput,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlL,
			Handler: c.clearInput,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEsc,
			Handler: c.handleEsc,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlV,
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     'v',
			Mod:     gocui.Modifier(tcell.ModAlt),
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     'v',
			Mod:     gocui.Modifier(tcell.ModMeta),
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyInsert,
			Mod:     gocui.Modifier(tcell.ModShift),
			Handler: c.handlePaste,
		},
		{
			View:    c.viewName,
			Key:     'v',
			Mod:     gocui.Modifier(tcell.ModCtrl | tcell.ModShift),
			Handler: c.handlePasteForce,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyF4,
			Handler: c.handleF4Expand,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyHome,
			Handler: c.handleHome,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyEnd,
			Handler: c.handleEnd,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlA,
			Handler: c.handleCtrlA,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlE,
			Handler: c.handleCtrlE,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowLeft,
			Mod:     gocui.Modifier(tcell.ModAlt),
			Handler: c.handleAltLeft,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyArrowRight,
			Mod:     gocui.Modifier(tcell.ModAlt),
			Handler: c.handleAltRight,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlB,
			Handler: c.handleCtrlB,
		},
		{
			View:    c.viewName,
			Key:     gocui.KeyCtrlW,
			Handler: c.handleCtrlW,
		},
		{
			View:    c.viewName,
			Key:     'b',
			Mod:     gocui.Modifier(tcell.ModAlt),
			Handler: c.handleAltB,
		},
		{
			View:    c.viewName,
			Key:     'f',
			Mod:     gocui.Modifier(tcell.ModAlt),
			Handler: c.handleAltF,
		},
	}
}

func (c *InputComponent) handleSubmit(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(c.shellEditor.GetInputBuffer())
	if input == "" {
		return nil
	}

	// For input component, treat multiline text as a single message
	// Replace newlines with spaces to handle dictated multiline text
	input = strings.ReplaceAll(input, "\n", " ")
	// Clean up any extra spaces that might result from the replacement
	input = strings.Join(strings.Fields(input), " ")

	c.history.AddCommand(input)
	c.shellEditor.ResetHistoryNavigation() // History navigation reset is now handled by BasicShell

	c.shellEditor.ClearInput(v) // Clear input using the shell editor

	// Determine if input is a command and emit appropriate event
	if strings.HasPrefix(input, ":") {
		// Emit command event
		c.commandEventBus.Emit("user.input.command", input)
	} else if strings.HasPrefix(input, "/") {
		// Emit slash command event
		c.commandEventBus.Emit("user.input.slashcommand", input)
	} else {
		// Emit text/chat message event
		c.commandEventBus.Emit("user.input.text", input)
	}

	return nil
}

func (c *InputComponent) handleEsc(g *gocui.Gui, v *gocui.View) error {
	c.commandEventBus.Emit("user.input.cancel", "")

	// Ensure the input field remains properly rendered after ESC
	// The renderMessages() call from the cancel event can interfere with input display
	c.gui.PostUIUpdate(func() {
		// Force a refresh of the input view to ensure it's properly rendered
		if inputView := c.GetView(); inputView != nil {
			// Redraw the input field by moving cursor to its current position
			cx, cy := inputView.Cursor()
			inputView.SetCursor(cx, cy)
		}
	})

	return nil
}

func (c *InputComponent) navigateHistoryUp(g *gocui.Gui, v *gocui.View) error {
	c.shellEditor.NavigateHistoryUp(v)
	return nil
}

func (c *InputComponent) navigateHistoryDown(g *gocui.Gui, v *gocui.View) error {
	c.shellEditor.NavigateHistoryDown(v)
	return nil
}

func (c *InputComponent) clearInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimSpace(c.shellEditor.GetInputBuffer())

	if input == "" {
		return gocui.ErrQuit
	}

	c.shellEditor.ClearInput(v)
	c.shellEditor.ResetHistoryNavigation()
	return nil
}

func (c *InputComponent) LoadHistory() error {
	return c.history.Load()
}

func combineAtPosition(currentContent string, position int, newContent string) string {
	if currentContent == "" {
		return newContent
	}

	if position < 0 {
		position = 0
	}
	if position > len(currentContent) {
		position = len(currentContent)
	}

	before := currentContent[:position]
	after := currentContent[position:]

	return before + newContent + after
}

func cursorToStringPosition(content string, cursorX, cursorY int) int {
	if content == "" {
		return 0
	}

	lines := strings.Split(content, "\n")

	if cursorY < 0 {
		return 0
	}
	if cursorY >= len(lines) {
		return len(content)
	}

	position := 0
	for i := 0; i < cursorY; i++ {
		position += len(lines[i]) + 1
	}

	line := lines[cursorY]
	if cursorX > len(line) {
		cursorX = len(line)
	}
	position += cursorX

	return position
}

func stringPositionToCursor(content string, position int) (int, int) {
	if content == "" || position <= 0 {
		return 0, 0
	}

	if position > len(content) {
		position = len(content)
	}

	lines := strings.Split(content, "\n")
	currentPos := 0

	for lineIndex, line := range lines {
		lineLength := len(line)

		if currentPos+lineLength >= position {
			x := position - currentPos
			return x, lineIndex
		}

		currentPos += lineLength + 1
	}

	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		return len(lastLine), len(lines) - 1
	}

	return 0, 0
}

func (c *InputComponent) combineWithCurrentInput(v *gocui.View, newContent string) string {
	currentContent := v.Buffer()
	cx, cy := v.Cursor()

	position := cursorToStringPosition(currentContent, cx, cy)

	combinedContent := combineAtPosition(currentContent, position, newContent)

	c.shellEditor.ClearInput(v)

	return strings.TrimSpace(combinedContent)
}

func (c *InputComponent) handlePaste(g *gocui.Gui, v *gocui.View) error {
	clipboardContent, err := c.clipboard.Paste()
	if err != nil {
		return nil
	}

	if strings.Contains(clipboardContent, "\n") {
		combinedContent := c.combineWithCurrentInput(v, clipboardContent)
		c.commandEventBus.Emit("paste.multiline", combinedContent)
		return nil
	}

	currentContent := c.shellEditor.GetInputBuffer()
	cx, cy := v.Cursor()

	position := cursorToStringPosition(currentContent, cx, cy)

	newContent := combineAtPosition(currentContent, position, clipboardContent)

	c.shellEditor.SetInputBuffer(newContent, v)

	newPosition := position + len(clipboardContent)
	newCx, newCy := stringPositionToCursor(newContent, newPosition)
	v.SetCursor(newCx, newCy)

	return nil
}

func (c *InputComponent) handlePasteForce(g *gocui.Gui, v *gocui.View) error {
	clipboardContent, err := c.clipboard.Paste()
	if err != nil {
		c.commandEventBus.Emit("user.input.command", ":write")
		return nil
	}

	combinedContent := c.combineWithCurrentInput(v, clipboardContent)
	c.commandEventBus.Emit("paste.multiline", combinedContent)
	return nil
}

func (c *InputComponent) handleF4Expand(g *gocui.Gui, v *gocui.View) error {
	combinedContent := c.combineWithCurrentInput(v, "")
	c.commandEventBus.Emit("paste.multiline", combinedContent)
	return nil
}

func (c *InputComponent) handleHome(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.SetCursorToBeginning(v)
	}
	return nil
}

func (c *InputComponent) handleEnd(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.SetCursorToEnd(v)
	}
	return nil
}

func (c *InputComponent) handleCtrlA(g *gocui.Gui, v *gocui.View) error {
	return c.handleHome(g, v)
}

func (c *InputComponent) handleCtrlE(g *gocui.Gui, v *gocui.View) error {
	return c.handleEnd(g, v)
}

func (c *InputComponent) handleAltLeft(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.MoveToPreviousWord(v)
	}
	return nil
}

func (c *InputComponent) handleAltRight(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.MoveToNextWord(v)
	}
	return nil
}

func (c *InputComponent) handleCtrlB(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.MoveBackwardChar(v)
	}
	return nil
}

func (c *InputComponent) handleCtrlW(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.DeleteWordBackward(v)
	}
	return nil
}

func (c *InputComponent) handleAltB(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.MoveToPreviousWord(v)
	}
	return nil
}

func (c *InputComponent) handleAltF(g *gocui.Gui, v *gocui.View) error {
	if shell, ok := c.shellEditor.(*shell.BasicShell); ok {
		shell.MoveToNextWord(v)
	}
	return nil
}

