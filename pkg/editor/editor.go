package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SelectEditor returns the editor command to use based on priority:
// 1. flagValue (from --editor flag)
// 2. $EDITOR env var
// 3. $VISUAL env var
// 4. "vi" fallback
func SelectEditor(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vi"
}

// LaunchEditor opens the specified editor with the given file path.
// It connects the editor's stdin/stdout/stderr to the current terminal.
// Returns an error if the editor exits with non-zero status.
func LaunchEditor(editor, filePath string) error {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("empty editor command")
	}

	args := append(parts[1:], filePath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}
	return nil
}

// IsAborted returns true if the user has aborted editing.
// Abort is detected by the file being empty.
func IsAborted(editedContent []byte) bool {
	return len(strings.TrimSpace(string(editedContent))) == 0
}
