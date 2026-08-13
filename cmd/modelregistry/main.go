package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kcaldas/genie/internal/modelregistry"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/anthropic"
	"github.com/kcaldas/genie/pkg/llm/genai"
	"github.com/kcaldas/genie/pkg/llm/lmstudio"
	"github.com/kcaldas/genie/pkg/llm/ollama"
)

func main() {
	var (
		providers = flag.String("providers", "anthropic,google", "comma-separated hosted providers to refresh")
		output    = flag.String("output", "pkg/ctx/model_registry_generated.json", "generated registry path")
		timeout   = flag.Duration("timeout", 2*time.Minute, "maximum updater runtime")
		useOllama = flag.Bool("ollama", false, "include models from the configured Ollama instance")
		useLM     = flag.Bool("lmstudio", false, "include models from the configured LM Studio instance")
	)
	flag.Parse()

	if err := run(*providers, *output, *timeout, *useOllama, *useLM); err != nil {
		fmt.Fprintln(os.Stderr, "model registry update failed:", err)
		os.Exit(1)
	}
}

func run(providerList, output string, timeout time.Duration, useOllama, useLM bool) error {
	existing, err := os.ReadFile(output)
	if err != nil {
		return fmt.Errorf("read %s: %w", output, err)
	}
	sources, err := buildSources(providerList, useOllama, useLM)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	updated, changed, err := modelregistry.Update(ctx, existing, sources, time.Now)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("model registry is already current")
		printOpenAIChecklist()
		return nil
	}
	if err := atomicWrite(output, updated); err != nil {
		return err
	}
	fmt.Printf("updated %s\n", output)
	printOpenAIChecklist()
	return nil
}

func buildSources(providerList string, useOllama, useLM bool) ([]modelregistry.Source, error) {
	requested := strings.Split(providerList, ",")
	if useOllama {
		requested = append(requested, "ollama")
	}
	if useLM {
		requested = append(requested, "lmstudio")
	}
	seen := make(map[string]bool)
	var sources []modelregistry.Source
	for _, name := range requested {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "genai" {
			name = "google"
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		source, err := newSource(name)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no providers selected")
	}
	return sources, nil
}

func printOpenAIChecklist() {
	fmt.Println("OpenAI limits remain hand-maintained: verify new IDs against the published model documentation.")
}

func newSource(name string) (modelregistry.Source, error) {
	bus := &events.NoOpEventBus{}
	var (
		gen        ai.Gen
		err        error
		provenance string
	)
	switch name {
	case "anthropic":
		gen, err = anthropic.NewClient(bus)
		provenance = "Anthropic GET /v1/models and /v1/models/{id}"
	case "google", "genai":
		name = "google"
		gen, err = genai.NewClient(bus)
		provenance = "Google models.list and models.get"
	case "ollama":
		gen, err = ollama.NewClient(bus)
		provenance = "Ollama /api/tags and /api/show (opt-in local instance)"
	case "lmstudio":
		gen, err = lmstudio.NewClient(bus)
		provenance = "LM Studio /api/v1/models (opt-in local instance)"
	default:
		return modelregistry.Source{}, fmt.Errorf("unsupported provider %q", name)
	}
	if err != nil {
		return modelregistry.Source{}, err
	}
	discoverer, ok := gen.(ai.ModelCatalogDiscoverer)
	if !ok {
		return modelregistry.Source{}, fmt.Errorf("provider %q has no offline model discoverer", name)
	}
	return modelregistry.Source{Name: name, Provenance: provenance, Discoverer: discoverer}, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".model-registry-*")
	if err != nil {
		return fmt.Errorf("create temporary registry: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary registry: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("set generated registry permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace registry: %w", err)
	}
	return nil
}
