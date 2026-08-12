# How Genie Uses Model Context

Genie manages context at two different timescales:

1. **Between user messages:** Genie keeps conversation memory bounded.
2. **While answering one user message:** Genie may temporarily use much more of
   the model's context window.

These are deliberately separate.

## Example

Suppose:

- The model supports **1 million tokens**.
- Genie's `context_budget` is **100,000 tokens**.

When the next user message begins, Genie reconstructs approximately 100K of
retained chat history and remembered files.

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
context.

After the final answer, Genie does not retain that complete internal
transcript. The next user message starts again from the bounded retained
context.

```text
Across user messages:      bounded by context_budget
Inside one user message:   may grow toward the model's physical limit
```

This allows Genie to support long-running conversations without preventing an
individual task from using a large context window.

## What Is Retained?

After a successful user turn, Genie retains:

- The user message.
- The final assistant answer.
- Files remembered by the file-context provider.

Genie does not retain as chat history:

- Intermediate reasoning.
- Function calls.
- Function-call results.
- Search results.
- Intermediate assistant messages.

Remembered files are reconstructed on the next turn under the configured
file-context budget.

## What `context_budget` Means

`context_budget` limits the long-lived context Genie reconstructs for a new
user message.

By default, that budget is divided between:

- 70% chat history.
- 30% remembered file content.

It does not limit tool results produced while Genie is already answering the
current message.

It is also not the model's physical context limit. Instructions, tool
definitions, the current user message, and provider formatting consume
additional model context.

The retained budget is selected in this order:

1. The persona's `context_budget`.
2. `GENIE_CONTEXT_BUDGET`.
3. The model input limit multiplied by `GENIE_CONTEXT_BUDGET_RATIO`.

If provider capability discovery is unavailable, Genie uses its static model
registry to determine the model input limit.

## What the Model Limit Means

Every request sent to the provider must fit the selected model's physical
input limit.

During a tool loop, Genie tracks how much context the current user turn has
accumulated. Before adding new tool results, it checks how much room remains.

If a new result cannot fit:

- Genie reduces only the new result.
- Previously admitted context remains unchanged.
- The model receives a notice that the result was incomplete.

The provider's physical limit always wins. `context_budget` cannot make a
model accept a larger request.

## The 100K-on-1M Strategy

Configuring a 100K `context_budget` for a 1M model means:

- Long-lived conversation memory stays around 100K.
- The current task can temporarily use most of the remaining window.
- That temporary tool transcript is discarded after the final answer.
- The next user message starts bounded again.

It does not reserve exactly 900K for tools. The actual available space depends
on the assembled request, including instructions, the current message, tool
schemas, reasoning, and provider overhead.

Setting the retained budget to 1M does not disable blobs or tools. It means
only that retained context is allowed to grow that large. If it actually does,
little physical room may remain for the transient work needed to answer the
next message.

## Safety Limits

The byte limits for tool text and attachments are operational safety guards.
They protect Genie from pathological or untrusted tool outputs.

They are not conversation-memory settings and do not replace the model's
physical context limit.

| Setting | Controls |
|---|---|
| `context_budget` | Retained context reconstructed between user messages |
| `GENIE_CONTEXT_BUDGET` | Host-wide fallback for `context_budget` |
| `GENIE_CONTEXT_BUDGET_RATIO` | Retained-context size when no explicit budget is set |
| Model input limit | Maximum physical size of every provider request |
| `GENIE_MAX_TOOL_RESULT_BYTES` | Operational guard for one text result |
| `GENIE_MAX_TOOL_BATCH_BYTES` | Optional fixed guard for one step's combined result text |
| `GENIE_MAX_ATTACHMENT_BYTES` | Operational guard for one decoded attachment |

Setting an operational byte limit to `0` disables that fixed guard. Physical
model admission still applies.
