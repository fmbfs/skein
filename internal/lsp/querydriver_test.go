package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCompileCommands(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "compile_commands.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write compile_commands.json: %v", err)
	}
	return path
}

func TestDetectCompilerDriver_CommandForm(t *testing.T) {
	path := writeCompileCommands(t, `[{"directory":"/build","file":"a.cpp",
		"command":"/opt/sdk/bin/aarch64-poky-linux-g++ -c a.cpp -o a.o"}]`)

	got, err := DetectCompilerDriver(path)
	if err != nil {
		t.Fatalf("DetectCompilerDriver error: %v", err)
	}
	// The fake path doesn't exist on $PATH, so LookPath fails and the raw
	// string is returned unresolved — that's the expected fallback.
	if got != "/opt/sdk/bin/aarch64-poky-linux-g++" {
		t.Errorf("got %q, want the cross-compiler path from the \"command\" string", got)
	}
}

func TestDetectCompilerDriver_ArgumentsForm(t *testing.T) {
	path := writeCompileCommands(t, `[{"directory":"/build","file":"a.cpp",
		"arguments":["/opt/sdk/bin/aarch64-poky-linux-g++","-c","a.cpp"]}]`)

	got, err := DetectCompilerDriver(path)
	if err != nil {
		t.Fatalf("DetectCompilerDriver error: %v", err)
	}
	if got != "/opt/sdk/bin/aarch64-poky-linux-g++" {
		t.Errorf("got %q, want the cross-compiler path from \"arguments\"[0]", got)
	}
}

func TestDetectCompilerDriver_ResolvesPATH(t *testing.T) {
	path := writeCompileCommands(t, `[{"directory":"/build","file":"a.cpp","command":"sh -c true"}]`)

	got, err := DetectCompilerDriver(path)
	if err != nil {
		t.Fatalf("DetectCompilerDriver error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("got %q, want an absolute path resolved via $PATH for a real executable like sh", got)
	}
}

func TestDetectCompilerDriver_NoEntries(t *testing.T) {
	path := writeCompileCommands(t, `[]`)
	if _, err := DetectCompilerDriver(path); err == nil {
		t.Error("expected an error for an empty compile_commands.json, got nil")
	}
}

func TestDetectCompilerDriver_MissingFile(t *testing.T) {
	if _, err := DetectCompilerDriver(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Error("expected an error for a missing compile_commands.json, got nil")
	}
}
