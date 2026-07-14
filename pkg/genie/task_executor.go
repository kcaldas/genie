package genie

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/tools"
)

type nativeTaskExecutor struct {
	parent *core
}

func newNativeTaskExecutor(parent *core) tools.TaskExecutor {
	return &nativeTaskExecutor{parent: parent}
}

func (e *nativeTaskExecutor) RunTask(runCtx context.Context, request tools.TaskRequest, reporter tools.TaskReporter) (tools.TaskResult, error) {
	if e == nil || e.parent == nil {
		err := fmt.Errorf("native task executor is not configured")
		return tools.TaskResult{Error: err.Error()}, err
	}

	parentSession, err := e.parent.sessionMgr.GetSession()
	if err != nil {
		return tools.TaskResult{Error: err.Error()}, err
	}

	child, childEvents, err := e.newChildGenie()
	if err != nil {
		return tools.TaskResult{Error: err.Error()}, err
	}

	responseCh := make(chan events.ChatResponseEvent, 1)
	childEvents.Subscribe(events.ChatResponseEvent{}.Topic(), func(event interface{}) {
		response, ok := event.(events.ChatResponseEvent)
		if !ok {
			return
		}
		select {
		case responseCh <- response:
		default:
		}
	})

	workspace := strings.TrimSpace(request.Workspace)
	if workspace == "" {
		workspace = parentSession.GetWorkingDirectory()
	}
	personaID := strings.TrimSpace(request.Persona)
	if personaID == "" && parentSession.GetPersona() != nil {
		personaID = parentSession.GetPersona().GetID()
	}

	if reporter != nil {
		reporter.Log("starting native Genie child session")
	}

	startOptions := []StartOption{
		WithAllowedDirs(parentSession.GetAllowedDirectories()...),
		WithDeniedPaths(parentSession.GetDeniedPaths()...),
		WithReadOnlyPaths(parentSession.GetReadOnlyPaths()...),
	}
	if name, email := parentSession.GetCommitAuthor(); name != "" || email != "" {
		startOptions = append(startOptions, WithCommitAuthor(name, email))
	}

	var personaPtr *string
	if personaID != "" {
		personaPtr = &personaID
	}
	if _, err := child.Start(&workspace, personaPtr, startOptions...); err != nil {
		return tools.TaskResult{Error: err.Error()}, err
	}

	if err := child.Chat(runCtx, nativeTaskPrompt(request.Prompt), WithoutPromptCache()); err != nil {
		return tools.TaskResult{Error: err.Error()}, err
	}

	select {
	case response := <-responseCh:
		if response.Error != nil {
			return tools.TaskResult{Error: response.Error.Error()}, response.Error
		}
		return tools.TaskResult{Output: strings.TrimSpace(response.Response)}, nil
	case <-runCtx.Done():
		return tools.TaskResult{Error: runCtx.Err().Error()}, runCtx.Err()
	}
}

// newChildGenie assembles an isolated Genie for a Task subagent: its
// own event bus, session, context, and a registry without the Task
// tool (no recursive task trees), while sharing the parent's prompt
// runner, skill manager, and MCP client.
//
// It composes the SAME provider functions the Wire graph uses
// (provideContextRegistry, ProvideSkillManager, ...); when adding a
// component to the Wire graph in wire.go, mirror it here.
func (e *nativeTaskExecutor) newChildGenie() (Genie, events.EventBus, error) {
	childEvents := events.NewEventBus()
	skillManager, err := ProvideSkillManager()
	if err != nil {
		return nil, nil, err
	}
	mcpClient, err := ProvideMCPClient()
	if err != nil {
		return nil, nil, err
	}

	todoManager := tools.NewTodoManager()
	toolRegistry := tools.NewDefaultRegistryWithoutTask(childEvents, todoManager, skillManager, mcpClient)
	contextRegistry := provideContextRegistry(childEvents, skillManager)
	contextManager := ctx.NewContextManager(contextRegistry)
	promptLoader := prompts.NewPromptLoader(childEvents, toolRegistry)
	personaPromptFactory := persona.NewPersonaPromptFactory(promptLoader, skillManager)
	configManager := e.parent.configMgr
	if configManager == nil {
		configManager = config.NewConfigManager()
	}
	personaManager := persona.NewDefaultPersonaManager(personaPromptFactory, configManager, childEvents)
	outputFormatter := tools.NewOutputFormatter(toolRegistry)
	sessionManager := NewSessionManager(childEvents)

	return newGenieCore(
		e.parent.promptRunner,
		sessionManager,
		contextManager,
		childEvents,
		outputFormatter,
		personaManager,
		configManager,
		toolRegistry,
	), childEvents, nil
}

func nativeTaskPrompt(prompt string) string {
	return fmt.Sprintf(`DEEP RESEARCH TASK:

You are conducting thorough research. Examine the codebase systematically, validate findings, follow related code paths, and return concise findings with concrete file references where relevant.

TASK:
%s`, strings.TrimSpace(prompt))
}
