package gonfig

import (
	"context"
	"sync"

	"github.com/go-leo/gonfig/resource"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type Watcher[Config proto.Message] interface {
	ConfigC() <-chan Config
	ErrC() <-chan error
	Stop(ctx context.Context) error
}

type watcher[Config proto.Message] struct {
	notifyC  chan *structpb.Struct
	errC     chan error
	stop     func(context.Context) error
	configC  chan Config
	stopC    chan struct{}
	stopOnce sync.Once
}

func (w *watcher[Config]) ConfigC() <-chan Config {
	return w.configC
}

func (w *watcher[Config]) ErrC() <-chan error {
	return w.errC
}

func (w *watcher[Config]) Stop(ctx context.Context) error {
	var err error
	w.stopOnce.Do(func() {
		defer close(w.errC)
		defer close(w.configC)
		defer close(w.notifyC)
		defer close(w.stopC)
		err = w.stop(ctx)
	})
	return err
}

func (w *watcher[Config]) Watch(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				noBlockSend[error](w.errC, ctx.Err())
				return
			case <-w.stopC:
				return
			case value, ok := <-w.notifyC:
				if !ok {
					return
				}
				config, err := convert[Config](value)
				if err != nil {
					noBlockSend[error](w.errC, err)
					continue
				}
				w.configC <- config
			}
		}
	}()
}

func noBlockSend[T any](c chan T, v T) {
	select {
	case c <- v:
	default:
	}
}

func Watch[Config proto.Message](ctx context.Context, resource resource.Resource) (Watcher[Config], error) {
	notifyC := make(chan *structpb.Struct, 1)
	errC := make(chan error, 1)
	stop, err := resource.Watch(ctx, notifyC, errC)
	if err != nil {
		return nil, err
	}
	w := &watcher[Config]{
		notifyC: notifyC,
		errC:    errC,
		stop:    stop,
		configC: make(chan Config, 1),
		stopC:   make(chan struct{}),
	}
	w.Watch(ctx)
	return w, nil
}

func Load[Config proto.Message](ctx context.Context, resource resource.Resource) (Config, error) {
	var config Config
	value, err := resource.Load(ctx)
	if err != nil {
		return config, err
	}
	return convert[Config](value)
}

func convert[Config proto.Message](value *structpb.Struct) (Config, error) {
	var config Config
	data, err := value.MarshalJSON()
	if err != nil {
		return config, err
	}
	config = config.ProtoReflect().Type().New().Interface().(Config)
	if err := protojson.Unmarshal(data, config); err != nil {
		return config, err
	}
	return config, nil
}
