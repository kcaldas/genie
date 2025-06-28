package controllers

import (
	"fmt"
	"strings"
)

type SlashCommandHandler struct {
	commands map[string]CommandFunc
	aliases  map[string]string
}

type CommandFunc func(args []string) error

func NewSlashCommandHandler() *SlashCommandHandler {
	return &SlashCommandHandler{
		commands: make(map[string]CommandFunc),
		aliases:  make(map[string]string),
	}
}

func (h *SlashCommandHandler) RegisterCommand(name string, fn CommandFunc) {
	h.commands[name] = fn
}

func (h *SlashCommandHandler) RegisterAlias(alias, command string) {
	h.aliases[alias] = command
}

func (h *SlashCommandHandler) HandleCommand(command string, args []string) error {
	cmd := strings.TrimPrefix(command, "/")
	
	if actualCmd, ok := h.aliases[cmd]; ok {
		cmd = actualCmd
	}
	
	if fn, ok := h.commands[cmd]; ok {
		return fn(args)
	}
	
	return fmt.Errorf("unknown command: /%s", cmd)
}

func (h *SlashCommandHandler) GetAvailableCommands() []string {
	var commands []string
	for cmd := range h.commands {
		commands = append(commands, cmd)
	}
	return commands
}

func (h *SlashCommandHandler) GetCommandHelp() map[string]string {
	help := map[string]string{
		"help":   "Show this help message",
		"clear":  "Clear the conversation history",
		"debug":  "Toggle debug panel visibility",
		"config": "Open configuration menu",
		"exit":   "Exit the application",
		"quit":   "Exit the application (alias for /exit)",
		"theme":  "Change the color theme",
		"focus":  "Focus on a specific panel (messages, input, debug)",
		"toggle": "Toggle between layout modes (normal/half/full)",
		"layout": "Set layout mode (normal, half, full)",
		"export": "Export conversation to file",
		"save":   "Save current conversation",
	}
	
	return help
}