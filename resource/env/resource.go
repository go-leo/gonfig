// Package env provides environment variable-based implementation of the configuration resource interface
package env

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/exp/slices"

	"github.com/go-leo/gonfig/format"
	"github.com/go-leo/gonfig/resource"
	"google.golang.org/protobuf/types/known/structpb"
)

var _ resource.Resource = (*Resource)(nil)

// Resource represents a configuration resource loaded from environment variables
type Resource struct {
	// prefix is used to filter environment variables (only variables with this prefix are considered)
	prefix string
	// interval defines how often to check for environment variable changes
	interval time.Duration
	// formatter is used for parsing environment variables into structured data
	formatter format.Formatter
	// pre is atomic storage for previous configuration data to detect changes
	pre atomic.Value
}

// Load retrieves and parses environment variables with the specified prefix
// It collects all environment variables that start with the configured prefix,
// sorts them for consistency, and uses the formatter to convert them to structpb.Struct format
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

// load collects and prepares environment variables data
// It filters environment variables by the configured prefix, sorts them,
// and joins them with newlines to create a consistent representation
// Parameters:
//   - ctx: Context for cancellation and timeouts
//
// Returns:
//   - []byte: Formatted environment variables data
//   - error: Error if no environment variables found with the prefix
func (r *Resource) load(ctx context.Context) ([]byte, error) {
	var environs [][]byte
	// Filter environment variables by prefix
	for _, environ := range os.Environ() {
		if strings.HasPrefix(environ, r.prefix) {
			environs = append(environs, []byte(environ))
		}
	}
	if len(environs) <= 0 {
		return nil, fmt.Errorf("config: no environment variables found with prefix %s", r.prefix)
	}
	// Sort and join variables for consistent output
	slices.SortFunc(environs, bytes.Compare)
	return bytes.Join(environs, []byte("\n")), nil
}

// Watch monitors environment variables for changes at regular intervals
// It periodically checks for changes in environment variables with the specified prefix
// and notifies subscribers when changes are detected
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
			slog.Error("gonfig: failed to watch env", slog.String("error", err.Error()))
		}
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Create stop channel and stop function
	stopC := make(chan struct{})
	stop := func(ctx context.Context) error {
		close(stopC)
		return nil
	}

	// Start watching in a separate goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				// Context cancelled, exit goroutine
				return
			case <-stopC:
				// Stop signal received, exit goroutine
				return
			case <-time.After(r.interval):
				// Check for changes at regular intervals
				data, err := r.load(ctx)
				if err != nil {
					errFunc(err)
					continue
				}
				// Compare with previous data to avoid unnecessary notifications
				preData := r.pre.Load()
				if preData != nil && bytes.Equal(preData.([]byte), data) {
					continue // Skip if no changes
				}
				// Parse new configuration data
				newValue, err := r.formatter.Parse(data)
				if err != nil {
					errFunc(err)
					continue
				}
				// Notify subscribers of the change
				notifyFunc(newValue)
				// Store new data for future comparisons
				r.pre.Store(data)
			}
		}
	}()

	return stop, nil
}

// New creates a new environment variable configuration resource
// It sets up a resource that will monitor environment variables with the given prefix
// Parameters:
//   - prefix: The prefix used to filter environment variables (e.g., "APP_")
//   - interval: How often to check for changes (minimum 1 second)
//
// Returns:
//   - *Resource: New environment variable resource instance
//   - error: Any error during initialization
func New(prefix string, interval time.Duration) (*Resource, error) {
	ext := "env"
	// Find appropriate formatter for environment variables
	formatter, ok := format.GetFormatter(ext)
	if !ok {
		return nil, fmt.Errorf("config: not found formatter for %s", ext)
	}

	// Set default interval if not provided or invalid
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// Return new resource instance
	return &Resource{
		prefix:    prefix,
		interval:  interval,
		formatter: formatter,
	}, nil
}
