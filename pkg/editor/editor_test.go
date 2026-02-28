package editor

import (
	"os"
	"testing"
)

func TestSelectEditor(t *testing.T) {
	// Save and restore env
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")
	defer func() {
		os.Setenv("EDITOR", origEditor)
		os.Setenv("VISUAL", origVisual)
	}()

	// Flag takes priority
	os.Setenv("EDITOR", "emacs")
	os.Setenv("VISUAL", "code")
	if got := SelectEditor("nano"); got != "nano" {
		t.Errorf("with flag: got %q, want %q", got, "nano")
	}

	// $EDITOR second
	if got := SelectEditor(""); got != "emacs" {
		t.Errorf("with EDITOR: got %q, want %q", got, "emacs")
	}

	// $VISUAL third
	os.Unsetenv("EDITOR")
	if got := SelectEditor(""); got != "code" {
		t.Errorf("with VISUAL: got %q, want %q", got, "code")
	}

	// Fallback to vi
	os.Unsetenv("VISUAL")
	if got := SelectEditor(""); got != "vi" {
		t.Errorf("fallback: got %q, want %q", got, "vi")
	}
}

func TestIsAborted(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{"empty", []byte(""), true},
		{"whitespace only", []byte("   \n\t\n  "), true},
		{"has content", []byte("apiVersion: v1\nkind: Pod\n"), false},
		{"single char", []byte("x"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAborted(tt.content); got != tt.want {
				t.Errorf("IsAborted(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
