// Package resource defines the core interface and types for configuration resources.
package resource

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"
)

// NotifyFunc defines the function type for notification callbacks.
// The value parameter is a pointer to structpb.Struct.
type NotifyFunc func(value *structpb.Struct)

// ErrFunc defines the function type for error handling callbacks.
type ErrFunc func(err error)

// StopFunc defines the function type for stopping monitoring and cleanup.
// It accepts a context for graceful shutdown and returns any cleanup error.
type StopFunc func(context.Context) error

// Resource defines the core interface for configuration resource providers.
// Implementations should handle both synchronous loading and change monitoring
// of configuration data.
type Resource interface {
	// Load retrieves the current configuration state.
	// Args:
	//   - ctx: Context for cancellation and timeouts
	// Returns:
	//   - *structpb.Struct: Current configuration data in protobuf Struct format
	//   - error: Loading error if any

	Load(ctx context.Context) (*structpb.Struct, error)

	// Watch establishes a continuous monitoring of configuration changes.
	// Args:
	//   - ctx: Context for cancellation
	//   - notifyFunc: Function for receiving configuration updates
	//   - errFunc: Function for receiving monitoring errors
	// Returns:
	//   - func(context.Context) error: Cleanup function that stops watching,
	//     takes context for graceful shutdown, returns any cleanup error
	//   - error: Immediate error if watch setup fails
	Watch(ctx context.Context, notifyFunc NotifyFunc, errFunc ErrFunc) (StopFunc, error)
}
