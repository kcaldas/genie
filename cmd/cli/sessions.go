package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kcaldas/genie/pkg/session"
	"github.com/spf13/cobra"
)

const sessionsDir = ".genie/sessions"

// NewSessionsCommand reads recorded session files. It deliberately does
// not boot Genie: its own PersistentPreRunE overrides the root's, so
// listing and showing traces needs no provider config and starts
// instantly.
func NewSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "sessions",
		Short:             "Inspect recorded session traces (see --trace)",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List recorded sessions in ./" + sessionsDir,
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := sessionFiles()
			if err != nil {
				return err
			}
			if len(files) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no recorded sessions in ./%s (run genie with --trace)\n", sessionsDir)
				return nil
			}
			for _, file := range files {
				header, entries, err := readSessionFile(file)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  (unreadable: %v)\n", filepath.Base(file), err)
					continue
				}
				turns := session.Turns(entries)
				info, _ := os.Stat(file)
				size := int64(0)
				if info != nil {
					size = info.Size()
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %2d turn(s)  %6.1fKB  %s\n",
					filepath.Base(file), len(turns), float64(size)/1024, header.ID)
			}
			return nil
		},
	}

	var turnFlag int
	show := &cobra.Command{
		Use:   "show [file]",
		Short: "Show a session; --turn N reconstructs that turn's full model input",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := resolveSessionFile(args)
			if err != nil {
				return err
			}
			header, entries, err := readSessionFile(file)
			if err != nil {
				return err
			}
			turns := session.Turns(entries)
			out := cmd.OutOrStdout()

			if turnFlag > 0 {
				if turnFlag > len(turns) {
					return fmt.Errorf("session has %d turn(s), no turn %d", len(turns), turnFlag)
				}
				renderTurnFull(out, turnFlag, turns[turnFlag-1])
				return nil
			}

			fmt.Fprintf(out, "# session %s\n  %s — %s\n\n", header.ID, header.Timestamp, header.Cwd)
			for i, turn := range turns {
				renderTurnSummary(out, i+1, turn)
			}
			fmt.Fprintf(out, "use --turn N for a turn's full reconstructed model input\n")
			return nil
		},
	}
	show.Flags().IntVar(&turnFlag, "turn", 0, "reconstruct and print this turn's complete model input")

	cmd.AddCommand(list, show)
	return cmd
}

// renderTurnSummary prints the composition table and the turn's records.
func renderTurnSummary(out interface{ Write([]byte) (int, error) }, n int, turn session.Turn) {
	fmt.Fprintf(out, "━━━ turn %d · %s ━━━\n", n, timeOnly(turn.Context.Timestamp))
	for _, name := range partOrder(turn) {
		ref, _ := turn.Context.Payload["parts"].(map[string]any)[name].(map[string]any)
		changed := "·"
		if isChanged(ref) {
			changed = "✏️"
		}
		fmt.Fprintf(out, "  %-24s %8db  %s\n", name, intFrom(ref, "bytes"), changed)
	}
	for _, warning := range turn.Warnings {
		fmt.Fprintf(out, "  ⚠ %s\n", warning)
	}
	for _, entry := range turn.Entries {
		switch entry.Type {
		case "tool_call":
			status := "✅"
			if success, _ := entry.Payload["success"].(bool); !success {
				status = "❌"
			}
			fmt.Fprintf(out, "  🔧 %s %s\n", entry.Payload["tool"], status)
		case "thinking":
			fmt.Fprintf(out, "  💭 %s\n", excerpt(fieldText(entry.Payload["text"]), 100))
		case "message":
			fmt.Fprintf(out, "  💬 user: %s\n     asst: %s\n",
				excerpt(fieldText(entry.Payload["user"]), 100),
				excerpt(fieldText(entry.Payload["assistant"]), 100))
		case "error":
			fmt.Fprintf(out, "  ❌ %s\n", excerpt(fieldText(entry.Payload["error"]), 160))
		case "prune":
			fmt.Fprintf(out, "  ✂️ prune %v: kept %v/%v\n", entry.Payload["strategy"], entry.Payload["kept"], entry.Payload["total"])
		default:
			fmt.Fprintf(out, "  · %s\n", entry.Type)
		}
	}
	fmt.Fprintln(out)
}

// renderTurnFull prints one turn's completely reconstructed model input
// in wire order, then its records verbatim.
func renderTurnFull(out interface{ Write([]byte) (int, error) }, n int, turn session.Turn) {
	fmt.Fprintf(out, "━━━ turn %d — full reconstructed model input · %s ━━━\n\n", n, timeOnly(turn.Context.Timestamp))
	for _, warning := range turn.Warnings {
		fmt.Fprintf(out, "⚠ %s\n", warning)
	}
	for i, name := range partOrder(turn) {
		content, ok := turn.Parts[name]
		if !ok {
			continue
		}
		fmt.Fprintf(out, "─── %d. %s (%db) %s\n%s\n\n", i+1, name, len(content), strings.Repeat("─", 30), content)
	}
	for _, entry := range turn.Entries {
		switch entry.Type {
		case "tool_call":
			fmt.Fprintf(out, "─── 🔧 %s (success=%v) %s\n", entry.Payload["tool"], entry.Payload["success"], strings.Repeat("─", 30))
			if params := fieldText(entry.Payload["params"]); params != "" {
				fmt.Fprintf(out, "params: %s\n", params)
			}
			if result := fieldText(entry.Payload["result"]); result != "" {
				fmt.Fprintf(out, "result: %s\n", result)
			}
			fmt.Fprintln(out)
		case "thinking":
			fmt.Fprintf(out, "─── 💭 thinking %s\n%s\n\n", strings.Repeat("─", 30), fieldText(entry.Payload["text"]))
		case "message":
			fmt.Fprintf(out, "─── 💬 exchange (model %v) %s\nuser: %s\n\nassistant: %s\n\n",
				entry.Payload["model"], strings.Repeat("─", 30),
				fieldText(entry.Payload["user"]), fieldText(entry.Payload["assistant"]))
		case "error":
			fmt.Fprintf(out, "─── ❌ error %s\n%s\n\n", strings.Repeat("─", 30), fieldText(entry.Payload["error"]))
		}
	}
}

func sessionFiles() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(sessionsDir, "*.session.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches) // timestamp-named → chronological
	return matches, nil
}

func resolveSessionFile(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	files, err := sessionFiles()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no recorded sessions in ./%s (run genie with --trace)", sessionsDir)
	}
	return files[len(files)-1], nil
}

func readSessionFile(path string) (session.Header, []session.GenericEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return session.Header{}, nil, err
	}
	defer file.Close()
	return session.ReadSession(file)
}

func partOrder(turn session.Turn) []string {
	if len(turn.Order) > 0 {
		return turn.Order
	}
	names := make([]string, 0, len(turn.Parts))
	for name := range turn.Parts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func fieldText(raw any) string {
	field, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	text, _ := field["text"].(string)
	if truncated, _ := field["truncated"].(bool); truncated {
		text += " …[truncated]"
	}
	if redacted, _ := field["redacted"].(bool); redacted {
		text = "[redacted]"
	}
	return text
}

func excerpt(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// timeOnly trims an RFC3339Nano timestamp to its clock time.
func timeOnly(timestamp string) string {
	if len(timestamp) >= 19 {
		return timestamp[11:19]
	}
	return timestamp
}

func isChanged(ref map[string]any) bool {
	changed, _ := ref["changed"].(bool)
	return changed
}

func intFrom(ref map[string]any, key string) int {
	if value, ok := ref[key].(float64); ok {
		return int(value)
	}
	return 0
}
