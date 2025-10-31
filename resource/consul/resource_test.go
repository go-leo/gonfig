package consul

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/go-leo/gonfig/format"
	"github.com/go-leo/gonfig/format/env"
	"github.com/hashicorp/consul/api"
	_ "golang.org/x/exp/maps"
	_ "golang.org/x/net/http2"
	_ "golang.org/x/sys/unix"
	_ "golang.org/x/text"
	"google.golang.org/protobuf/types/known/structpb"
)

func consulFactory() (*api.Client, error) {
	return api.NewClient(api.DefaultConfig())
}

func TestResource_Load_Consul(t *testing.T) {
	client, err := consulFactory()
	if err != nil {
		t.Errorf("factory() error = %v", err)
		return
	}
	format.RegisterFormatter("env", env.Env{})
	key := "consul.env"

	_, err = client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte("TEST_KEY=test_value"),
	}, nil)
	if err != nil {
		t.Errorf("Put() error = %v", err)
		return
	}

	defer func() {
		_, err = client.KV().Delete(key, nil)
		if err != nil {
			t.Errorf("Delete() error = %v", err)
			return
		}
	}()

	time.Sleep(time.Second)

	r, err := New(client, key)
	if err != nil {
		t.Errorf("New() error = %v", err)
		return
	}
	ctx := context.Background()
	content, err := r.Load(ctx)
	if err != nil {
		t.Errorf("Load() error = %v", err)
		return
	}

	if !reflect.DeepEqual(content.AsMap(), map[string]any{"TEST_KEY": "test_value"}) {
		t.Errorf("Load() data = %v, want data to contain 'TEST_KEY=test_value'", content.AsMap())
	}

	time.Sleep(time.Second)
}

func TestResource_Watch_Consul(t *testing.T) {
	client, err := consulFactory()
	if err != nil {
		t.Errorf("factory() error = %v", err)
		return
	}
	format.RegisterFormatter("env", env.Env{})
	key := "consul.env"

	_, err = client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte("TEST_KEY=" + time.Now().Format(time.DateTime)),
	}, nil)
	if err != nil {
		t.Errorf("PublishConfig() error = %v", err)
		return
	}

	defer func() {
		_, err = client.KV().Delete(key, nil)
		if err != nil {
			t.Errorf("PublishConfig() error = %v", err)
			return
		}
	}()

	r, err := New(client, key)
	if err != nil {
		t.Errorf("New() error = %v", err)
		return
	}

	ctx := context.Background()
	_, _ = r.Load(ctx)

	c := make(chan *structpb.Struct)
	notifyC := func(value *structpb.Struct) {
		c <- value
	}
	errC := func(error) {}
	// Start watching
	stopFunc, err := r.Watch(ctx, notifyC, errC)
	if err != nil {
		t.Errorf("Watch() error = %v", err)
		return
	}
	defer stopFunc(context.Background())

	meta, err := client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte("TEST_KEY=updated"),
	}, nil)
	if err != nil {
		t.Errorf("PublishConfig() error = %v", err)
		return
	}
	_ = meta

	// Wait for the event
	newVal := <-c
	if newVal == nil {
		t.Error("received nil value")
		return
	}
	val := newVal.GetFields()["TEST_KEY"].GetStringValue()
	if val != "updated" {
		t.Errorf("expected value 'updated'; got %q", val)
	}
}
