# Typed Tool Outputs

Tool handlers return `ai.ToolOutput` instead of `map[string]any`.
Model-facing content and host-facing metadata are deliberately separate:

```go
func (t *Tool) Handler() ai.HandlerFunc {
    return func(ctx context.Context, args map[string]any) (ai.ToolOutput, error) {
        details := map[string]any{"success": true, "path": "report.pdf"}
        return ai.ContentToolOutput(details,
            ai.TextContent{Text: "Report loaded."},
            ai.BlobContent{
                MIMEType: "application/pdf",
                Data:     pdfBytes,
                Name:     "report.pdf",
            },
        ), nil
    }
}
```

Use `ai.JSONToolOutput(details)` when the same structured result should be
shown to the model and retained by the host. Use
`ai.ErrorToolOutput(details)` for an expected tool-level failure that the
model can recover from. Return a non-nil Go error only when execution itself
failed.

`ToolOutput.Content` accepts `ai.TextContent`, `ai.JSONContent`, and
`ai.BlobContent`. Do not put base64 data or data URLs in `Details`; provider
adapters own wire encoding. `ToolOutput.Details` continues to feed
`ToolExecutedEvent`, formatters, and session recording.

## Mutiro Migration

When bumping Genie in Mutiro:

1. Change every custom `ai.HandlerFunc` return type to `ai.ToolOutput`.
2. Wrap ordinary result maps with `ai.JSONToolOutput`.
3. Move image, document, audio, and other binary bytes into
   `ai.BlobContent`; remove `data_base64` and `data_url` keys.
4. Mark recoverable tool failures with `ai.ErrorToolOutput`.
5. Keep UI-only fields in `Details` and update direct result indexing in
   tests to use `result.Details`.

Create one capability resolver per Mutiro agent and inject it into every
short-lived Genie instance:

```go
capabilityResolver := ai.NewCapabilityResolver(agentCapabilityStore)

instance, err := genie.NewGenie(
    genie.WithCapabilityResolver(capabilityResolver),
    // existing options...
)
```

`agentCapabilityStore` may be `ai.NewMemoryCapabilityStore()` or Mutiro's own
implementation of `ai.CapabilityStore`. Keep the resolver, not only the store,
at agent scope so concurrent instance startup shares the same in-flight
provider lookup.

Native content does not spend the synthetic context-part budget. Anthropic and
Gemini admit it with an authoritative count of the accumulated request; other
providers use the latest usage and a conservative local estimate. Shared
context windows reserve output capacity first. This lets a 100K synthetic text
budget use the remaining headroom of a 1M-token model without treating that
temporary work as retained conversation memory.
