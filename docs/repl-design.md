# Genie REPL Design with Bubble Tea

## Overview

Implement a modern Terminal User Interface (TUI) for Genie using the Bubble Tea framework, inspired by LazyGit's excellent terminal interface. This will provide both Unix-friendly direct commands and an interactive REPL experience.

## Research Background

### LazyGit's Tech Stack
- **Bubble Tea**: Modern TUI framework for Go (React-like for terminals)
- **Lipgloss**: Styling and layout engine
- **Bubbles**: Reusable UI components (input, table, viewport, etc.)
- **Charm**: Cloud backend services (optional)

Created by **Charm.sh** team - well-maintained, production-ready ecosystem.

## Dual Mode Architecture

### Direct Command Mode (Current)
```bash
# Single commands - Unix friendly
genie ask "What is Go?"
genie config get model
genie status
genie --help
```

### REPL Mode (New with Bubble Tea)
```bash
# Interactive mode
genie
> ask What is Go?
> /help
> /config get model
> /status
> /clear
> exit
```

## Implementation Strategy

### Phase 1: Dependencies & Basic Structure
```go
// go.mod additions
github.com/charmbracelet/bubbletea
github.com/charmbracelet/lipgloss  
github.com/charmbracelet/bubbles/textinput
github.com/charmbracelet/bubbles/viewport
github.com/charmbracelet/bubbles/table
```

### Phase 2: Mode Detection
```go
// cmd/genie/main.go
func main() {
    if len(os.Args) == 1 {
        // No args = REPL mode
        startRepl()
    } else {
        // Args = Direct command mode  
        rootCmd.Execute()
    }
}
```

### Phase 3: REPL Model Structure
```go
// cmd/genie/repl.go
type ReplModel struct {
    // Input handling
    input      textinput.Model
    
    // Chat display
    messages   []Message
    viewport   viewport.Model
    
    // Session management
    sessionMgr session.SessionManager
    currentSession session.Session
    
    // State
    width      int
    height     int
    ready      bool
}

type Message struct {
    Type      MessageType // User, Assistant, System, Error
    Content   string
    Timestamp time.Time
}

type MessageType int
const (
    UserMessage MessageType = iota
    AssistantMessage
    SystemMessage
    ErrorMessage
)
```

### Phase 4: Core REPL Interface
```go
func (m ReplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "exit":
            return m, tea.Quit
        case "enter":
            return m.handleCommand()
        case "ctrl+l":
            return m.clearScreen()
        }
    case tea.WindowSizeMsg:
        return m.handleResize(msg)
    }
    
    return m, nil
}

func (m ReplModel) View() string {
    if !m.ready {
        return "Initializing Genie REPL..."
    }
    
    return lipgloss.JoinVertical(
        lipgloss.Left,
        m.headerView(),
        m.messagesView(),
        m.inputView(),
        m.footerView(),
    )
}
```

## Feature Roadmap

### Core Features (MVP)
- [x] Basic input/output REPL
- [x] Session management integration
- [x] Command parsing and routing
- [x] Message history display
- [x] Slash commands (`/help`, `/status`, etc.)

### Enhanced Features
- [ ] Multi-line input support
- [ ] Tab completion for commands
- [ ] History navigation (↑/↓ arrows)
- [ ] Real-time typing indicators
- [ ] Syntax highlighting for code
- [ ] Copy/paste support
- [ ] Search through chat history

### Advanced Features
- [ ] Split pane layout (chat + sidebar)
- [ ] Session switcher/tabs
- [ ] Context/history viewer pane
- [ ] Progress indicators for LLM calls
- [ ] Customizable themes
- [ ] Export chat sessions

## UI Layout Design

### Single Pane Layout (MVP)
```
┌─────────────────────────────────────────┐
│ Genie v1.0.0 | Session: main           │ Header
├─────────────────────────────────────────┤
│ > ask What is Go?                       │
│ Go is a programming language...         │ Messages
│                                         │ (scrollable)
│ > /status                               │
│ Status: Ready | Model: gemini-pro       │
├─────────────────────────────────────────┤
│ > _                                     │ Input
├─────────────────────────────────────────┤
│ /help | /status | /clear | exit         │ Footer
└─────────────────────────────────────────┘
```

