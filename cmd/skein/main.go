// Command skein is a clangd-powered codebase exploration tool.
//
// Two modes:
//
//	skein              open the interactive TUI (default), starting in tangle view
//	skein <symbol>     open the TUI with <symbol> as the first thread
//	skein draw -m foo  fast mode: print a relationship tree to stdout and exit
//
// See docs/SPEC.md for the full design.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fmbfs/skein/internal/compositor"
	"github.com/fmbfs/skein/internal/lsp"
	"github.com/fmbfs/skein/internal/tree"
	"github.com/fmbfs/skein/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// Build-time variables, injected via -ldflags.
var (
	version   = "dev"
	buildDate = "unknown"
	commit    = "none"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("skein %s (commit %s, built %s)\n", version, commit, buildDate)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "draw" {
		if err := runDraw(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "skein:", err)
			os.Exit(1)
		}
		return
	}

	var symbol, pinned string
	if len(os.Args) > 1 {
		symbol = os.Args[1]
	}
	if len(os.Args) > 2 {
		pinned = os.Args[2]
	}
	if err := runTUI(symbol, pinned); err != nil {
		fmt.Fprintln(os.Stderr, "skein:", err)
		os.Exit(1)
	}
}

func runTUI(symbol, pinned string) error {
	rootDir, err := resolveCompileCommandsDir("")
	if err != nil {
		return err
	}

	clangd, err := exec.LookPath("clangd")
	if err != nil {
		return fmt.Errorf("clangd not found on $PATH — install clangd >= 14 or pass --clangd <path>")
	}

	var extraArgs []string
	if driver, err := lsp.DetectCompilerDriver(filepath.Join(rootDir, "compile_commands.json")); err == nil {
		extraArgs = append(extraArgs, "--query-driver="+driver)
	}

	client, err := lsp.New(clangd, rootDir, extraArgs...)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	m := tui.New(client, rootDir, symbol, pinned)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func runDraw(args []string) error {
	fs := flag.NewFlagSet("draw", flag.ContinueOnError)
	method := fs.String("m", "", "method/function name to draw")
	class := fs.String("c", "", "class name to draw; with -m, scopes the method to this class")
	file := fs.String("f", "", "file to draw")
	symbol := fs.String("s", "", "symbol to draw (generic: resolves to method or class automatically)")
	ply := fs.Int("ply", 1, "traversal depth (max 3)")
	strands := fs.Int("strands", 50, "max visible nodes before truncation")
	dbPath := fs.String("db", "", "path to compile_commands.json (default: search upward from cwd)")
	clangdPath := fs.String("clangd", "", "clangd binary (default: $PATH)")
	asJSON := fs.Bool("json", false, "JSON output")
	noColour := fs.Bool("no-color", false, "disable ANSI colour")
	absolute := fs.Bool("absolute", false, "print absolute file paths instead of root-relative")
	if err := fs.Parse(args); err != nil {
		return err
	}

	set := 0
	for _, v := range []string{*method, *class, *file, *symbol} {
		if v != "" {
			set++
		}
	}
	// -c may accompany -m to scope a method query to one class (e.g.
	// `-m Get -c DB`); it cannot accompany -f/-s, and -c alone still means
	// "draw this class". So the only valid two-flag combination is m+c.
	validCombo := set == 1 || (*method != "" && *class != "" && *file == "" && *symbol == "")
	if !validCombo {
		return fmt.Errorf("usage: skein draw (-m <method> [-c <class>] | -c <class> | -f <file> | -s <symbol>)")
	}

	rootDir, err := resolveCompileCommandsDir(*dbPath)
	if err != nil {
		return err
	}

	clangd := *clangdPath
	if clangd == "" {
		clangd, err = exec.LookPath("clangd")
		if err != nil {
			return fmt.Errorf("clangd not found on $PATH — install clangd >= 14 or pass --clangd <path>")
		}
	}

	var extraArgs []string
	if driver, err := lsp.DetectCompilerDriver(filepath.Join(rootDir, "compile_commands.json")); err == nil {
		extraArgs = append(extraArgs, "--query-driver="+driver)
	}

	client, err := lsp.New(clangd, rootDir, extraArgs...)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	colourOpt := tree.WithColour(tree.AutoColour(os.Stdout, *noColour))

	switch {
	case *method != "":
		return drawMethod(client, rootDir, *method, *class, *ply, *strands, *asJSON, *absolute, colourOpt)

	case *class != "":
		return drawClass(client, rootDir, *class, *strands, *asJSON, *absolute, colourOpt)

	case *file != "":
		return drawFile(client, rootDir, *file, *strands, *asJSON, *absolute, colourOpt)

	case *symbol != "":
		return drawSymbol(client, rootDir, *symbol, *ply, *strands, *asJSON, *absolute, colourOpt)
	}
	return nil
}

// drawMethod builds and prints a method/function's RelationMap, applying
// strand-limit truncation and printing a stderr warning when the map had to
// be cut down.
func drawMethod(client *lsp.Client, rootDir, method, class string, ply, strands int, asJSON, absolute bool, colourOpt tree.Option) error {
	mc := compositor.NewMethodCompositor(client, rootDir)
	rm, err := mc.Build(method, class, ply)
	if err != nil {
		return err
	}
	if len(rm.Ambiguous) > 0 {
		fmt.Fprintf(os.Stderr,
			"skein: %q also found in %s — showing %s. Re-run with -c <class> to pick one.\n",
			method, joinQuoted(truncateList(rm.Ambiguous, 5)), rm.Container)
	}
	warnCalledIn := rm.TruncateCalledIn(strands)
	warnCalls := rm.TruncateCalls(strands)
	printTruncationWarning(warnCalledIn)
	printTruncationWarning(warnCalls)
	if absolute {
		absolutizeRelationMap(rm, rootDir)
	}
	if asJSON {
		return tree.PrintJSON(os.Stdout, rm)
	}
	tree.Print(os.Stdout, rm, colourOpt)
	return nil
}

func drawClass(client *lsp.Client, rootDir, class string, strands int, asJSON, absolute bool, colourOpt tree.Option) error {
	cc := compositor.NewClassCompositor(client, rootDir)
	cm, err := cc.Build(class)
	if err != nil {
		return err
	}
	printTruncationWarning(cm.TruncateMembers(strands))
	if absolute {
		absolutizeClassMap(cm, rootDir)
	}
	if asJSON {
		return tree.PrintClassJSON(os.Stdout, cm)
	}
	tree.PrintClass(os.Stdout, cm, colourOpt)
	return nil
}

func drawFile(client *lsp.Client, rootDir, file string, strands int, asJSON, absolute bool, colourOpt tree.Option) error {
	path := file
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = filepath.Join(cwd, path)
	}
	fc := compositor.NewFileCompositor(client, rootDir)
	fm, err := fc.Build(path)
	if err != nil {
		return err
	}
	printTruncationWarning(fm.TruncateSymbols(strands))
	if absolute {
		absolutizeFileMap(fm, rootDir)
	}
	if asJSON {
		return tree.PrintFileJSON(os.Stdout, fm)
	}
	tree.PrintFile(os.Stdout, fm, colourOpt)
	return nil
}

