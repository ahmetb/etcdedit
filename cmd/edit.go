package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ahmetb/etcdedit/pkg/codec"
	"github.com/ahmetb/etcdedit/pkg/editor"
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
		return err
	}
	defer client.Close()

	// Read from etcd
	result, err := client.Get(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("reading from etcd: %w", err)
	}
	if result == nil {
		return fmt.Errorf("key not found: %s", keyPath)
	}

	modRevision := result.ModRevision
	originalBytes := result.Value

	// Decode to YAML
	yamlBytes, decodeResult, err := codec.UnstructuredToYAML(keyPath, originalBytes)
	if err != nil {
		return fmt.Errorf("decoding: %w", err)
	}

	// Write YAML to temp file for editing
	tmpFile, err := os.CreateTemp("", "etcdedit-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(yamlBytes); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	// Select editor
	editorCmd := editor.SelectEditor(editorFlag)

	// Edit loop
	for {
		// Launch editor
		if err := editor.LaunchEditor(editorCmd, tmpPath); err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}

		// Read edited content
		editedBytes, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("reading edited file: %w", err)
		}

		// Check for abort
		if editor.IsAborted(editedBytes) {
			return fmt.Errorf("edit aborted (empty file)")
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
			return fmt.Errorf("writing to etcd: %w", err)
		}
		if !ok {
			return fmt.Errorf("concurrency conflict: key was modified by another process since you started editing, please retry")
		}

		// Save backup of original raw etcd value
		backupPath := saveBackup(originalBytes)
		successf("Updated %s", keyPath)
		fmt.Fprintf(os.Stderr, "Backup of original saved to: %s\n", backupPath)
		return nil
	}
}

// saveBackup saves the original raw etcd bytes to a backup file and returns the path.
func saveBackup(rawBytes []byte) string {
	filename := fmt.Sprintf("etcdedit-backup-%d.bin", time.Now().UnixMilli())
	backupPath := filepath.Join(os.TempDir(), filename)

	if err := os.WriteFile(backupPath, rawBytes, 0644); err != nil {
		warnf("failed to save backup: %v", err)
		return "(backup failed)"
	}
	return backupPath
}
