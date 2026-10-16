package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/bootstrap"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
	"github.com/kcaldas/genie/cmd/tui/help"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/shell"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui"
	"github.com/kcaldas/genie/cmd/tui/ui/bridge"
	"github.com/kcaldas/genie/cmd/tui/ui/dialog"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/files"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

// Program is the Bubble Tea TUI: the genie core, its controllers and the
// root model, wired by hand.
type Program struct {
	model      *ui.Model
	program    *tea.Program
	dispatcher *bridge.Dispatcher
	bus        *events.CommandEventBus
	chat       *controllers.ChatController
	history    history.ChatHistory
}

// Option customises NewProgram, mainly for tests.
type Option func(*programOptions)

type programOptions struct {
	session     genie.Session
	genie       genie.Genie
	config      *helpers.ConfigManager
	historyPath string
	now         func() time.Time
	teaOptions  []tea.ProgramOption
	fileLogging bool
}

// WithGenie uses the given core instead of the process-wide singleton.
func WithGenie(g genie.Genie) Option { return func(o *programOptions) { o.genie = g } }

// WithConfigManager uses a prepared config manager.
func WithConfigManager(cm *helpers.ConfigManager) Option {
	return func(o *programOptions) { o.config = cm }
}

// WithHistoryPath stores command history at path; "" keeps it in memory.
func WithHistoryPath(path string) Option {
	return func(o *programOptions) { o.historyPath = path; o.fileLogging = false }
}

// WithClock injects the clock the status bar reads.
func WithClock(now func() time.Time) Option { return func(o *programOptions) { o.now = now } }

// WithProgramOptions passes options through to tea.NewProgram.
func WithProgramOptions(opts ...tea.ProgramOption) Option {
	return func(o *programOptions) { o.teaOptions = append(o.teaOptions, opts...) }
}

// NewProgram wires the TUI for session. Nothing runs until Run.
func NewProgram(session genie.Session, opts ...Option) (*Program, error) {
	o, err := resolveOptions(session, opts)
	if err != nil {
		return nil, err
	}
	if o.fileLogging {
		logging.SetGlobalLogger(logging.NewFileLoggerFromEnv("genie-debug.log"))
	}

	cfg := o.config.GetConfig()
	bus := events.NewCommandEventBus()
	dispatcher := bridge.NewDispatcher()
	chatState := state.NewChatState(cfg.MaxChatMessages)
	debugState := state.NewDebugState()
	accessor := state.NewStateAccessor(chatState, state.NewUIState())
	hist := openHistory(o)
	registry := commands.NewCommandRegistry()
	slashManager := slashcommands.NewManager()
	keys := ui.DefaultKeymap()
	clipboard := helpers.NewClipboard()

	chat := wireControllers(controllerDeps{
		ui: dispatcher, genie: o.genie, state: accessor, config: o.config, bus: bus,
		chatState: chatState, debugState: debugState, registry: registry,
		slashManager: slashManager, keys: keys, clipboard: clipboard,
	})
	bridge.Subscribe(bus, dispatcher)

	completer := shell.NewCompleter()
	completer.RegisterSuggester(shell.NewCommandSuggester(registry))
	completer.RegisterSuggester(shell.NewSlashCommandSuggester(slashManager))

	model, err := ui.New(ui.Deps{
		Dispatcher:     dispatcher,
		Config:         o.config,
		Chat:           chatState,
		Debug:          debugState,
		State:          accessor,
		Bus:            bus,
		Prompt:         editor.NewPrompt(completer, hist),
		History:        hist,
		Clipboard:      clipboard,
		Keys:           keys,
		Now:            o.now,
		Commands:       func() []dialog.PickerItem { return paletteCommands(registry, slashManager) },
		Files:          func() ([]string, bool) { return files.List(workingDir(session), 0) },
		EditExternally: editExternally,
	})
	if err != nil {
		return nil, err
	}

	return &Program{
		model:      model,
		program:    tea.NewProgram(model, o.teaOptions...),
		dispatcher: dispatcher,
		bus:        bus,
		chat:       chat,
		history:    hist,
	}, nil
}

// resolveOptions applies the options and fills in the production defaults:
// the process-wide genie core and the on-disk TUI config.
func resolveOptions(session genie.Session, opts []Option) (programOptions, error) {
	o := programOptions{fileLogging: true, session: session}
	for _, opt := range opts {
		opt(&o)
	}
	var err error
	if o.genie == nil {
		if o.genie, err = bootstrap.Genie(); err != nil {
			return o, fmt.Errorf("genie core: %w", err)
		}
	}
	if o.config == nil {
		if o.config, err = helpers.NewConfigManager(); err != nil {
			return o, fmt.Errorf("tui config: %w", err)
		}
	}
	return o, nil
}

// openHistory loads the prompt history from the configured path, or from
// the session's genie home; with neither it stays in memory.
func openHistory(o programOptions) history.ChatHistory {
	path := o.historyPath
	if path == "" && o.session != nil {
		path = filepath.Join(o.session.GetGenieHomeDirectory(), ".genie", "history")
	}
	if path == "" {
		return history.NewChatHistory("", false)
	}
	hist := history.NewChatHistory(path, true)
	_ = hist.Load() // a missing history file is not an error
	return hist
}

