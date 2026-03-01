package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ahmetb/etcdedit/pkg/etcd"
	"github.com/spf13/cobra"
)

// Connection flag variables
var (
	endpoints          string
	cacert             string
	cert               string
	key                string
	user               string
	password           string
	insecureSkipTLS    bool
	dialTimeout        time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "etcdedit",
	Short: "Directly edit Kubernetes resources stored in etcd",
	Long: `etcdedit is a CLI tool for directly editing Kubernetes resources stored in etcd,
bypassing the Kubernetes API server.

WARNING: Fields unknown to this build's Kubernetes library version may be dropped
during protobuf roundtrips. Compile against the same or newer Kubernetes version
as your target cluster.`,
}

func init() {
	pf := rootCmd.PersistentFlags()

	pf.StringVar(&endpoints, "endpoints", envOrDefault("ETCDCTL_ENDPOINTS", "localhost:2379"), "comma-separated etcd endpoints")
	pf.StringVar(&cacert, "cacert", envOrDefaultMulti("", "ETCDCTL_CACERT", "ETCDCTL_CA_FILE"), "CA certificate file")
	pf.StringVar(&cert, "cert", envOrDefaultMulti("", "ETCDCTL_CERT", "ETCDCTL_CERT_FILE"), "client certificate file")
	pf.StringVar(&key, "key", envOrDefaultMulti("", "ETCDCTL_KEY", "ETCDCTL_KEY_FILE"), "client key file")
	pf.StringVar(&user, "user", os.Getenv("ETCDCTL_USER"), "username[:password] for authentication")
	pf.StringVar(&password, "password", os.Getenv("ETCDCTL_PASSWORD"), "password for authentication")
	pf.BoolVar(&insecureSkipTLS, "insecure-skip-tls-verify", envBool("ETCDCTL_INSECURE_SKIP_TLS_VERIFY"), "skip TLS verification")
	pf.DurationVar(&dialTimeout, "dial-timeout", 2*time.Second, "dial timeout")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// connOpts builds ConnOpts from the current flag values.
func connOpts() etcd.ConnOpts {
	eps := strings.Split(endpoints, ",")
	return etcd.ConnOpts{
		Endpoints:       eps,
		CACert:          cacert,
		Cert:            cert,
		Key:             key,
		User:            user,
		Password:        password,
		InsecureSkipTLS: insecureSkipTLS,
		DialTimeout:     dialTimeout,
	}
}

// newEtcdClient creates a new etcd client from current flags.
func newEtcdClient(ctx context.Context) (*etcd.Client, error) {
	c, err := etcd.NewClient(ctx, connOpts())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}
	return c, nil
}

// envOrDefault returns the environment variable value or a default.
func envOrDefault(envVar, defaultVal string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultVal
}

// envOrDefaultMulti checks multiple env vars in order, returns first non-empty or default.
func envOrDefaultMulti(defaultVal string, envVars ...string) string {
	for _, ev := range envVars {
		if v := os.Getenv(ev); v != "" {
			return v
		}
	}
	return defaultVal
}

// envBool returns true if the env var is set to a truthy value.
func envBool(envVar string) bool {
	v := strings.ToLower(os.Getenv(envVar))
	return v == "true" || v == "1" || v == "yes"
}

// ANSI color helpers
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
)

func warnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorYellow+"WARNING: "+format+colorReset+"\n", args...)
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorRed+"ERROR: "+format+colorReset+"\n", args...)
}

func successf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorGreen+format+colorReset+"\n", args...)
}
