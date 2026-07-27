package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LocalizedText is owner-authored display text that accepts either a plain
// JSON string or an object keyed by BCP-47 locale tags:
//
//	"signalText": "Checking rates"
//	"signalText": {"en": "Checking rates", "pt-BR": "Consultando tarifas"}
type LocalizedText struct {
	// Default holds the text when it was configured as a plain string.
	Default string
	// Locales holds the per-locale texts when configured as an object.
	Locales map[string]string
}

func (t *LocalizedText) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*t = LocalizedText{Default: s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("localized text must be a string or an object of locale tag to string")
	}
	*t = LocalizedText{Locales: m}
	return nil
}

// IsZero reports whether no text is configured at all.
func (t LocalizedText) IsZero() bool {
	return t.Default == "" && len(t.Locales) == 0
}

// Resolve picks the best text for a BCP-47 locale tag: exact match, then
// same-language match, then English, then the plain-string default, then
// the first entry by sorted key. Matching is case-insensitive and treats
// underscores as hyphens ("pt_BR" matches "pt-BR"). Empty when nothing is
// configured.
func (t LocalizedText) Resolve(locale string) string {
	if len(t.Locales) == 0 {
		return t.Default
	}

	keys := make([]string, 0, len(t.Locales))
	for k := range t.Locales {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	first := func(match func(key string) bool) string {
		for _, k := range keys {
			if match(normalizeLocale(k)) && strings.TrimSpace(t.Locales[k]) != "" {
				return t.Locales[k]
			}
		}
		return ""
	}

	if want := normalizeLocale(locale); want != "" {
		if v := first(func(k string) bool { return k == want }); v != "" {
			return v
		}
		lang := strings.SplitN(want, "-", 2)[0]
		if v := first(func(k string) bool { return k == lang || strings.HasPrefix(k, lang+"-") }); v != "" {
			return v
		}
	}
	if v := first(func(k string) bool { return k == "en" || strings.HasPrefix(k, "en-") }); v != "" {
		return v
	}
	if t.Default != "" {
		return t.Default
	}
	return first(func(string) bool { return true })
}

func normalizeLocale(tag string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
}

// MCPToolMeta is the display metadata for one MCP-discovered tool,
// assembled from the server's .mcp.json entry and the tool definition the
// server advertised.
type MCPToolMeta struct {
	// Server is the .mcp.json key of the server providing the tool.
	Server string
	// ServerDisplayName is the configured human-readable server name;
	// empty when the config does not provide one.
	ServerDisplayName string
	// DisplayName is the per-tool display name: the config override when
	// present, otherwise the title the server advertised. Never used for
	// progress signals — those show SignalText or the raw tool name.
	DisplayName string
	// SignalText is the configured progress text: the per-tool text when
	// present, otherwise the server-level text. Zero when neither is
	// configured.
	SignalText LocalizedText
}
