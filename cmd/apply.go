package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ahmetb/etcdedit/pkg/codec"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var manifestFile string

var applyCmd = &cobra.Command{
	Use:   "apply <key>",
	Short: "Apply a Kubernetes manifest to an etcd key",
	Args:  cobra.ExactArgs(1),
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().StringVarP(&manifestFile, "file", "f", "", "path to YAML manifest file (required)")
	applyCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	keyPath := args[0]
	ctx := context.Background()

	// Read manifest file
	yamlBytes, err := os.ReadFile(manifestFile)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// Parse YAML
	data, err := codec.FromYAML(yamlBytes)
	if err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	// Connect to etcd
	client, err := newEtcdClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// Check if key exists (for created vs updated messaging and UID handling)
	exists, err := client.Exists(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("checking key existence: %w", err)
	}

	// Reconcile metadata.name with the etcd key — the key is authoritative.
	// The API server computes the storage key from the object's metadata.name,
	// so a mismatch makes the resource invisible to Kubernetes.
	keyName := codec.NameFromKey(keyPath)
	manifestName := codec.GetName(data)
	if manifestName == "" {
		fmt.Fprintf(os.Stderr, "Setting metadata.name to %q (derived from etcd key).\n", keyName)
	} else if manifestName != keyName {
		warnf("Manifest metadata.name %q does not match etcd key name %q.", manifestName, keyName)
		warnf("Overriding metadata.name to %q so the API server can find this resource.", keyName)
	}
	codec.SetName(data, keyName)

	// Reconcile metadata.namespace with the etcd key when detectable.
	keyNamespace := codec.NamespaceFromKey(keyPath)
	if keyNamespace != "" {
		manifestNamespace := codec.GetNamespace(data)
		if manifestNamespace == "" {
			fmt.Fprintf(os.Stderr, "Setting metadata.namespace to %q (derived from etcd key).\n", keyNamespace)
		} else if manifestNamespace != keyNamespace {
			warnf("Manifest metadata.namespace %q does not match etcd key namespace %q.", manifestNamespace, keyNamespace)
			warnf("Overriding metadata.namespace to %q so the API server can find this resource.", keyNamespace)
		}
		codec.SetNamespace(data, keyNamespace)
	}

	// UID handling — only for new resources
	if !exists {
		if uid := codec.GetUID(data); uid != "" {
			warnf("Manifest contains metadata.uid: %s", uid)
			warnf("Reusing a UID from another resource can cause conflicts in Kubernetes.")

			if promptYesNo("Reuse this UID?") {
				fmt.Fprintf(os.Stderr, "Keeping existing UID: %s\n", uid)
			} else {
				newUID := uuid.New().String()
				codec.SetUID(data, newUID)
				fmt.Fprintf(os.Stderr, "Generated new UID: %s\n", newUID)
			}
		} else {
			// Auto-generate UID for new resources that don't have one
			newUID := uuid.New().String()
			codec.SetUID(data, newUID)
		}
	}

	// Encode
	encoded, err := codec.EncodeForKey(keyPath, data)
	if err != nil {
		return fmt.Errorf("encoding: %w", err)
	}

	// Write to etcd
	if err := client.Put(ctx, keyPath, encoded); err != nil {
		return fmt.Errorf("writing to etcd: %w", err)
	}

	if exists {
		successf("Updated %s", keyPath)
	} else {
		successf("Created %s", keyPath)
	}

	return nil
}

// promptYesNo asks a yes/no question and returns true for yes.
func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, colorYellow+"%s [y/N]: "+colorReset, question)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
