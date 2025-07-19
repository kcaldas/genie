# The Genie Manifesto (WIP)

*AI tools are too opinionated. We're building something different.*

## Why Genie Exists

AI coding assistants are transforming our profession. But they come with opinions, constraints, and walls between you and the models.

When a tool has such profound impact on how we work, we deserve more than black boxes and prescribed workflows.

Genie was born from a simple belief: **powerful tools must be open, transparent, and get out of your way**.

## The Name

Kent Beck called his AI interactions his "genie." The metaphor stuck. Like a genie, AI should serve us powerfully, but we should understand the magic behind the wishes.

## Core Principles

### Understanding Over Convenience
Too many AI tools prioritize ease over insight. We choose understanding. Every interaction should teach us something about how AI thinks, how our code evolves, and how we can improve both.

### Control Over Automation
We've spent decades learning to control our editors, our shells, our workflows. Why should AI be different? Genie gives you the same level of control and customization you expect from tools like vim.

### Speed Over Friction
Like lazygit revolutionized git interactions, Genie makes AI interactions instant. Pop it open from nvim with a shortcut. Access it over SSH in tmux. Run it in VSCode's terminal. 

Terminal-first means accessible everywhere.

### Core Over Features
Rather than another monolithic AI assistant, Genie is a foundation. Like git enabled countless tools, Genie's core enables infinite possibilities.

## The Philosophy

**Get out of the way.**

"Not in your way" means being as close to the models as possible. Create an empty persona with no tools? You're talking directly to the model. No abstractions, no opinions, no barriers.

**Unix philosophy meets AI.**

Genie is a proper Unix citizen:
- Pipe inputs and outputs to integrate LLMs into any workflow
- Use as a CLI tool: `genie ask "explain this" < code.py`
- Works as an MCP client (and soon server) for infinite extensibility

**Genie is a thin layer, not a thick wall.**

The heavy lifting is done by the models. Genie doesn't dictate how you should work - it's the customizable interface that makes models work for you. Like vim doesn't tell you how to write code, Genie doesn't tell you how to use AI.

## The Vision

**AI for everything, configured for anything.**

Yes, Genie writes code. But it also:
- Manages projects with custom personas
- Takes notes the way you think
- Teaches English to a Brazilian mom (true story)
- Adapts to any workflow you imagine

Create a persona. Give it tools. Configure its behavior. Make it yours.

**This is what we've been searching for.**

A tool that learns from the best:
- Customizable like vim - your configurations, your keybindings, your workflow
- Composable like Unix - pipe it, script it, chain it with any tool
- Accessible like SSH - use it anywhere, anytime, from any terminal
- Foundational like git - others can build on top, extend, integrate

Not another platform. Just a tool that does AI right.

## Model Agnostic

**Your choice of models, always.**

Today: Gemini. Tomorrow: Claude, GPT, Llama, Mixtral. Any model, anywhere:
- Commercial APIs for power
- Ollama for local privacy
- Custom endpoints for experimentation

The future is multi-model. Genie embraces it.

## In Practice

```bash
# Direct model access - no barriers
genie ask --persona minimal "explain this regex"

# Unix integration - LLMs as first-class citizens
find . -name "*.go" | genie ask "what patterns do you see?"
git diff | genie ask "suggest a commit message"

# Instant access from vim
:!genie  # pop-up TUI, like lazygit

# Choose your model, any model
genie ask --model gemini-pro "optimize this"
genie ask --model ollama:codellama "explain this"
```

## Why Not [Insert Tool Here]?

**Claude Code** comes closest to the experience we want. But it's closed source. When a tool becomes part of your thinking process, you deserve to own it.

**Cursor, Windsurf, Codex** (yes, we had that name first) - they're all opinionated about how you should work. They have strengths, but they also have walls.

**Gemini CLI, aichat, others** - good tools, but still missing something. Too rigid, too limited, or too complex.

We're not competing. We're building what we wished existed: **Your models, your workflow, your code.**

## Current Reality

This is early. Very early. Expect everything to change - the code, the interfaces, the approach. Much of it isn't up to standards yet.

But it works. **Genie is already writing large chunks of its own code.** Every interaction feeds back into making it better. Every use case reveals new possibilities.

## Open Source Commitment

**This much power must be open.**

Genie is and will always be open source. When tools shape how we think and work, their inner workings must be transparent. You deserve to see, modify, and own every line of code that processes your thoughts.

## The Opportunity

**Developer tools are a massive market.** AI coding assistants are growing exponentially. Yet they're all making the same mistake: building walls instead of bridges.

Genie represents a different path:
- **For developers**: The tool they've been searching for - open, fast, customizable
- **For enterprises**: Control over their AI infrastructure, not vendor lock-in
- **For the ecosystem**: A foundation others can build on, extend, integrate with

## The Challenge

**This needs focused, sustained development.** The architecture is clear. The vision is proven (Genie is already writing its own code). But making this production-ready while keeping it open requires funding.

We're seeking supporters who understand:
- Why developer tools must be open source
- Why terminal-first still matters in 2025
- Why the next decade belongs to those who own their AI stack

If you see the potential - if you understand why developers need to own their AI tools - let's talk.

---

*Genie: Your AI, your way, your code.*
