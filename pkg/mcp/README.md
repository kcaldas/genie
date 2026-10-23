# MCP client

Genie uses `github.com/modelcontextprotocol/go-sdk` v1.8.0 for MCP protocol
handling. It targets **2026-07-28**, with SDK negotiation for older servers.
Genie remains the owner of tool registration, model adapters, permissions,
interceptors, display metadata, and result-size limits.

## Configuration

Put `.mcp.json` in the project directory:

```json
{
  "mcpServers": {
    "connections": {
      "type": "http",
      "url": "https://connections.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${CONNECTIONS_TOKEN}"
      },
      "displayName": "Connections",
      "signalText": {
        "en": "Using a connection",
        "pt-BR": "Usando uma conexão"
      },
      "tools": {
        "bkper_record": {
          "displayName": "Record transaction",
          "signalText": {
            "en": "Recording transaction",
            "pt-BR": "Registrando transação"
          }
        }
      }
    },
    "local": {
      "command": "my-mcp-server",
      "args": [],
      "env": {"MODE": "local"}
    }
  }
}
```

- `http` (or `streamable-http`) selects Streamable HTTP, including JSON and
  SSE responses to POST requests. It does not mean the legacy SSE transport.
- `stdio`, or an omitted type, launches `command` and owns its process lifetime.
- `sse` explicitly selects the legacy HTTP+SSE transport for older servers.
- `headers` supports configured credentials; protocol headers cannot be
  overridden. Cross-origin requests and redirects are refused to keep
  configured headers at their intended server.
- `toolPrefix`, when set, prefixes exposed tool names. For example,
  `"toolPrefix": "accounts_"` exposes remote `record` as `accounts_record`.
  Unqualified names that collide across servers are unavailable, rather than
  being routed according to map iteration order. A warning names the conflicting
  tool and servers and suggests setting `toolPrefix`.
- Configuration lookup remains project first, then the existing user paths.
  `${NAME}` and `${NAME:-default}` environment expansion remain supported.
  Malformed configuration is logged and reported by `ServerErrors` under
  `<config>` without preventing Genie from starting. Individual server
  configuration/connection failures are reported under the server name.

## Display and translation extensions

`displayName`, `signalText`, and per-tool overrides in `tools` retain their
existing meanings. `signalText` accepts a string or a locale-to-string object;
locale resolution is still provided by `tools.LocalizedText.Resolve`.
`Client.ToolsMeta` and registry `MCPToolsMeta` preserve server attribution,
server/tool display names, and localized progress text. Per-tool overrides
use the **remote tool name**, even with `toolPrefix`. The server's advertised
`title` is the fallback tool display name.

These are Genie host extensions, not MCP protocol fields. They are never
sent as tool arguments or used to change tool permissions.

## Supported protocol behavior

The SDK handles discovery/version negotiation, request metadata and routing
headers, request IDs and concurrent calls, JSON/SSE framing, legacy sessions,
and multi-round-trip requests. The new protocol does not use an initialize
handshake or an MCP session header. HTTP cancellation closes the originating
request; Genie suppresses the SDK's legacy cancellation notification for
2026-07-28 HTTP calls. Failed tool calls are not automatically replayed by
Genie, and transport reconnect retries are disabled.

Tool discovery follows pagination and SDK cache hints. Tool-list change
notifications refresh the client's catalog atomically. Bursts coalesce into
one worker per server with at most one pending refresh. A failed refresh keeps
the last successful snapshot, logs a warning, and records `RefreshErrors`;
`ServerErrors` remains reserved for configuration and connection failures.
A successful refresh clears its refresh error. Removed or ambiguous
tools cannot be invoked through previously obtained handlers. An embedding
host can use `Options.ToolsChanged` to rebuild its authorized tool registry;
Genie's ordinary registry takes a startup snapshot. Catalog discovery alone
never authorizes a new tool.

Input and output JSON Schemas are preserved, including `$defs`, `$ref`, union
and composition keywords, boolean subschemas, and additional-property rules.
The model adapters pass the original schema through their JSON Schema APIs;
each model provider still determines which schema features it accepts.
Structured tool results can be any JSON value. Text, image, audio, resource
links, and embedded text/binary resources retain their existing result
conversion and blob-size checks. Binary content is omitted from diagnostic
result details.

## Embedding and optional capabilities

`NewClient` continues to work with the existing registry interface.
`NewClientWithOptions` additionally accepts:

- `ClientOptions(server)`: SDK interaction, progress, and resource/prompt
  notification handlers. SDK multi-round-trip handling uses these callbacks.
- `OAuthHandler(server)`: SDK OAuth handling for Streamable HTTP. The host
  owns browser interaction, consent, and token persistence. Static headers
  work without an OAuth handler; this change does not add a CLI login UI.
- `HTTPClient`: the host's HTTP client (copied before applying server headers
  and redirect policy).
- `ConfigureClient(server, client)`: register SDK custom/extension methods
  and middleware before connection.
- `ToolsChanged(server)`: notification after a successful catalog refresh.

`Session(server)` exposes the SDK's typed resource, resource-template, prompt,
completion, and subscription APIs, plus registered extension methods. The
session belongs to the Genie client; close the client to release it.

Capabilities are optional and advertised according to configured handlers.
By default Genie does not advertise sampling, roots, or elicitation. Hosts
must supply the corresponding implementations before enabling them. Tasks,
Apps, and other extensions are not advertised automatically. SDK-compatible
extensions can be installed through `ConfigureClient` and `ClientOptions`.

Progressive disclosure is a host/catalog presentation choice, not a separate
MCP transport feature. This upgrade retains normal named-tool registration;
it does not introduce a search-tools UI, local file upload conventions,
automatic resource downloads, or provider-specific file binding. Those can
be added separately without making the protocol client provider-aware.

## Tests

```sh
go test ./pkg/mcp ./pkg/llm/...
go test -race ./pkg/mcp ./pkg/tools
```

Remote tests use both an official SDK server and raw JSON-RPC fixtures. They
cover JSON/SSE responses, new protocol metadata, pagination, cancellation,
credential redirect isolation, input requests, resources/prompts, catalog
changes, naming collisions, and localized display metadata. Subprocess tests
exercise the production stdio path, cancellation, concurrent response pairing,
stderr floods, and shutdown. The old standalone fixture tests legacy-version
negotiation; it is not the implementation of the production protocol client.

### Embedded hosts with separate conversation workspaces

Pass `genie.WithMCPConfigDir(agentConfigDir)` to `Start` to discover MCP servers
from a trusted agent directory while file tools remain bound to the session's
working directory. This does not add the config directory to allowed file paths.
Without the option, discovery still uses the working directory. An explicit
configuration directory must exist and be a directory.