// drawSymbol resolves a generic symbol name via workspace/symbol and
// dispatches to the method or class draw path depending on its kind —
// mirrors internal/tui's resolveGeneric so `skein draw -s <symbol>` and the
// TUI's initial-symbol resolution behave consistently.
func drawSymbol(client *lsp.Client, rootDir, symbol string, ply, strands int, asJSON, absolute bool, colourOpt tree.Option) error {
	matches, err := compositor.ResolveSymbol(client, rootDir, symbol)
	if err != nil {
		return fmt.Errorf("workspace/symbol %q: %w", symbol, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no symbol found matching %q", symbol)
	}
	best := matches[0]
	if isClassKind(best.Kind) {
		return drawClass(client, rootDir, best.Name, strands, asJSON, absolute, colourOpt)
	}
	return drawMethod(client, rootDir, best.Name, "", ply, strands, asJSON, absolute, colourOpt)
}

// isClassKind mirrors internal/tui/search.go's followKindForSymbol
// class/struct branch — duplicated rather than imported to keep cmd/skein
// decoupled from the TUI package.
func isClassKind(k lsp.SymbolKind) bool {
	return k == lsp.SymbolKindClass || k == lsp.SymbolKindStruct
}

func printTruncationWarning(w compositor.TruncationWarning) {
	if w == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "skein: %s\n", w)
}

func absolutizeRelationMap(rm *compositor.RelationMap, rootDir string) {
	rm.DefinedAt.Path = toAbsolute(rm.DefinedAt.Path, rootDir)
	for i := range rm.CalledIn {
		rm.CalledIn[i].File = toAbsolute(rm.CalledIn[i].File, rootDir)
	}
}

func absolutizeClassMap(cm *compositor.ClassMap, rootDir string) {
	cm.DefinedAt.Path = toAbsolute(cm.DefinedAt.Path, rootDir)
}

func absolutizeFileMap(fm *compositor.FileMap, rootDir string) {
	fm.ThreadName = toAbsolute(fm.ThreadName, rootDir)
}

// toAbsolute resolves a root-relative path (as produced by the compositor,
// typically starting with "../") to an absolute path anchored at rootDir.
// Already-absolute paths pass through unchanged.
func toAbsolute(path, rootDir string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(rootDir, path))
}

// joinQuoted renders names as a comma-separated, double-quoted list for
// stderr warnings, e.g. []string{"A","B"} -> `"A", "B"`.
func joinQuoted(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}
	return strings.Join(quoted, ", ")
}

// truncateList caps names at n entries, appending an "and N more" marker —
// real-world headers (gtest/gmock template instantiations, in particular)
// can produce dozens of ambiguous containers, and dumping all of them makes
// the warning unreadable instead of actionable.
func truncateList(names []string, n int) []string {
	if len(names) <= n {
		return names
	}
	out := make([]string, n, n+1)
	copy(out, names[:n])
	return append(out, fmt.Sprintf("and %d more", len(names)-n))
}

// resolveCompileCommandsDir returns the directory containing
// compile_commands.json: explicitDB if set, otherwise the nearest one found
// walking upward from the current working directory (mirrors clangd's own
// lookup). Fails fast with an actionable error if none is found — see
// docs/REVIEW.md §14 Q4.
func resolveCompileCommandsDir(explicitDB string) (string, error) {
	if explicitDB != "" {
		return filepath.Dir(explicitDB), nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "compile_commands.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf(
		"no compile_commands.json found above the current directory.\n" +
			"Generate one and try again:\n" +
			"  CMake:     cmake -B build -DCMAKE_EXPORT_COMPILE_COMMANDS=ON\n" +
			"  Non-CMake: bear -- make")
}
