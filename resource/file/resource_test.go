package file

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/go-leo/gonfig/format/json"
	_ "github.com/go-leo/gonfig/format/yaml"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		expectErr string
	}{
		{"Valid YAML File", "test.yaml", ""},
		{"Valid JSON File", "test.json", ""},
		{"Empty Extension", "test", "config: file extension is empty"},
		{"Unsupported Extension", "test.txt", "config: not found formatter for txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, err := New(tt.filename)
			if tt.expectErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectErr) {
					t.Errorf("expected error %q; got %v", tt.expectErr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if resource == nil {
				t.Errorf("expected non-nil Resource")
			} else {
				if resource.filename != tt.filename {
					t.Errorf("expected filename %q; got %q", tt.filename, resource.filename)
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")
	content := `
key:
  nested_key: value
`
	err := os.WriteFile(testFile, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	resource, err := New(testFile)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	structData, err := resource.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}

	expectedStruct, _ := structpb.NewStruct(map[string]any{
		"key": map[string]any{
			"nested_key": "value",
		},
	})

	if !reflect.DeepEqual(structData, expectedStruct) {
		t.Errorf("expected %v; got %v", expectedStruct, structData)
	}
}

func TestWatch(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, time.Now().Format(time.DateTime)+"_test.yaml")
	content := `
key:
  nested_key: value
`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resource, err := New(testFile)
	if err != nil {
		t.Fatal(err)
	}

	c := make(chan *structpb.Struct)
	notifyFunc := func(newValue *structpb.Struct) {
		if newValue == nil {
			t.Error("expected non-nil struct from watch")
			return
		}
		c <- newValue
	}
	errFunc := func(err error) {
		t.Errorf("Error: %v", err)
	}

	stop, err := resource.Watch(ctx, notifyFunc, errFunc)
	if err != nil {
		t.Fatal(err)
	}
	defer stop(ctx)

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	newValue := <-c
	value := newValue.GetFields()["key"].GetStructValue().GetFields()["nested_key"].GetStringValue()
	if value != "value" {
		t.Errorf("expected value 'value'; got %q", value)
	}

	// 修改文件以触发 Watcher
	newContent := `
key:
  nested_key: updated_value
`
	err = os.WriteFile(testFile, []byte(newContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	newValue = <-c
	value = newValue.GetFields()["key"].GetStructValue().GetFields()["nested_key"].GetStringValue()
	if value != "updated_value" {
		t.Errorf("expected value 'updated_value'; got %q", value)
	}
}
