package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/abalkan/etcdedit/pkg/codec"
	"github.com/abalkan/etcdedit/pkg/editor"
	"github.com/spf13/cobra"
)

var editorFlag string

var editCmd = &cobra.Command{
	Use:   "edit <key>",
	Short: "Edit a Kubernetes resource at the specified etcd key",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

func init() {
	editCmd.Flags().StringVar(&editorFlag, "editor", "", "editor command (defaults to $EDITOR, $VISUAL, or vi)")
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	keyPath := args[0]
	ctx := context.Background()

	client, err := newEtcdClient(ctx)
	if err != nil {
		errorf("%v", err)
		os.Exit(ExitConnectionError)
	}
	defer client.Close()

	// Read from etcd
	result, err := client.Get(ctx, keyPath)
	if err != nil {
		errorf("reading from etcd: %v", err)
		os.Exit(ExitConnectionError)
	}
	if result == nil {
		errorf("key not found: %s", keyPath)
		os.Exit(ExitKeyNotFound)
	}

	modRevision := result.ModRevision
	originalBytes := result.Value

	// Decode to YAML
	yamlBytes, decodeResult, err := codec.UnstructuredToYAML(keyPath, originalBytes)
	if err != nil {
		errorf("decoding: %v", err)
		os.Exit(ExitEncodingError)
	}

	// Write YAML to temp file for editing
	tmpFile, err := os.CreateTemp("", "etcdedit-*.yaml")
	if err != nil {
		errorf("creating temp file: %v", err)
		os.Exit(ExitGeneralError)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(yamlBytes); err != nil {
		tmpFile.Close()
		errorf("writing temp file: %v", err)
		os.Exit(ExitGeneralError)
	}
	tmpFile.Close()

	// Select editor
	editorCmd := editor.SelectEditor(editorFlag)

	// Edit loop
	for {
		// Launch editor
		if err := editor.LaunchEditor(editorCmd, tmpPath); err != nil {
			errorf("editor failed: %v", err)
			os.Exit(ExitEditorFailed)
		}

		// Read edited content
		editedBytes, err := os.ReadFile(tmpPath)
		if err != nil {
			errorf("reading edited file: %v", err)
			os.Exit(ExitGeneralError)
		}

		// Check for abort
		if editor.IsAborted(editedBytes) {
			fmt.Fprintln(os.Stderr, "Edit aborted (empty file).")
			os.Exit(ExitEditorFailed)
		}

		// Try to encode back
		encoded, err := codec.YAMLToEncoded(keyPath, editedBytes, decodeResult.IsProtobuf)
		if err != nil {
			errorf("encoding failed: %v", err)
			warnf("Re-opening editor. Save an empty file to abort.")
			continue // re-open editor
		}

		// Optimistic concurrency check via transaction
		ok, err := client.PutIfUnmodified(ctx, keyPath, encoded, modRevision)
		if err != nil {
			errorf("writing to etcd: %v", err)
			os.Exit(ExitConnectionError)
		}
		if !ok {
			errorf("concurrency conflict: key was modified by another process since you started editing")
			errorf("Please re-run the edit command to reload the latest version.")
			os.Exit(ExitConcurrencyConflict)
		}

		// Save backup of original value
		backupPath := saveBackup(keyPath, yamlBytes)
		successf("Updated %s", keyPath)
		fmt.Fprintf(os.Stderr, "Backup of original saved to: %s\n", backupPath)
		return nil
	}
}

// saveBackup saves the original YAML to a backup temp file and returns the path.
func saveBackup(keyPath string, yamlBytes []byte) string {
	hash := sha256.Sum256([]byte(keyPath))
	filename := fmt.Sprintf("etcdedit-backup-%x-%d.yaml", hash[:8], time.Now().Unix())
	backupPath := filepath.Join(os.TempDir(), filename)

	if err := os.WriteFile(backupPath, yamlBytes, 0600); err != nil {
		warnf("failed to save backup: %v", err)
		return "(backup failed)"
	}
	return backupPath
}
