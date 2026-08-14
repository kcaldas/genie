---
name: update-model-registry
description: Refresh pkg/ctx/model_registry.go from provider metadata APIs and docs. Use when a provider launches a new model, when a model name fails registry lookup, or for a periodic sweep. Produces a reviewed diff on a branch — never edits blind, never runs at request time.
---

# Update the Model Registry

`pkg/ctx/model_registry.go` is the single source of model metadata Genie reads at
runtime (context windows, output caps, verified input modalities). Runtime makes **no
network call** for model metadata — this skill is the maintenance path that keeps the
checked-in data current. The cache-invalidation mechanism is git review, matching how
often the data actually changes (provider model launches).

The deliverable is always a **reviewable diff with per-value provenance**, on a branch.
Never push to main. Never write a value you cannot cite to an API response or a docs URL
fetched during this run.

## 1. Baseline

Read `pkg/ctx/model_registry.go` in full. Note the current `ModelInfo` fields (the
struct may have evolved since this skill was written — adapt to what is there) and the
existing entries per provider. Then establish scope:

- If the user named specific models, verify those.
- Otherwise do a full sweep of the providers whose API keys are available, plus any
  model name referenced by Genie defaults or persona configs that has no registry entry
  (`grep -rn "model_name\|GENIE_MODEL_NAME" --include='*.yaml' --include='*.go' | grep -v _test`
  and check each against `LookupModelInfo` coverage).

## 2. Fetch provider metadata

Fail loudly: if a fetch errors, times out, or returns a shape you don't recognize,
report it and skip that provider — never fill the gap with a guess.

**Anthropic** (needs `ANTHROPIC_API_KEY`):

```bash
curl -sf https://api.anthropic.com/v1/models \
  -H "x-api-key: $ANTHROPIC_API_KEY" -H "anthropic-version: 2023-06-01"
```

Paginate with `?after_id=<last_id>` while `has_more` is true. Per model:
`max_input_tokens` → `ContextWindow` (this is a **shared** context window — input and
generated output both live in it), `max_tokens` → `MaxOutputTokens`, and the
`capabilities` object for modalities (see §3 for when to trust it).

**Google** (needs `GEMINI_API_KEY`):

```bash
curl -sf "https://generativelanguage.googleapis.com/v1beta/models?pageSize=200&key=$GEMINI_API_KEY"
```

Strip the `models/` prefix from names. `inputTokenLimit` → `ContextWindow`,
`outputTokenLimit` → `MaxOutputTokens`. Note: Gemini's limit is **input-only** (output
is budgeted separately by the provider). Genie currently treats all registry windows as
shared, which over-reserves slightly on Gemini — that is the safe direction; leave it
unless `ModelInfo` has grown a semantics field, in which case record `input_only`.

**OpenAI — docs-sourced, no exceptions.** `/v1/models` does not publish token limits.
For each OpenAI entry you are adding or changing, WebFetch the official models
documentation (platform.openai.com/docs/models) and read the limit from the page. Cite
the URL per value. If the docs are unreachable or ambiguous, leave the entry untouched
and flag it.

**Ollama / LM Studio** — only when the user asks and a local instance is running
(`curl -sf localhost:11434/api/tags`; LM Studio `curl -sf localhost:1234/v1/models`).
These entries are conservative family defaults; don't churn them from one local install.

## 3. Editing rules (the invariants)

1. **Never delete a disappeared model.** If a model is gone from the provider catalog,
   keep the entry and mark it with a dated comment (`// stale: absent from provider
   catalog as of YYYY-MM-DD`). Deletion is a human decision made in PR review.
2. **`InputModalities` is opt-in per entry and stays nil by default.** Populate it only
   when the provider returned a complete, unambiguous capability object for that model
   in this run. A wrong context window costs headroom; a wrong modality map silently
   stops media delivery with no error anywhere — nil means "unknown" and preserves each
   provider adapter's own behavior. When in doubt, nil.
3. **Respect prefix matching.** Lookup is longest-prefix-wins on a name boundary. Before
   adding a key, check it neither shadows nor is shadowed by an existing entry in a way
   that changes other lookups (e.g. adding `gpt-5` alongside `gpt-5-chat-latest` is
   fine; adding a bare `claude` would not be). The lookup tests in
   `pkg/ctx/model_registry_test.go` are the check — extend them for new families.
4. **Keep the file shape.** Same provider grouping and ordering, `gofmt` clean. New
   fields on `ModelInfo` require updating this skill, not working around it.
5. **Never echo key material** into output, commits, or the PR.

## 4. Validate

```bash
go build ./... && gofmt -l pkg/ctx && go test ./pkg/ctx/... ./pkg/config/... ./pkg/llm/...
```

The registry-walk test (`TestDefaultMaxTokensLeavesInputRoomForEveryRegistryEntry`)
must pass — it asserts every entry leaves usable tool room under default
`GENIE_MAX_TOKENS`. If a new small-window entry fails it, that is the test doing its
job: the entry needs a `MaxOutputTokens` value or the window is wrong.

## 5. Deliver

1. Branch from `origin/main` (e.g. `chore/model-registry-YYYY-MM-DD`).
2. Commit with a message summarizing what changed and why now.
3. Present the user a provenance table before pushing, one row per changed value:

   | Model | Field | Old | New | Source |
   |---|---|---|---|---|
   | claude-x | ContextWindow | 200000 | 1000000 | GET /v1/models (this run) |
   | gpt-x | ContextWindow | — | 400000 | platform.openai.com/docs/models/gpt-x |

4. Open the PR with that table in the description. Stale-markings and any skipped
   providers get their own section so the reviewer decides deletions consciously.

## Failure modes to refuse

- A provider fetch failed → report, skip provider, continue with the rest. Never write
  partial guesses for the failed provider.
- Asked to wire fetching into runtime, startup, or the tool loop → decline and point at
  `docs/CONTEXT.md`: runtime metadata is checked-in by design; hosts embedding Genie
  cold-start too often for live discovery.
- A number that only exists in a blog post or memory → it goes in the PR description as
  a note for the reviewer, not in the registry.
