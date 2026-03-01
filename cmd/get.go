package cmd

import (
	"context"
	"fmt"

	"github.com/ahmetb/etcdedit/pkg/codec"
	"github.com/spf13/cobra"
)

var outputFormat string

var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Decode and print a Kubernetes resource from etcd",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func init() {
	getCmd.Flags().StringVarP(&outputFormat, "output", "o", "yaml", "output format: yaml or json")
	rootCmd.AddCommand(getCmd)
}

func runGet(cmd *cobra.Command, args []string) error {
	keyPath := args[0]
	ctx := context.Background()

	client, err := newEtcdClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := client.Get(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("reading from etcd: %w", err)
	}
	if result == nil {
		return fmt.Errorf("key not found: %s", keyPath)
	}

	decoded, err := codec.Decode(keyPath, result.Value)
	if err != nil {
		return fmt.Errorf("decoding: %w", err)
	}

	var output []byte
	switch outputFormat {
	case "yaml":
		output, err = codec.ToYAML(decoded.Data)
	case "json":
		output, err = codec.ToJSON(decoded.Data)
	default:
		return fmt.Errorf("unsupported output format: %s (use yaml or json)", outputFormat)
	}
	if err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}

	fmt.Print(string(output))
	return nil
}
