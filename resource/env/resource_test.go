package env

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-leo/gonfig/format"
	"github.com/go-leo/gonfig/format/env"
	"google.golang.org/protobuf/types/known/structpb"
)

func init() {
	format.RegisterFormatter("env", env.Env{})
}

func TestLoad(t *testing.T) {
	// 创建测试资源
	resource, err := New("TEST_", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// 设置测试环境变量
	testCases := []struct {
		key   string
		value string
	}{
		{"TEST_A", "1"},
		{"TEST_B", "2"},
		{"OTHER", "3"}, // 不应包含
	}

	// 设置环境变量并记录预期输出
	expected := map[string]any{}
	for _, tc := range testCases {
		os.Setenv(tc.key, tc.value)
		defer os.Unsetenv(tc.key)
		if strings.HasPrefix(tc.key, "TEST_") {
			expected[tc.key] = tc.value
		}
	}

	// 执行 Load
	data, err := resource.Load(context.Background())
	if err != nil {
		t.Errorf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(data.AsMap(), expected) {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, data.AsMap())
	}
}

func TestWatch(t *testing.T) {
	// 创建测试资源
	resource, err := New("TEST_", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// 准备测试通道
	c := make(chan *structpb.Struct)
	notifyC := func(value *structpb.Struct) {
		c <- value
	}
	errC := func(error) {
		t.Error(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()

	// 启动 Watcher
	stop, err := resource.Watch(ctx, notifyC, errC)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stop(ctx); err != nil {
			t.Error(err)
		}
	}()

	// 修改环境变量
	os.Setenv("TEST_KEY", "updated")
	defer os.Unsetenv("TEST_KEY")

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
