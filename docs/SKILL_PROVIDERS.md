# Skill providers

Genie separates skill content from session state. A `skills.Provider` supplies
skill definitions and resources; a `skills.SkillManager` owns each session's
active skill and loaded resources. Hosts can supply a provider without replacing
Genie's skill tool or implementing its context lifecycle.

```go
g, err := genie.NewGenie(genie.WithSkillProvider(provider))
```

The provider replaces default discovery. It is used by the prompt's skill
catalog, the `Skill` tool (including `file` and `list_files`), and active-skill
context. Native task children inherit the provider but create separate managers.
A host using a custom task executor is responsible for configuring its children.

## Provider contract

```go
type Provider interface {
    ListSkills(context.Context) ([]SkillMetadata, error)
    LoadSkill(context.Context, string) (*Skill, error)
    ReadFile(ctx context.Context, name, path string) ([]byte, error)
    ListFiles(ctx context.Context, name string) ([]string, error)
}
```

- Operations receive the caller's context. Session ID, Genie home and working
  directory are carried by `pkg/toolctx` when Genie runs a session. The session
  ID is Genie's own ID, not a host's conversation ID or an authorization identity.
  Child turns carry their own ID even when their caller context came from the
  parent. Filtering by it is optional; hosts can instead construct scoped
  providers. Pre-session persona validation may call a provider without a
  session ID, so providers must tolerate its absence.
- Implementations must support concurrent calls, including calls from child
  engines. Keep shared definitions immutable or synchronize updates.
- List only eligible skills and enforce the same policy when loading a skill,
  reading a resource, or listing resources. The manager also checks the catalog
  before exposing active content. Use `SkillNotFoundError` for unavailable skills;
  active context then omits the skill without interrupting the conversation.
  Other errors while revalidating active skills fail context assembly and the
  turn before any model request. The error is reported through the normal chat
  response event, and conversation state remains available for retry.
- Resource names are skill-relative paths with forward slashes. Resources can
  come from folders, embedded files, or another source. Genie does not fall back
  to local files when a custom provider returns an error.
- `Skill.BaseDir` describes the skill's location; it does not cause Genie to read
  that directory. Providers backed by actual folders should supply an absolute
  directory so scripts can resolve their accompanying files. A virtual location
  does not make scripts executable; materialize files if execution is needed.
- Genie copies definitions into its session state. `GetActiveSkill` returns a
  snapshot; to read loaded resources, request a fresh snapshot after loading.
- Skill availability never grants tool permissions. The host and tool registry
  remain responsible for access control. A host requiring different eligibility
  for child agents must use their effective tool scope when constructing or
  evaluating its provider, rather than assuming the parent's tools are present.

A provider can combine bundled and owner-installed folders, implement overrides,
or filter skills by available tools. These are host policies, not Genie policy.
A host that wants defaults alongside its skills can compose
`skills.NewDefaultProvider()` explicitly and define its own precedence.

## Defaults

Without `WithSkillProvider` (or with a nil provider), discovery retains its
existing precedence, highest first:

1. `<genie home>/.claude/skills`
2. `<genie home>/.genie/skills`
3. `$HOME/.genie/skills`
4. Embedded skills, currently `skill-creator` (internal test fixtures excluded)

The default provider caches catalogs by Genie home so concurrent sessions do not
change each other's discovery roots. `SetGenieHome` invalidates these catalogs.
As before, newly installed skills need rediscovery or an engine restart.

Filesystem skills first resolve supporting files inside the skill directory,
then in the operation's working directory. Embedded resources are read from the
embedded bundle. Absolute paths and traversal are rejected; filesystem resource
symlinks must stay within their respective root.

Each Genie instance now owns its manager. There is no process-global active
skill state. Skill lifecycle events are still emitted as notifications; context
reads session state directly instead of depending on event delivery. In-memory
personas rebuild their catalog with the current operation context, so eligibility
is not frozen when the persona is configured.

## Frontmatter and eligibility

The [Agent Skills specification](https://agentskills.io/specification) defines
experimental `allowed-tools` as pre-approved tools, not a dependency gate.
`compatibility` describes environment requirements in text. There is no standard
machine-readable required-tools gate in that specification.

Its `metadata` extension is a map of string keys to string values. Genie preserves
that map during discovery and loading, without interpreting it. For example, a
host could choose this convention:

```yaml
---
name: workspace-html
description: Create HTML pages for the workspace.
metadata:
  mutiro.requires-tools: "readFile writeFile"
  mutiro.requires-capabilities: "workspace_html_preview"
---
```

These keys are an example host convention, not an Agent Skills standard or a
built-in Genie permission mechanism. Other harnesses can still load the folder
and choose whether to interpret those keys.

## Existing integrations

`NewDefaultSkillManager` and `WithSkillProvider` keep ordinary Genie callers on
the default discovery path unless they opt in. Custom `SkillManager`
implementations must now implement `ListSkillFiles` and `ClearAllActiveSkills`.
Consumers that used lifecycle events to inject context should instead activate
skills through the manager with the appropriate session context.

Advanced callers of `NewGenieWithComponents` may pass the same provider as its
optional final argument so native children inherit it. They must also wire that
provider into their manually assembled tool, persona and context managers.
