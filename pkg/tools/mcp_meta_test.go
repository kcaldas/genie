package tools

import (
	"encoding/json"
	"testing"
)

func TestLocalizedTextUnmarshalString(t *testing.T) {
	var lt LocalizedText
	if err := json.Unmarshal([]byte(`"Checking rates"`), &lt); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if lt.Default != "Checking rates" || len(lt.Locales) != 0 {
		t.Fatalf("unexpected value: %+v", lt)
	}
	if lt.IsZero() {
		t.Fatal("non-empty text reported zero")
	}
}

func TestLocalizedTextUnmarshalMap(t *testing.T) {
	var lt LocalizedText
	data := `{"en": "Checking rates", "pt-BR": "Consultando tarifas"}`
	if err := json.Unmarshal([]byte(data), &lt); err != nil {
		t.Fatalf("unmarshal map: %v", err)
	}
	if lt.Default != "" || len(lt.Locales) != 2 {
		t.Fatalf("unexpected value: %+v", lt)
	}
	if lt.Locales["pt-BR"] != "Consultando tarifas" {
		t.Fatalf("unexpected pt-BR value: %q", lt.Locales["pt-BR"])
	}
}

func TestLocalizedTextUnmarshalInvalid(t *testing.T) {
	var lt LocalizedText
	if err := json.Unmarshal([]byte(`42`), &lt); err == nil {
		t.Fatal("expected error for non-string, non-object value")
	}
}

func TestLocalizedTextIsZero(t *testing.T) {
	if !(LocalizedText{}).IsZero() {
		t.Fatal("empty value must be zero")
	}
	if (LocalizedText{Locales: map[string]string{"en": "x"}}).IsZero() {
		t.Fatal("locale map must not be zero")
	}
}

func TestLocalizedTextResolve(t *testing.T) {
	mapped := LocalizedText{Locales: map[string]string{
		"en":    "Checking rates",
		"pt-BR": "Consultando tarifas",
		"fr":    "Vérification des tarifs",
	}}

	tests := []struct {
		name   string
		text   LocalizedText
		locale string
		want   string
	}{
		{"plain string ignores locale", LocalizedText{Default: "Working"}, "pt-BR", "Working"},
		{"exact match", mapped, "pt-BR", "Consultando tarifas"},
		{"exact match is case-insensitive", mapped, "PT-br", "Consultando tarifas"},
		{"underscore tag matches hyphen key", mapped, "pt_BR", "Consultando tarifas"},
		{"language-prefix match", mapped, "pt", "Consultando tarifas"},
		{"regional variant falls back to language", mapped, "fr-CA", "Vérification des tarifs"},
		{"unknown locale falls back to en", mapped, "de", "Checking rates"},
		{"empty locale falls back to en", mapped, "", "Checking rates"},
		{"zero value resolves empty", LocalizedText{}, "en", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.text.Resolve(tt.locale); got != tt.want {
				t.Fatalf("Resolve(%q) = %q, want %q", tt.locale, got, tt.want)
			}
		})
	}
}

func TestLocalizedTextResolveNoEnglishFallsBackToFirstSortedKey(t *testing.T) {
	lt := LocalizedText{Locales: map[string]string{
		"pt-BR": "Consultando tarifas",
		"es":    "Consultando tarifas (es)",
	}}
	if got := lt.Resolve("de"); got != "Consultando tarifas (es)" {
		t.Fatalf("Resolve(de) = %q, want first sorted entry", got)
	}
}