// controllerDeps is everything the controllers and commands are built from.
type controllerDeps struct {
	ui           types.Dispatcher
	genie        genie.Genie
	state        *state.StateAccessor
	config       *helpers.ConfigManager
	bus          *events.CommandEventBus
	chatState    *state.ChatState
	debugState   *state.DebugState
	registry     *commands.CommandRegistry
	slashManager *slashcommands.Manager
	keys         ui.Keymap
	clipboard    *helpers.Clipboard
}

// wireControllers builds the controllers and registers every ':' command.
// Controllers that only subscribe to buses are constructed for their side
// effects and need not be kept.
func wireControllers(d controllerDeps) *controllers.ChatController {
	chat := controllers.NewChatController(d.ui, d.genie, d.state, d.config, d.bus)
	_ = controllers.NewSlashCommandController(d.bus, d.slashManager, chat)
	_ = controllers.NewConfirmationPanel(d.ui, d.state, d.config, d.genie.GetEventBus(), d.bus)
	helpPanel := controllers.NewHelpPanel(d.ui, help.NewManPageHelpRenderer(d.registry, d.keys.Shortcuts(), d.slashManager))
	debugPanel := controllers.NewDebugPanel(d.ui, d.debugState, d.clipboard, d.bus)
	contextPanel := controllers.NewLLMContextPanel(d.ui, d.genie, d.state, d.bus)

	handler := commands.NewCommandHandler(d.bus, chat, d.registry)
	for _, cmd := range []commands.Command{
		commands.NewClearCommand(chat),
		commands.NewConfigCommand(d.config, d.bus, chat),
		commands.NewContextCommand(contextPanel),
		commands.NewDebugCommand(debugPanel, chat),
		commands.NewDemoCommand(d.genie.GetEventBus(), chat),
		commands.NewExitCommand(d.bus),
		commands.NewHelpCommand(helpPanel),
		commands.NewPersonaCommand(chat, d.genie, d.bus, d.config),
		commands.NewStatusCommand(chat, d.genie),
		commands.NewThemeCommand(d.config, d.bus, chat),
		commands.NewUpdateCommand(chat),
		commands.NewWriteCommand(controllers.NewWritePanel(d.ui)),
		commands.NewYankCommand(d.chatState, d.clipboard, chat),
	} {
		handler.RegisterNewCommand(cmd)
	}
	return chat
}

// Model exposes the root model so tests can drive it with teatest.
func (p *Program) Model() tea.Model { return p.model }

// Start posts the welcome message and, if given, the first user message.
func (p *Program) Start(initialMessage string) {
	p.chat.ShowWelcomeMessage()
	if initialMessage != "" {
		p.bus.Emit(uimsg.TopicInputText, initialMessage)
	}
}

// Run starts the UI and blocks until it exits.
func (p *Program) Run(initialMessage string) error {
	p.Start(initialMessage)
	_, err := p.program.Run()
	if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}

// Stop ends the program if it is still running.
func (p *Program) Stop() {
	if p.program != nil {
		p.program.Quit()
	}
	_ = p.history.Save()
}

// paletteCommands lists the '/' slash commands and ':' commands for the
// command palette, slash commands first.
func paletteCommands(registry *commands.CommandRegistry, slash *slashcommands.Manager) []dialog.PickerItem {
	var items []dialog.PickerItem
	for _, name := range slash.GetCommandNames() {
		cmd, ok := slash.GetCommand(name)
		if !ok {
			continue
		}
		items = append(items, dialog.PickerItem{Label: "/" + name, Detail: firstLine(cmd.Description), Value: "/" + name})
	}
	all := registry.GetAllCommands()
	sort.Slice(all, func(i, j int) bool { return all[i].GetName() < all[j].GetName() })
	for _, cmd := range all {
		items = append(items, dialog.PickerItem{Label: ":" + cmd.GetName(), Detail: cmd.GetDescription(), Value: ":" + cmd.GetName()})
	}
	return items
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// workingDir is the root the @ picker lists: the session's working
// directory, else the process's.
func workingDir(session genie.Session) string {
	if session != nil {
		if dir := session.GetWorkingDirectory(); dir != "" {
			return dir
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// editExternally writes text to a temporary file, runs $VISUAL or $EDITOR
// (vi by default) on it in the terminal, and returns the file's content.
func editExternally(text string) tea.Cmd {
	f, err := os.CreateTemp("", "genie-prompt-*.md")
	if err != nil {
		return func() tea.Msg { return uimsg.ExternalEditDone{Text: text, Err: err} }
	}
	path := f.Name()
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return func() tea.Msg { return uimsg.ExternalEditDone{Text: text, Err: err} }
	}
	_ = f.Close()
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	c := exec.Command(parts[0], append(parts[1:], path)...) //nolint:gosec // the user's own editor
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer os.Remove(path)
		if err != nil {
			return uimsg.ExternalEditDone{Text: text, Err: err}
		}
		out, readErr := os.ReadFile(path)
		if readErr != nil {
			return uimsg.ExternalEditDone{Text: text, Err: readErr}
		}
		return uimsg.ExternalEditDone{Text: string(out)}
	})
}
