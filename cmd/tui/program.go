package tui

import (
	"errors"
	"fmt"
	"path/filepath"
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
	"github.com/kcaldas/genie/cmd/tui/ui"
	"github.com/kcaldas/genie/cmd/tui/ui/bridge"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
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
	o := programOptions{fileLogging: true}
	for _, opt := range opts {
		opt(&o)
	}
	if o.fileLogging {
		logging.SetGlobalLogger(logging.NewFileLoggerFromEnv("genie-debug.log"))
	}

	var err error
	if o.genie == nil {
		if o.genie, err = bootstrap.Genie(); err != nil {
			return nil, fmt.Errorf("genie core: %w", err)
		}
	}
	if o.config == nil {
		if o.config, err = helpers.NewConfigManager(); err != nil {
			return nil, fmt.Errorf("tui config: %w", err)
		}
	}
	cfg := o.config.GetConfig()

	bus := events.NewCommandEventBus()
	chatState := state.NewChatState(cfg.MaxChatMessages)
	debugState := state.NewDebugState()
	accessor := state.NewStateAccessor(chatState, state.NewUIState())
	clipboard := helpers.NewClipboard()
	keys := ui.DefaultKeymap()

	historyPath := o.historyPath
	persist := true
	if historyPath == "" {
		if session != nil {
			historyPath = filepath.Join(session.GetGenieHomeDirectory(), ".genie", "history")
		} else {
			persist = false
		}
	}
	hist := history.NewChatHistory(historyPath, persist)
	if persist {
		_ = hist.Load() // a missing history file is not an error
	}

	registry := commands.NewCommandRegistry()
	slashManager := slashcommands.NewManager()
	completer := shell.NewCompleter()
	completer.RegisterSuggester(shell.NewCommandSuggester(registry))
	completer.RegisterSuggester(shell.NewSlashCommandSuggester(slashManager))

	dispatcher := bridge.NewDispatcher()
	chat := controllers.NewChatController(dispatcher, o.genie, accessor, o.config, bus)
	controllers.NewSlashCommandController(bus, slashManager, chat)
	helpPanel := controllers.NewHelpPanel(dispatcher, help.NewManPageHelpRenderer(registry, keys.Shortcuts(), slashManager))
	debugPanel := controllers.NewDebugPanel(dispatcher, debugState, clipboard, bus)
	contextPanel := controllers.NewLLMContextPanel(dispatcher, o.genie, accessor, bus)

	handler := commands.NewCommandHandler(bus, chat, registry)
	for _, cmd := range []commands.Command{
		commands.NewClearCommand(chat),
		commands.NewConfigCommand(o.config, bus, chat),
		commands.NewContextCommand(contextPanel),
		commands.NewDebugCommand(debugPanel, chat),
		commands.NewDemoCommand(o.genie.GetEventBus(), chat),
		commands.NewExitCommand(bus),
		commands.NewHelpCommand(helpPanel),
		commands.NewPersonaCommand(chat, o.genie, bus, o.config),
		commands.NewStatusCommand(chat, o.genie),
		commands.NewThemeCommand(o.config, bus, chat),
		commands.NewUpdateCommand(chat),
		commands.NewWriteCommand(controllers.NewWritePanel(dispatcher)),
		commands.NewYankCommand(chatState, clipboard, chat),
	} {
		handler.RegisterNewCommand(cmd)
	}

	bridge.Subscribe(bus, dispatcher)

	model, err := ui.New(ui.Deps{
		Dispatcher: dispatcher,
		Config:     o.config,
		Chat:       chatState,
		Debug:      debugState,
		State:      accessor,
		Bus:        bus,
		Line:       editor.NewLine(completer, hist),
		History:    hist,
		Clipboard:  clipboard,
		Keys:       keys,
		Now:        o.now,
	})
	if err != nil {
		return nil, err
	}

	p := &Program{model: model, dispatcher: dispatcher, bus: bus, chat: chat, history: hist}
	p.program = tea.NewProgram(model, o.teaOptions...)
	return p, nil
}

// Model exposes the root model so tests can drive it with teatest.
func (p *Program) Model() tea.Model { return p.model }

// Start posts the welcome message and, if given, the first user message.
func (p *Program) Start(initialMessage string) {
	p.chat.ShowWelcomeMessage()
	if initialMessage != "" {
		p.bus.Emit("user.input.text", initialMessage)
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
