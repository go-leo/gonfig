// Package file provides file-based implementation of the configuration resource interface
package file

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/go-leo/gonfig/format"
	"github.com/go-leo/gonfig/resource"
	"google.golang.org/protobuf/types/known/structpb"
)

var _ resource.Resource = (*Resource)(nil)

// Resource represents a configuration resource loaded from a file
type Resource struct {
	// filename is the path to the configuration file
	filename string
	// formatter is used for parsing file content into structured data
	formatter format.Formatter
	// pre is atomic storage for previous configuration data to detect changes
	pre atomic.Value
}

// Load reads and parses the configuration file
// It loads the file content and uses the formatter to convert it to structpb.Struct format
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
	r.pre.Store(data)
	return r.formatter.Parse(data)
}

// load is an internal helper function to read raw file content
// Parameters:
//   - ctx: Context for cancellation and timeouts
//
// Returns:
//   - []byte: Raw file content
//   - error: Any error that occurred while reading the file
func (r *Resource) load(ctx context.Context) ([]byte, error) {
	return os.ReadFile(r.filename)
}

// Watch monitors the file for changes and notifies subscribers when updates occur
// It uses fsnotify to watch for filesystem events and filters for relevant changes
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

	// Initialize filesystem watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch the directory containing the file
	if err := fsWatcher.Add(filepath.Dir(r.filename)); err != nil {
		return nil, err
	}

	// Create stop channel and stop function
	stopC := make(chan struct{})
	stop := func(ctx context.Context) error {
		close(stopC)
		return nil
	}

	// Set default error handler if none provided
	if errFunc == nil {
		errFunc = func(err error) {
			slog.Error("gonfig: failed to watch file", slog.String("error", err.Error()))
		}
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Start watching in a separate goroutine
	go func() {
		// Ensure watcher is closed when goroutine exits
		defer func() {
			if err := fsWatcher.Close(); err != nil {
				errFunc(err)
			}
		}()

		// Event loop
		for {
			select {
			case <-ctx.Done():
				// Context cancelled, exit goroutine
				return

			case <-stopC:
				// Stop signal received, exit goroutine
				return

			case err, ok := <-fsWatcher.Errors:
				// Error from filesystem watcher
				if !ok {
					return
				}
				errFunc(err)

			case event, ok := <-fsWatcher.Events:
				// File system event received
				if !ok {
					return
				}
				// Only process events for our specific file
				if filepath.Clean(event.Name) != filepath.Clean(r.filename) {
					continue
				}
				// Only react to write/create events
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
					continue
				}
				// Handle file change
				data, err := r.load(ctx)
				if err != nil {
					errFunc(err)
					continue
				}
				// Compare with previous data to avoid unnecessary notifications
				preData := r.pre.Load()
				if preData != nil && bytes.Equal(preData.([]byte), data) {
					continue // Skip if content hasn't changed
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

// New creates a new file-based configuration resource
// It validates the file extension and finds an appropriate formatter
// Parameters:
//   - filename: Path to the configuration file
//
// Returns:
//   - *Resource: New file resource instance
//   - error: Any error during initialization
func New(filename string) (*Resource, error) {
	// Extract file extension
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	if ext == "" {
		return nil, fmt.Errorf("config: file extension is empty")
	}

	// Find appropriate formatter for the file extension
	formatter, ok := format.GetFormatter(ext)
	if !ok {
		return nil, fmt.Errorf("config: not found formatter for %s", ext)
	}

	// Return new resource instance
	return &Resource{
		filename:  filename,
		formatter: formatter,
	}, nil
}
