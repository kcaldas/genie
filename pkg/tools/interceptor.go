package tools

import (
	"context"

	"github.com/kcaldas/genie/pkg/ai"
)

// ToolCall is one call the model asked for, as an Interceptor sees it.
type ToolCall struct {
	Name string
	Args map[string]any
}

// Interceptor runs in front of every tool call the model makes, whatever
// the tool: built-in, custom, or MCP. It is the one seam a host has to
// enforce its own rules on what the model may do.
//
// BeforeTool returns the call to run, possibly with replaced Args, or an
// error to refuse it: the handler never runs, and the model sees the
// error's text as the failed tool result. Name is informational; a
// changed Name is ignored.
type Interceptor interface {
	BeforeTool(ctx context.Context, call ToolCall) (ToolCall, error)
}

// InterceptorFunc adapts a function to Interceptor.
type InterceptorFunc func(ctx context.Context, call ToolCall) (ToolCall, error)

// BeforeTool implements Interceptor.
func (f InterceptorFunc) BeforeTool(ctx context.Context, call ToolCall) (ToolCall, error) {
	return f(ctx, call)
}

// Intercept returns the registry with every tool it hands out guarded by
// the interceptor. Tools registered later are guarded too: the guard is
// applied on the way out, not at registration. A nil interceptor returns
// the registry as is.
func Intercept(registry Registry, interceptor Interceptor) Registry {
	if registry == nil || interceptor == nil {
		return registry
	}
	return &interceptedRegistry{Registry: registry, interceptor: interceptor}
}

type interceptedRegistry struct {
	Registry
	interceptor Interceptor
}

func (r *interceptedRegistry) guard(tool Tool) Tool {
	if tool == nil {
		return nil
	}
	if _, already := tool.(*interceptedTool); already {
		return tool
	}
	return &interceptedTool{Tool: tool, interceptor: r.interceptor}
}

func (r *interceptedRegistry) guardAll(list []Tool) []Tool {
	out := make([]Tool, 0, len(list))
	for _, tool := range list {
		out = append(out, r.guard(tool))
	}
	return out
}

func (r *interceptedRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.Registry.Get(name)
	if !ok {
		return nil, false
	}
	return r.guard(tool), true
}

func (r *interceptedRegistry) GetAll() []Tool { return r.guardAll(r.Registry.GetAll()) }

func (r *interceptedRegistry) GetToolSet(setName string) ([]Tool, bool) {
	list, ok := r.Registry.GetToolSet(setName)
	if !ok {
		return nil, false
	}
	return r.guardAll(list), true
}

// interceptedTool is a tool whose handler runs the interceptor first.
type interceptedTool struct {
	Tool
	interceptor Interceptor
}

func (t *interceptedTool) Handler() ai.HandlerFunc {
	inner := t.Tool.Handler()
	name := t.Tool.Declaration().Name
	return func(ctx context.Context, args map[string]any) (ai.ToolOutput, error) {
		if args == nil {
			args = map[string]any{}
		}
		call, err := t.interceptor.BeforeTool(ctx, ToolCall{Name: name, Args: args})
		if err != nil {
			return ai.ToolOutput{}, err
		}
		if call.Args != nil {
			args = call.Args
		}
		return inner(ctx, args)
	}
}
