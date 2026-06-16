package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// compileCommandsEntry is the subset of a compile_commands.json entry needed
// to find the compiler driver. Entries use either "command" (a single shell
// string) or "arguments" (already-split argv) — both are valid per the
// compilation database spec, and real projects use either form.
type compileCommandsEntry struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
}

// DetectCompilerDriver returns the compiler executable used by the first
// entry in compile_commands.json, resolved to an absolute path if found on
// $PATH.
//
// Why this matters: clangd only trusts its own bundled resource-dir's
// standard library include paths unless told to query a specific compiler
// driver for its own (via --query-driver). Cross-compilation toolchains
// (Yocto/poky and similar) ship their own libstdc++ alongside the cross
// compiler, in a location clangd has no way to know about otherwise.
// Without --query-driver pointed at that exact compiler, clangd fails to
// find headers like <functional>, hits its error cap, and silently stops
// parsing partway through the file — every semantic query past that point
// (hover, definition, call hierarchy) then returns empty with no error.
// Confirmed against a real embedded Yocto/poky cross-compilation project
// (aarch64-poky-linux-g++): incomingCalls went from 0 results to correctly
// finding all 3 real call sites once --query-driver was set correctly.
func DetectCompilerDriver(compileCommandsPath string) (string, error) {
	data, err := os.ReadFile(compileCommandsPath)
	if err != nil {
		return "", err
	}
	var entries []compileCommandsEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("compile_commands.json has no entries")
	}

	var compiler string
	if len(entries[0].Arguments) > 0 {
		compiler = entries[0].Arguments[0]
	} else if entries[0].Command != "" {
		fields := strings.Fields(entries[0].Command)
		if len(fields) == 0 {
			return "", fmt.Errorf("empty command in first compile_commands.json entry")
		}
		compiler = fields[0]
	} else {
		return "", fmt.Errorf("first compile_commands.json entry has neither command nor arguments")
	}

	if resolved, err := exec.LookPath(compiler); err == nil {
		return resolved, nil
	}
	return compiler, nil
}
