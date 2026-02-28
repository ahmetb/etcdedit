package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/abalkan/etcdedit/pkg/codec"
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
		errorf("reading manifest: %v", err)
		os.Exit(ExitGeneralError)
	}

	// Parse YAML
	data, err := codec.FromYAML(yamlBytes)
	if err != nil {
		errorf("parsing manifest: %v", err)
		os.Exit(ExitEncodingError)
	}

	// Name mismatch warning: the etcd key determines the actual target, not the manifest name
	manifestName := codec.GetName(data)
	keyName := codec.NameFromKey(keyPath)
	if manifestName != "" && manifestName != keyName {
		warnf("Manifest metadata.name %q does not match the etcd key name %q.", manifestName, keyName)
		warnf("The resource will be written to the etcd key %s regardless of the manifest name.", keyPath)
	}

	// UID handling
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
	}

	// Connect to etcd
	client, err := newEtcdClient(ctx)
	if err != nil {
		errorf("%v", err)
		os.Exit(ExitConnectionError)
	}
	defer client.Close()

	// Check if key exists (for created vs updated messaging)
	exists, err := client.Exists(ctx, keyPath)
	if err != nil {
		errorf("checking key existence: %v", err)
		os.Exit(ExitConnectionError)
	}

	// Encode
	encoded, err := codec.EncodeForKey(keyPath, data)
	if err != nil {
		errorf("encoding: %v", err)
		os.Exit(ExitEncodingError)
	}

	// Write to etcd
	if err := client.Put(ctx, keyPath, encoded); err != nil {
		errorf("writing to etcd: %v", err)
		os.Exit(ExitConnectionError)
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
