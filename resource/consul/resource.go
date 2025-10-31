// Package consul provides Consul KV store-based implementation of the configuration resource interface
package consul

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-leo/gonfig/format"
	"github.com/go-leo/gonfig/resource"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/go-hclog"

	"google.golang.org/protobuf/types/known/structpb"
)

var _ resource.Resource = (*Resource)(nil)

// Resource represents a configuration resource stored in Consul KV store
type Resource struct {
	// client is the Consul API client used to interact with the Consul server
	client *api.Client
	// key is the path to the configuration in the Consul KV store
	key string
	// formatter is used for parsing config data into structured format
	formatter format.Formatter
	// pre is atomic storage for previous configuration data to detect changes
	pre atomic.Value
}

// Load retrieves and parses the configuration from Consul KV store
// It fetches the value at the configured key path and uses the formatter to parse it
// Parameters:
//   - ctx: Context for cancellation and timeouts
//
// Returns:
//   - *structpb.Struct: Parsed configuration data
//   - error: Any error that occurred during loading or parsing
func (r *Resource) Load(ctx context.Context) (*structpb.Struct, error) {
	data, err := r.load(ctx)
	if err != nil {
		return nil, err
	}
	parsed, err := r.formatter.Parse(data)
	if err != nil {
		return nil, err
	}
	r.pre.Store(data)
	return parsed, nil
}

// load is an internal helper function to fetch raw data from Consul KV store
// Parameters:
//   - ctx: Context for cancellation and timeouts
//
// Returns:
//   - []byte: Raw configuration data from Consul
//   - error: Any error that occurred while fetching from Consul
func (r *Resource) load(ctx context.Context) ([]byte, error) {
	pair, _, err := r.client.KV().Get(r.key, new(api.QueryOptions).WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, fmt.Errorf("gonfig: consul key %q not found", r.key)
	}
	return pair.Value, nil
}

// Watch sets up a watcher for configuration changes in Consul using Consul's watch mechanism
// It creates a watch plan that monitors changes to the specific key and notifies subscribers
// Parameters:
//   - ctx: Context for cancellation
//   - notifyFunc: Callback function for configuration updates
//   - errFunc: Callback function for error reporting
//
// Returns:
//   - resource.StopFunc: Function to stop watching
//   - error: Any immediate error during setup
func (r *Resource) Watch(ctx context.Context, notifyFunc resource.NotifyFunc, errFunc resource.ErrFunc) (resource.StopFunc, error) {
	// Validate notify function
	if notifyFunc == nil {
		return nil, fmt.Errorf("gonfig: notifyFunc is nil")
	}

	// Set default error handler if none provided
	if errFunc == nil {
		errFunc = func(err error) {
			slog.Error("gonfig: failed to watch consul", slog.String("error", err.Error()))
		}
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Prepare watch parameters for key monitoring
	params := map[string]any{
		"type": "key",
		"key":  r.key,
	}

	// Create watch plan
	plan, err := watch.Parse(params)
	if err != nil {
		return nil, err
	}

	// Set up handler for watch events
	plan.Handler = func(idx uint64, raw interface{}) {
		// Validate the received data
		if raw == nil {
			errFunc(fmt.Errorf("gonfig: consul watch returned unexpected type %T", raw))
			return
		}

		// Type assert to KVPair
		pair, ok := raw.(*api.KVPair)
		if !ok {
			return
		}

		// Get the data value
		data := pair.Value

		// Compare with previous data to avoid unnecessary notifications
		preData := r.pre.Load()
		if preData != nil && bytes.Equal(preData.([]byte), data) {
			return
		}

		// Parse new configuration data
		newValue, err := r.formatter.Parse(data)
		if err != nil {
			errFunc(err)
			return
		}

		// Notify subscribers of the change
		notifyFunc(newValue)

		// Store new data for future comparisons
		r.pre.Store(data)
	}

	// Start watching in a separate goroutine
	go func() {
		// Create custom logger that forwards errors to errFunc
		logger := &consulLogger{
			Logger:  hclog.NewNullLogger(),
			errFunc: errFunc,
		}

		// Run the watch plan with the Consul client
		if err := plan.RunWithClientAndHclog(r.client, logger); err != nil {
			errFunc(err)
		}
	}()

	// Create stop function with sync.Once to ensure it's only called once
	var onceStop sync.Once
	stop := func(ctx context.Context) error {
		onceStop.Do(func() {
			plan.Stop()
		})
		return nil
	}

	// Start a goroutine to handle context cancellation
	go func() {
		select {
		case <-ctx.Done():
			stop(context.Background())
			errFunc(ctx.Err())
		}
	}()

	return stop, nil
}

// consulLogger is a custom logger that forwards errors to the error function
type consulLogger struct {
	hclog.Logger
	errFunc resource.ErrFunc
}

// Error implements the hclog.Logger interface and forwards errors to errFunc
// It formats the error message and arguments into a single error and passes it to errFunc
func (l *consulLogger) Error(msg string, args ...interface{}) {
	buf := bytes.NewBufferString(msg)
	for i := 0; i < len(args); i += 2 {
		buf.WriteString(fmt.Sprintf(" %v=%v", args[i], args[i+1]))
	}
	l.errFunc(errors.New(buf.String()))
}

// New creates a new Consul configuration resource
// It validates the key extension and finds an appropriate formatter
// Parameters:
//   - client: Consul API client
//   - key: Path to the configuration in Consul KV store
//
// Returns:
//   - *Resource: New Consul resource instance
//   - error: Any error during initialization
func New(client *api.Client, key string) (*Resource, error) {
	// Extract key extension
	ext := strings.TrimPrefix(filepath.Ext(key), ".")
	if ext == "" {
		return nil, fmt.Errorf("config: key extension is empty")
	}

	// Find appropriate formatter for the key extension
	formatter, ok := format.GetFormatter(ext)
	if !ok {
		return nil, fmt.Errorf("config: not found formatter for %s", ext)
	}

	// Return new resource instance
	return &Resource{
		client:    client,
		key:       key,
		formatter: formatter,
	}, nil
}
