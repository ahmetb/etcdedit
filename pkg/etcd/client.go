package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ConnOpts holds etcd connection options, compatible with etcdctl flags.
type ConnOpts struct {
	Endpoints           []string
	CACert              string
	Cert                string
	Key                 string
	User                string
	Password            string
	InsecureSkipTLS     bool
	DialTimeout         time.Duration
}

// GetResult holds the result of an etcd Get operation.
type GetResult struct {
	Value       []byte
	ModRevision int64
}

// Client wraps an etcd v3 client.
type Client struct {
	kv clientv3.KV
	c  *clientv3.Client
}

// NewClient creates a new etcd client from connection options.
func NewClient(ctx context.Context, opts ConnOpts) (*Client, error) {
	cfg := clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: opts.DialTimeout,
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 2 * time.Second
	}

	// TLS configuration
	if opts.CACert != "" || opts.Cert != "" {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: opts.InsecureSkipTLS,
		}

		if opts.CACert != "" {
			caCert, err := os.ReadFile(opts.CACert)
			if err != nil {
				return nil, fmt.Errorf("read CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA cert")
			}
			tlsCfg.RootCAs = pool
		}

		if opts.Cert != "" && opts.Key != "" {
			cert, err := tls.LoadX509KeyPair(opts.Cert, opts.Key)
			if err != nil {
				return nil, fmt.Errorf("load client cert/key: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}

		cfg.TLS = tlsCfg
	} else if opts.InsecureSkipTLS {
		cfg.TLS = &tls.Config{InsecureSkipVerify: true}
	}

	// Auth
	if opts.User != "" {
		parts := strings.SplitN(opts.User, ":", 2)
		cfg.Username = parts[0]
		if len(parts) == 2 {
			cfg.Password = parts[1]
		}
	}
	if opts.Password != "" {
		cfg.Password = opts.Password
	}

	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to etcd: %w", err)
	}

	// Probe connectivity so --dial-timeout is actually honored.
	// The etcd v3 client connects lazily, so without this check the
	// client would hang indefinitely when the endpoint is unreachable.
	probeCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	_, err = c.Status(probeCtx, opts.Endpoints[0])
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("connect to etcd: %w", err)
	}

	return &Client{kv: c.KV, c: c}, nil
}

// Get reads a key from etcd. Returns the value and mod_revision.
func (c *Client) Get(ctx context.Context, key string) (*GetResult, error) {
	resp, err := c.kv.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("etcd get: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil // key not found
	}

	kv := resp.Kvs[0]
	return &GetResult{
		Value:       kv.Value,
		ModRevision: kv.ModRevision,
	}, nil
}

// Put writes a value to etcd unconditionally.
func (c *Client) Put(ctx context.Context, key string, value []byte) error {
	_, err := c.kv.Put(ctx, key, string(value))
	if err != nil {
		return fmt.Errorf("etcd put: %w", err)
	}
	return nil
}

// PutIfUnmodified writes a value only if the key's mod_revision matches expected.
// Uses an etcd transaction for optimistic concurrency control.
// Returns true if the write succeeded, false if the revision changed.
func (c *Client) PutIfUnmodified(ctx context.Context, key string, value []byte, expectedModRevision int64) (bool, error) {
	txnResp, err := c.kv.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(key), "=", expectedModRevision)).
		Then(clientv3.OpPut(key, string(value))).
		Commit()
	if err != nil {
		return false, fmt.Errorf("etcd txn: %w", err)
	}
	return txnResp.Succeeded, nil
}

// Exists checks if a key exists in etcd.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	resp, err := c.kv.Get(ctx, key, clientv3.WithCountOnly())
	if err != nil {
		return false, fmt.Errorf("etcd get (count): %w", err)
	}
	return resp.Count > 0, nil
}

// Close closes the etcd client connection.
func (c *Client) Close() error {
	return c.c.Close()
}
