package tree

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
)

// failingWriter always returns an error on Write, to exercise the error
// path of encodeJSON / PrintJSON / PrintClassJSON / PrintFileJSON.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("boom")
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{ThreadName: "foo", Kind: "function", Ply: 1}
	if err := PrintJSON(&buf, rm); err != nil {
		t.Fatalf("PrintJSON() error: %v", err)
	}

	var decoded compositor.RelationMap
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.ThreadName != "foo" || decoded.Kind != "function" || decoded.Ply != 1 {
		t.Errorf("decoded = %+v, want ThreadName=foo Kind=function Ply=1", decoded)
	}
}

func TestPrintClassJSON(t *testing.T) {
	var buf bytes.Buffer
	cm := &compositor.ClassMap{ThreadName: "Foo", Kind: "class"}
	if err := PrintClassJSON(&buf, cm); err != nil {
		t.Fatalf("PrintClassJSON() error: %v", err)
	}
	var decoded compositor.ClassMap
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.ThreadName != "Foo" {
		t.Errorf("decoded.ThreadName = %q, want %q", decoded.ThreadName, "Foo")
	}
}

func TestPrintFileJSON(t *testing.T) {
	var buf bytes.Buffer
	fm := &compositor.FileMap{ThreadName: "a.cpp", Kind: "file"}
	if err := PrintFileJSON(&buf, fm); err != nil {
		t.Fatalf("PrintFileJSON() error: %v", err)
	}
	var decoded compositor.FileMap
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.ThreadName != "a.cpp" {
		t.Errorf("decoded.ThreadName = %q, want %q", decoded.ThreadName, "a.cpp")
	}
}

func TestPrintJSON_WriteError(t *testing.T) {
	rm := &compositor.RelationMap{ThreadName: "foo"}
	if err := PrintJSON(failingWriter{}, rm); err == nil {
		t.Error("expected an error when the writer fails, got nil")
	}
}

func TestPrintJSON_Indented(t *testing.T) {
	var buf bytes.Buffer
	rm := &compositor.RelationMap{ThreadName: "foo", Kind: "function"}
	if err := PrintJSON(&buf, rm); err != nil {
		t.Fatalf("PrintJSON() error: %v", err)
	}
	// enc.SetIndent("", "  ") means nested output uses a real newline + 2 spaces.
	if !bytes.Contains(buf.Bytes(), []byte("\n  \"")) {
		t.Errorf("expected indented JSON output, got: %s", buf.String())
	}
}
