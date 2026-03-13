package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLaunchEditor_EmptyCommand(t *testing.T) {
	err := LaunchEditor("", "/tmp/dummy")
	if err == nil {
		t.Fatal("expected error for empty editor command")
	}
	if err.Error() != "empty editor command" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLaunchEditor_SingleWord(t *testing.T) {
	// "true" is a command that always exits 0
	err := LaunchEditor("true", "/tmp/dummy")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestLaunchEditor_SingleWordFailure(t *testing.T) {
	err := LaunchEditor("false", "/tmp/dummy")
	if err == nil {
		t.Fatal("expected error from false command")
	}
}

func TestLaunchEditor_MultiWordCommand(t *testing.T) {
	// Verify that a multi-word editor command is handled via sh -c
	// and the file path is passed as $1 to the shell.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "testfile.txt")
	if err := os.WriteFile(filePath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a helper script that writes "edited" into the file it receives
	// as its last positional argument (like a real editor would).
	scriptPath := filepath.Join(dir, "editor.sh")
	// The script uses the last argument: shift through all args to find it.
	script := "#!/bin/sh\nfor f; do :; done\necho edited > \"$f\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Multi-word command: script + a flag, like "vim --noplugin"
	editorCmd := scriptPath + " --noplugin"

	err := LaunchEditor(editorCmd, filePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if got := string(data); got != "edited\n" {
		t.Fatalf("expected file to be modified to 'edited', got: %q", got)
	}
}

func TestLaunchEditor_ShellQuotingWithSpacesInPath(t *testing.T) {
	// Verify that file paths with spaces work correctly through sh -c
	// because the file path is passed as $1 and quoted in the shell command.
	dir := t.TempDir()
	subdir := filepath.Join(dir, "path with spaces")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(subdir, "test file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a helper script that writes "edited" into its last argument
	scriptPath := filepath.Join(dir, "editor.sh")
	script := "#!/bin/sh\nfor f; do :; done\necho edited > \"$f\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Multi-word command so it goes through sh -c path
	editorCmd := scriptPath + " --flag"

	err := LaunchEditor(editorCmd, filePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != "edited\n" {
		t.Fatalf("expected 'edited', got: %q", string(data))
	}
}

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
