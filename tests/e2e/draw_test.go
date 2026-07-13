// Package e2e contains end-to-end tests that run skein against a real clangd
// over the fixture project in tests/fixtures/simple_cpp.
package e2e

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/fmbfs/skein/internal/compositor"
	"github.com/fmbfs/skein/internal/lsp"
)

// fixtureDir locates tests/fixtures/simple_cpp relative to this test file,
// so the test works regardless of the caller's working directory.
func fixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "fixtures", "simple_cpp")
}

// requireClangdAndCmake skips the calling test when either tool is missing
// from $PATH — every E2E test in this package needs both.
func requireClangdAndCmake(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("clangd"); err != nil {
		t.Skip("clangd not found on $PATH, skipping E2E test")
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake not found on $PATH, skipping E2E test")
	}
}

// buildFixture runs cmake configure (compile_commands.json export) against
// the fixture project and returns its build directory.
func buildFixture(t *testing.T, dir string) string {
	t.Helper()
	buildDir := filepath.Join(dir, "build")
	cmakeCmd := exec.Command("cmake", "-B", buildDir, "-DCMAKE_EXPORT_COMPILE_COMMANDS=ON")
	cmakeCmd.Dir = dir
	if out, err := cmakeCmd.CombinedOutput(); err != nil {
		t.Fatalf("cmake configure failed: %v\n%s", err, out)
	}
	return buildDir
}

// newFixtureClient spawns a real clangd rooted at buildDir, configured with
// the fixture's detected compiler driver.
func newFixtureClient(t *testing.T, buildDir string) *lsp.Client {
	t.Helper()
	clangdPath, err := exec.LookPath("clangd")
	if err != nil {
		t.Fatalf("clangd disappeared from $PATH: %v", err)
	}

	var extraArgs []string
	if driver, err := lsp.DetectCompilerDriver(filepath.Join(buildDir, "compile_commands.json")); err == nil {
		extraArgs = append(extraArgs, "--query-driver="+driver)
	}

	client, err := lsp.New(clangdPath, buildDir, extraArgs...)
	if err != nil {
		t.Fatalf("lsp.New: %v", err)
	}
	return client
}

// TestDrawProcessFrame locks in skein's known-correct output against the
// fixture project, end to end through real clangd: the LSP client,
// workspace/symbol indexing wait, source-vs-header disambiguation, and the
// outgoingCalls fallback all have to work together correctly for this to
// pass. This is the regression test described in README.md's worked
// example — if this breaks, the documented example is lying.
func TestDrawProcessFrame(t *testing.T) {
	requireClangdAndCmake(t)

	dir := fixtureDir(t)
	buildDir := buildFixture(t, dir)
	client := newFixtureClient(t, buildDir)
	defer client.Close()

	mc := compositor.NewMethodCompositor(client, buildDir)
	rm, err := mc.Build("processFrame", "", 1)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got, want := rm.DefinedAt.Path, filepath.Join("..", "src", "pipeline.cpp"); got != want {
		t.Errorf("DefinedAt.Path = %q, want %q", got, want)
	}
	if rm.DefinedAt.Line != 5 {
		t.Errorf("DefinedAt.Line = %d, want 5", rm.DefinedAt.Line)
	}

	wantCalledIn := []compositor.CalledInGroup{
		{File: filepath.Join("..", "src", "main.cpp"), Lines: []int{8, 9}},
	}
	if !reflect.DeepEqual(rm.CalledIn, wantCalledIn) {
		t.Errorf("CalledIn = %+v, want %+v", rm.CalledIn, wantCalledIn)
	}

	if got, want := rm.Calls, []string{"acquire()"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Calls = %v, want %v", got, want)
	}
}
