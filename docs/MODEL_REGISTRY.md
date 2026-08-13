# Model Registry Maintenance

Genie reads model limits from checked-in files. Request execution never calls a
provider's model metadata endpoint. Updating metadata is a deliberate
maintenance operation whose diff is reviewed and committed.

The registry has two layers:

- `pkg/ctx/model_registry.go` contains hand-maintained prefixes. OpenAI stays
  here because `/v1/models` does not publish context or output limits.
- `pkg/ctx/model_registry_generated.json` contains provider metadata fetched
  by the offline updater. Generated exact model IDs override matching static
  prefixes.

## Running the Updater

Configure credentials for Anthropic and Google, then run:

```bash
make update-model-registry
```

The equivalent command is:

```bash
go run ./cmd/modelregistry -providers anthropic,google
```

Local providers are opt-in because their catalogs depend on what is installed
and running on the current machine:

```bash
make update-model-registry MODEL_REGISTRY_ARGS="-providers anthropic,google -ollama -lmstudio"
```

Use `-timeout` to change the two-minute command deadline and `-output` when
testing against a copy of the generated file.

The command fetches every selected provider completely before writing. A
network failure, timeout, empty catalog, missing input limit, or malformed
response exits nonzero and leaves the checked-in file untouched. Writes use a
temporary file and atomic rename.

Re-running against unchanged provider data produces byte-identical output.
The `generated_at` provenance timestamp changes only when model data changes.

## Reviewing the Diff

Each provider block records its source. Models that disappear from a provider
response remain in the file with `stale: true` and a human-review note. The
updater never decides to delete a model.

Check these points before committing:

1. Context-window changes agree with the provider announcement.
2. `input_limit_semantics` is `shared_context_window` for Anthropic and local
   models, and `input_only` for Google.
3. `max_output_tokens` is present when the provider reports it.
4. `input_modalities` appears only when the complete capability shape is
   understood. A missing or renamed field must remain unknown (`nil`), which
   preserves the provider adapter's existing media behavior.
5. Stale entries are retained unless a human separately decides to remove
   them.

## OpenAI Models

OpenAI's model-list endpoint identifies models but does not publish their
context windows. For a new OpenAI model:

1. Verify the context and maximum output limits in OpenAI's published model
   documentation.
2. Add or update the narrowest correct prefix in `defaultModelRegistry`.
3. Leave `InputModalities` nil unless every modality has been verified for the
   whole matched prefix.
4. Run `go test ./pkg/ctx ./pkg/config ./pkg/prompts`.

Run the updater for new model launches and provider limit announcements. It is
not a startup task and should not be placed in Genie's request path.
