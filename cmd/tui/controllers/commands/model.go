package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/genie"
)

// ModelDefaults persists the model chosen with ":model default".
type ModelDefaults interface {
	SetLocalValue(key string, value any) error
}

// ModelCommand switches the provider and model at runtime, on top of
// whatever the persona pins. Pointing a provider at a proxy makes every
// model behind it one ":model" away.
type ModelCommand struct {
	BaseCommand
	notification types.Notification
	genieService genie.Genie
	bus          *events.CommandEventBus
	defaults     ModelDefaults
}

// NewModelCommand builds the ":model" command. defaults may be nil, which
// disables ":model default".
func NewModelCommand(notification types.Notification, genieService genie.Genie, bus *events.CommandEventBus, defaults ModelDefaults) *ModelCommand {
	return &ModelCommand{
		BaseCommand: BaseCommand{
			Name:        "model",
			Description: "Switch the LLM provider and model for the next turns",
			Usage:       ":model (picker) | :model <model> | :model <provider>/<model> | :model list [provider] | :model reset | :model default <spec>|clear",
			Examples: []string{
				":model",
				":model gpt-5.5",
				":model anthropic/claude-sonnet-5-5",
				":model list openai",
				":model reset",
				":model default openai/gpt-5.5",
			},
			Aliases:  []string{"m"},
			Category: "Persona",
		},
		notification: notification,
		genieService: genieService,
		bus:          bus,
		defaults:     defaults,
	}
}

// Execute runs the subcommand.
func (c *ModelCommand) Execute(args []string) error {
	if len(args) == 0 {
		c.bus.Emit(uimsg.TopicPaletteOpen, uimsg.PaletteModels)
		return nil
	}
	switch args[0] {
	case "list", "-l", "ls":
		provider := ""
		if len(args) > 1 {
			provider = args[1]
		}
		return c.list(provider)
	case "reset", "clear":
		return c.set(genie.ModelSelection{})
	case "default":
		if len(args) < 2 {
			return fmt.Errorf("usage: :model default <provider>/<model> | :model default clear")
		}
		return c.setDefault(args[1])
	default:
		return c.set(genie.ParseModelSpec(strings.Join(args, " ")))
	}
}

func (c *ModelCommand) set(sel genie.ModelSelection) error {
	if err := c.genieService.SetModel(sel); err != nil {
		return err
	}
	c.Announce()
	current := c.genieService.Model(context.Background())
	if sel.IsZero() {
		c.notification.AddSystemMessage(fmt.Sprintf("Model override cleared; using the persona's %s", current))
	} else {
		c.notification.AddSystemMessage(fmt.Sprintf("Using %s for the next turns", current))
	}
	return nil
}

func (c *ModelCommand) setDefault(spec string) error {
	if c.defaults == nil {
		return fmt.Errorf("no settings file to save the default model to")
	}
	if spec == "clear" || spec == "reset" {
		if err := c.defaults.SetLocalValue("Model", nil); err != nil {
			return err
		}
		c.notification.AddSystemMessage("Default model cleared from .genie/settings.tui.json")
		return c.set(genie.ModelSelection{})
	}
	sel := genie.ParseModelSpec(spec)
	if err := c.genieService.SetModel(sel); err != nil {
		return err
	}
	if err := c.defaults.SetLocalValue("Model", sel.String()); err != nil {
		return err
	}
	c.Announce()
	c.notification.AddSystemMessage(fmt.Sprintf("Using %s now and at every start (saved to .genie/settings.tui.json)", c.genieService.Model(context.Background())))
	return nil
}

// Announce tells the UI which provider and model the next turn will use.
func (c *ModelCommand) Announce() {
	sel := c.genieService.Model(context.Background())
	c.bus.Emit(uimsg.TopicModelChanged, map[string]interface{}{"provider": sel.Provider, "model": sel.Model})
}

func (c *ModelCommand) list(provider string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	current := c.genieService.Model(ctx)
	var b strings.Builder
	fmt.Fprintf(&b, "Current: %s\n", current)
	providers := c.genieService.Providers()
	if provider != "" {
		providers = []string{strings.ToLower(provider)}
	}
	if len(providers) == 0 {
		b.WriteString("The backend does not list providers; set one with :model <provider>/<model>\n")
	}
	for _, p := range providers {
		models, err := c.genieService.ListModels(ctx, p)
		switch {
		case errors.Is(err, ai.ErrNoModelList):
			fmt.Fprintf(&b, "%s: cannot list models\n", p)
		case err != nil:
			fmt.Fprintf(&b, "%s: %v\n", p, err)
		default:
			fmt.Fprintf(&b, "%s:\n", p)
			for _, m := range models {
				fmt.Fprintf(&b, "  • %s/%s\n", p, m)
			}
		}
	}
	c.notification.AddSystemMessage(strings.TrimRight(b.String(), "\n"))
	return nil
}