### Split Pane Layout (Future)
```
┌─────────────────────────┬───────────────┐
│ Genie v1.0.0           │ Session: main │ Header
├─────────────────────────┼───────────────┤
│ > ask What is Go?       │ Context:      │
│ Go is a programming...  │ - file.go     │ Main + 
│                         │ - README.md   │ Sidebar
│ > /status               │               │
│ Status: Ready           │ History:      │
│                         │ - Session 1   │
│                         │ - Session 2   │
├─────────────────────────┴───────────────┤
│ > _                                     │ Input
├─────────────────────────────────────────┤
│ /help | /status | /clear | exit         │ Footer
└─────────────────────────────────────────┘
```

## Command Integration

### Slash Commands
```go
// Support all existing commands as slash commands
/ask <prompt>          // Same as: genie ask <prompt>
/config get <key>      // Same as: genie config get <key>
/config set <key> <val> // Same as: genie config set <key> <val>
/status                // Same as: genie status
/help                  // Show REPL help
/clear                 // Clear chat history
/session new <name>    // Create new session
/session switch <name> // Switch session
/history               // Show session history
/export                // Export current session
```

### Direct Input
```go
// Plain text = ask command
What is Go?            // Same as: /ask What is Go?
Explain this code      // Same as: /ask Explain this code
```

## Styling with Lipgloss

### Color Scheme
```go
var (
    // Colors
    primaryColor   = lipgloss.Color("#7C3AED")  // Purple
    secondaryColor = lipgloss.Color("#10B981")  // Green  
    errorColor     = lipgloss.Color("#EF4444")  // Red
    mutedColor     = lipgloss.Color("#6B7280")  // Gray
    
    // Styles
    headerStyle = lipgloss.NewStyle().
        Background(primaryColor).
        Foreground(lipgloss.Color("#FFFFFF")).
        Bold(true).
        Padding(0, 1)
        
    messageStyle = lipgloss.NewStyle().
        Padding(0, 2)
        
    inputStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(primaryColor).
        Padding(0, 1)
)
```

## Integration Points

### Session Management
- Use existing `session.SessionManager` from our pubsub architecture
- Create new session for each REPL instance
- Support session switching within REPL

### Event System
- Integrate with existing channel-based pubsub
- REPL subscribes to session events for real-time updates
- Support background operations

### Configuration
- Respect existing config system
- REPL-specific settings (theme, layout, etc.)
- User preferences persistence

## Development Phases

### Phase 1: Foundation (Week 1)
- Add Bubble Tea dependencies
- Basic REPL structure
- Mode detection (REPL vs direct)
- Simple input/output

### Phase 2: Core Features (Week 2)
- Session integration
- Slash command parsing
- Message history display
- Basic styling

### Phase 3: Polish (Week 3)
- Enhanced UI/UX
- Error handling
- Help system
- Documentation

### Phase 4: Advanced (Future)
- Split panes
- Tab completion
- Themes
- Export features

## Benefits

### For Users
- **Modern Experience**: Interactive, responsive TUI
- **Unix Compatible**: Direct commands still work
- **Productivity**: History, completion, multi-session
- **Visual**: Syntax highlighting, colors, layout

### For Development
- **Go Native**: Pure Go, no external dependencies
- **Maintainable**: Component-based architecture
- **Testable**: Bubble Tea has good testing support
- **Extensible**: Easy to add new features

## References

- [Bubble Tea Framework](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- [LazyGit Source](https://github.com/jesseduffield/lazygit)
- [Charm Examples](https://github.com/charmbracelet/charm)

## Next Steps

1. **Research**: Study LazyGit's REPL implementation
2. **Prototype**: Basic Bubble Tea REPL
3. **Integrate**: Connect with existing session system
4. **Iterate**: Add features based on user feedback
5. **Polish**: Styling, performance, documentation