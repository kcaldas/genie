package ai

// ToolOutput is the result of executing a tool. Content is the only part sent
// to the model; Details is host-facing data for events, formatters, and session
// records. Keeping those surfaces separate prevents provider adapters from
// inferring model content from arbitrary result-map keys.
type ToolOutput struct {
	Content []ToolContent
	Details map[string]any
	IsError bool
	// Sticky is the tool's retention hint for the turn's activity digest:
	// true = this execution mattered (a real mutation, a failure worth
	// remembering), false = don't bother (a no-op edit, a status read),
	// nil = no opinion. The tool decides — consumers apply no heuristics.
	Sticky *bool
}

// ToolContent is one model-facing block returned by a tool.
type ToolContent interface {
	isToolContent()
}

// TextContent is plain model-facing text.
type TextContent struct {
	Text string
}

func (TextContent) isToolContent() {}

// JSONContent is structured model-facing data. It is serialized centrally
// before provider encoding and may be replaced by bounded text if it exceeds
// the configured output policy.
type JSONContent struct {
	Value any
}

func (JSONContent) isToolContent() {}

// BlobContent is binary model-facing content. MIMEType determines how a
// provider may encode it; Name is an optional user-visible source name or path.
type BlobContent struct {
	MIMEType string
	Data     []byte
	Name     string
}

func (BlobContent) isToolContent() {}

// JSONToolOutput exposes details as one structured model-facing block.
func JSONToolOutput(details map[string]any) ToolOutput {
	return ToolOutput{
		Content: []ToolContent{JSONContent{Value: details}},
		Details: details,
	}
}

// ErrorToolOutput exposes details as a failed structured tool result.
func ErrorToolOutput(details map[string]any) ToolOutput {
	output := JSONToolOutput(details)
	output.IsError = true
	return output
}

// ContentToolOutput constructs an output whose model content differs from its
// host-facing details, as is normally the case for media and MCP tools.
func ContentToolOutput(details map[string]any, content ...ToolContent) ToolOutput {
	return ToolOutput{Content: content, Details: details}
}
