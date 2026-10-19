package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/genie"
)

// ModelDefaults persists the model chosen with ":model default" and reports
// what the merged settings still hold after a change.
type ModelDefaults interface {
	SetLocalValue(key string, value any) error
	GetConfig() *types.Config
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
		if len(args) > 1 {
			return fmt.Errorf("usage: %s", c.Usage)
		}
		return c.setSpec(args[0])
	}
}

// setSpec parses and applies one "<provider>/<model>" argument.
func (c *ModelCommand) setSpec(spec string) error {
	sel := genie.ParseModelSpec(spec)
	if sel.IsZero() {
		return fmt.Errorf("usage: %s", c.Usage)
	}
	return c.set(sel)
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
		if err := c.set(genie.ModelSelection{}); err != nil {
			return err
		}
		if global := c.defaults.GetConfig().Model; global != "" {
			c.notification.AddSystemMessage(fmt.Sprintf("Default model cleared from .genie/settings.tui.json; ~/.genie/settings.tui.json still sets %q and applies at the next start", global))
		} else {
			c.notification.AddSystemMessage("Default model cleared from .genie/settings.tui.json")
		}
		return nil
	}
	if err := c.setSpec(spec); err != nil {
		return err
	}
	if err := c.defaults.SetLocalValue("Model", c.genieService.Model(context.Background()).String()); err != nil {
		return fmt.Errorf("model switched for this session, but saving the default failed: %w", err)
	}
	c.notification.AddSystemMessage("Saved as the default model in .genie/settings.tui.json; it applies at every start")
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
	for _, listed := range ListProviderModels(ctx, c.genieService, providers) {
		switch {
		case errors.Is(listed.Err, ai.ErrNoModelList):
			fmt.Fprintf(&b, "%s: cannot list models\n", listed.Provider)
		case listed.Err != nil:
			fmt.Fprintf(&b, "%s: %v\n", listed.Provider, listed.Err)
		default:
			fmt.Fprintf(&b, "%s:\n", listed.Provider)
			for _, m := range listed.Models {
				fmt.Fprintf(&b, "  • %s/%s\n", listed.Provider, m)
			}
		}
	}
	c.notification.AddSystemMessage(strings.TrimRight(b.String(), "\n"))
	return nil
}

// ProviderModels is one provider's listing, or why it has none.
type ProviderModels struct {
	Provider string
	Models   []string
	Err      error
}

// providerListTimeout bounds each provider's model listing, so one slow
// endpoint cannot hold up the others.
const providerListTimeout = 4 * time.Second

// ListProviderModels asks every provider for its models concurrently and
// returns the results in the providers' order.
func ListProviderModels(ctx context.Context, g genie.Genie, providers []string) []ProviderModels {
	out := make([]ProviderModels, len(providers))
	var wg sync.WaitGroup
	for i, provider := range providers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, providerListTimeout)
			defer cancel()
			models, err := g.ListModels(pctx, provider)
			out[i] = ProviderModels{Provider: provider, Models: models, Err: err}
		}()
	}
	wg.Wait()
	return out
}
