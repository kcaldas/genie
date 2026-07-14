package toolctx

import (
	"context"
	"reflect"
	"testing"
)

func TestStringRoundTrips(t *testing.T) {
	cases := []struct {
		name string
		set  func(context.Context, string) context.Context
		get  func(context.Context) (string, bool)
	}{
		{"WorkingDir", WithWorkingDir, WorkingDir},
		{"GenieHome", WithGenieHome, GenieHome},
		{"CommitAuthorName", WithCommitAuthorName, CommitAuthorName},
		{"CommitAuthorEmail", WithCommitAuthorEmail, CommitAuthorEmail},
		{"Persona", WithPersona, Persona},
		{"SessionID", WithSessionID, SessionID},
		{"ExecutionID", WithExecutionID, ExecutionID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.set(context.Background(), "some-value")
			got, ok := tc.get(ctx)
			if !ok || got != "some-value" {
				t.Fatalf("got (%q, %v), want (%q, true)", got, ok, "some-value")
			}

			// Empty string is still "set": ok must be true.
			ctx = tc.set(context.Background(), "")
			got, ok = tc.get(ctx)
			if !ok || got != "" {
				t.Fatalf("empty value: got (%q, %v), want (\"\", true)", got, ok)
			}
		})
	}
}

func TestStringAbsentKeys(t *testing.T) {
	ctx := context.Background()
	getters := map[string]func(context.Context) (string, bool){
		"WorkingDir":        WorkingDir,
		"GenieHome":         GenieHome,
		"CommitAuthorName":  CommitAuthorName,
		"CommitAuthorEmail": CommitAuthorEmail,
		"Persona":           Persona,
		"SessionID":         SessionID,
		"ExecutionID":       ExecutionID,
	}
	for name, get := range getters {
		if got, ok := get(ctx); ok || got != "" {
			t.Errorf("%s on empty context: got (%q, %v), want (\"\", false)", name, got, ok)
		}
	}
}

func TestSliceRoundTrips(t *testing.T) {
	cases := []struct {
		name string
		set  func(context.Context, []string) context.Context
		get  func(context.Context) ([]string, bool)
	}{
		{"AllowedDirs", WithAllowedDirs, AllowedDirs},
		{"DeniedPaths", WithDeniedPaths, DeniedPaths},
		{"ReadOnlyPaths", WithReadOnlyPaths, ReadOnlyPaths},
	}
	want := []string{"/a", "b/**", "*.yaml"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.set(context.Background(), want)
			got, ok := tc.get(ctx)
			if !ok || !reflect.DeepEqual(got, want) {
				t.Fatalf("got (%v, %v), want (%v, true)", got, ok, want)
			}
		})
	}
}

func TestSliceAbsentKeys(t *testing.T) {
	ctx := context.Background()
	getters := map[string]func(context.Context) ([]string, bool){
		"AllowedDirs":   AllowedDirs,
		"DeniedPaths":   DeniedPaths,
		"ReadOnlyPaths": ReadOnlyPaths,
	}
	for name, get := range getters {
		if got, ok := get(ctx); ok || got != nil {
			t.Errorf("%s on empty context: got (%v, %v), want (nil, false)", name, got, ok)
		}
	}
}

func TestKeysDoNotCollide(t *testing.T) {
	ctx := WithWorkingDir(context.Background(), "/work")
	ctx = WithGenieHome(ctx, "/home")
	ctx = WithPersona(ctx, "engineer")

	if got, _ := WorkingDir(ctx); got != "/work" {
		t.Errorf("WorkingDir = %q, want %q", got, "/work")
	}
	if got, _ := GenieHome(ctx); got != "/home" {
		t.Errorf("GenieHome = %q, want %q", got, "/home")
	}
	if got, _ := Persona(ctx); got != "engineer" {
		t.Errorf("Persona = %q, want %q", got, "engineer")
	}
	if _, ok := SessionID(ctx); ok {
		t.Error("SessionID should not be set")
	}
}
