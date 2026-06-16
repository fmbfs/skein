package compositor

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// compileCommandsEntry is the subset of a compile_commands.json entry skein needs.
type compileCommandsEntry struct {
	Directory string `json:"directory"`
	File      string `json:"file"`
}

// nudgeIndexer opens one translation unit from compile_commands.json so
// clangd's background index actually starts. Without at least one
// textDocument/didOpen, clangd never populates its workspace symbol index —
// confirmed empirically: workspace/symbol returns nothing indefinitely
// without an open document, but populates within ~1-2s once one exists.
// This is a deliberate, minimal nudge — not a substitute for proper
// workspace-wide indexing progress tracking (a v0.2+ improvement).
func (b *base) nudgeIndexer() error {
	data, err := os.ReadFile(filepath.Join(b.RootDir, "compile_commands.json"))
	if err != nil {
		return err
	}
	var entries []compileCommandsEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	path := entries[0].File
	if !filepath.IsAbs(path) {
		path = filepath.Join(entries[0].Directory, path)
	}
	return b.openFile(path)
}
