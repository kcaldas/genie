# Genie Event System Overview

This document details the events used within the Genie project, including their structure, the components that publish them, and the components that subscribe to them. The event system is built around an `EventBus` for asynchronous communication, promoting loose coupling between components.

## Event Summary Table

| Event Type | Topic | Event Body (JSON) | Publishers (Full Package Path) | Subscribers (Full Package Path) |
|---|---|---|---|---|
| `SessionInteractionEvent` | `session.interaction` | `{"SessionID": "string", "UserMessage": "string", "AssistantResponse": "string"}` | (Used in `pkg/events/HistoryChannel` and `pkg/events/ContextChannel`, not directly published on EventBus in search results) | (Used in `pkg/events/HistoryChannel` and `pkg/events/ContextChannel`, not directly subscribed from EventBus in search results) |
| `ToolExecutedEvent` | `tool.executed` | `{"ExecutionID": "string", "SessionID": "string", "ToolName": "string", "Parameters": {}, "Message": "string", "Result": {}}` | `pkg/genie/mock_prompt_runner`, `pkg/ctx/project_context_part_provider_test`, `pkg/ctx/context_manager_test` | `pkg/ctx/file_ctx_provider` |
| `ToolConfirmationRequest` | `tool.confirmation.request` | `{"ExecutionID": "string", "SessionID": "string", "ToolName": "string", "Command": "string", "Message": "string"}` | `pkg/ai/prompt` | `cmd/cli/ask` |
| `ToolConfirmationResponse` | `tool.confirmation.response` | `{"ExecutionID": "string", "Confirmed": "bool"}` | Client (Implicit, after user interaction) | (No direct subscriber found in search results. The component making the request, e.g., `pkg/ai/prompt`, would typically await this response.) |
| `UserConfirmationRequest` | `user.confirmation.request` | `{"ExecutionID": "string", "SessionID": "string", "Title": "string", "Content": "string", "ContentType": "string", "FilePath": "string", "Message": "string", "ConfirmText": "string", "CancelText": "string"}` | `pkg/ai/prompt` | `cmd/cli/ask` |
| `UserConfirmationResponse` | `user.confirmation.response` | `{"ExecutionID": "string", "Confirmed": "bool"}` | Client (Implicit, after user interaction) | `pkg/tools/write`, `pkg/ai/prompt`, `pkg/handlers/file_generator` |
| `ChatResponseEvent` | `chat.response` | `{"SessionID": "string", "Message": "string", "Response": "string", "Error": "error"}` | `cmd/cli/ask`, `pkg/genie/pubsub_integration_test`, `pkg/genie/genie_test`, `pkg/genie/core`, `pkg/ctx/chat_context_part_provider_test`, `pkg/ctx/context_manager_test` | `cmd/cli/ask`, `pkg/genie/genie_test`, `pkg/ctx/chat_context_part_provider` |
| `ChatStartedEvent` | `chat.started` | `{"SessionID": "string", "Message": "string"}` | `pkg/genie/genie_test`, `pkg/genie/core` | `pkg/genie/genie_test` |

## Notes on Event Usage:

*   **`SessionInteractionEvent`**: While defined in `pkg/events`, the search results indicate its primary use within `HistoryChannel` and `ContextChannel` for managing conversation history and context, rather than direct publication on the main `EventBus` for real-time signaling.
*   **Confirmation Responses (`ToolConfirmationResponse`, `UserConfirmationResponse`)**: These events are typically published by the client (e.g., `cmd/cli/ask`) after receiving user input in response to a confirmation request. The component that initiated the request (e.g., an AI prompt processor) would then process this response.
