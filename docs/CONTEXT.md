# How Genie Uses Model Context

Genie manages context at two different timescales:

1. **Between user messages:** Genie keeps conversation memory bounded.
2. **While answering one user message:** Genie may temporarily use much more of
   the model's context window.

These are deliberately separate.

## Example

Suppose the model supports 1 million tokens and Genie's `context_budget` is
100,000 tokens. When the next user message begins, Genie reconstructs about
100K of retained chat history and remembered files.

While answering that message, the model may call tools:

```text
Retained context + user message
    -> model requests searchInFiles
    -> search results are added
    -> model requests readFile
    -> file result is added
    -> model returns the final answer
```

The search and file results may temporarily use much of the remaining model
context. After the final answer, Genie does not retain that complete internal
transcript. The next user message starts again from bounded retained context.

```text
Across user messages:      bounded by context_budget
Inside one user message:   may grow toward the model's physical limit
```

This supports long-running conversations without preventing one task from
using a large model window.

## What Is Retained?

After a successful user turn, Genie retains the user message, the final
assistant answer, and files remembered by the file-context provider.

Genie does not retain intermediate reasoning, function calls, function-call
results, search results, or intermediate assistant messages as chat history.
Remembered files are reconstructed on the next turn under the configured
file-context budget.

## What `context_budget` Means

`context_budget` limits the long-lived context Genie reconstructs for a new
user message. By default, that budget is divided between 70% chat history and
30% remembered file content.

It does not limit tool results produced while Genie is already answering the
current message. It is also not the model's physical context limit.
Instructions, tool definitions, the current user message, and provider
formatting consume additional model context.

The retained budget is selected in this order:

1. The persona's `context_budget`.
2. `GENIE_CONTEXT_BUDGET`.
3. The static model registry's context window multiplied by
   `GENIE_CONTEXT_BUDGET_RATIO`.

## What the Model Limit Means

Every request sent to a provider must fit the selected model's physical
window. Registry values are context windows shared by replayed input and the
next generated response, so Genie reserves the configured generation cap
before estimating room for new tool text. The generation reserve is capped at
half of a shared window. This keeps at least half available for request input
when a global `GENIE_MAX_TOKENS` value is too large for a smaller model.

For each internal tool step, Genie combines the latest usage already returned
by the provider with a conservative estimate of three bytes per token. It
reduces only the pending tool-result text; previously admitted context remains
unchanged, and omitted or truncated results carry a notice telling the model
to narrow the tool call.

This admission is approximate by design. Genie makes no token-count or model
metadata network call inside the tool loop. Provider tokenization, native
media, tool schemas, and protocol overhead can still make the real request
larger than the estimate. The provider's physical limit always wins.

## The 100K-on-1M Strategy

Configuring a 100K `context_budget` for a 1M model means:

- Long-lived conversation memory stays around 100K.
- The current task can temporarily use most of the remaining window.
- That temporary tool transcript is discarded after the final answer.
- The next user message starts bounded again.

It does not reserve exactly 900K for tools. The available space depends on the
assembled request, including instructions, the current message, tool schemas,
reasoning, media, and provider overhead.

Setting the retained budget to 1M does not disable blobs or tools. It means
only that retained context may grow that large. If it actually does, little
physical room may remain for transient work in the next message.

## Safety Limits

Tool text and attachment byte limits are operational safety guards. They
protect Genie from pathological or untrusted tool outputs. They are not
conversation-memory settings and do not replace the model's physical limit.

| Setting | Controls |
|---|---|
| `context_budget` | Retained context reconstructed between user messages |
| `GENIE_CONTEXT_BUDGET` | Host-wide fallback for `context_budget` |
| `GENIE_CONTEXT_BUDGET_RATIO` | Retained-context size when no explicit budget is set |
| Registry context window | Physical request window used for local admission estimates |
| `GENIE_MAX_TOOL_RESULT_BYTES` | Operational guard for one text result |
| `GENIE_MAX_TOOL_BATCH_BYTES` | Guard for one step's combined result text |
| `GENIE_MAX_ATTACHMENT_BYTES` | Operational guard for one decoded attachment |

Setting an operational byte limit to `0` disables that fixed guard. It does
not disable the separate local physical-window estimate.
