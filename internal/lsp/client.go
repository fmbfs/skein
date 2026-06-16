package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
)

// Client is a synchronous, blocking LSP client over a clangd subprocess.
//
// Design decision (docs/REVIEW.md §14 Q1): one blocking call at a time, no
// goroutine pool. clangd's typical response time (<200ms) makes this
// acceptable for v0.1, and callers running inside a bubbletea tea.Cmd already
// get non-blocking behaviour from bubbletea's own event loop.
type Client struct {
	cmd     *exec.Cmd
	w       io.WriteCloser
	r       *bufio.Reader
	nextID  int64
	rootURI string
}

// New spawns clangdPath as a subprocess rooted at rootDir (the directory
// containing compile_commands.json) and performs the LSP initialize
// handshake. extraArgs are appended to the clangd invocation — typically
// --query-driver=<compiler path> for cross-compiled projects (see
// DetectCompilerDriver).
func New(clangdPath, rootDir string, extraArgs ...string) (*Client, error) {
	args := append([]string{"--log=error"}, extraArgs...)
	cmd := exec.Command(clangdPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: start %s: %w", clangdPath, err)
	}

	c := &Client{
		cmd:     cmd,
		w:       stdin,
		r:       bufio.NewReader(stdout),
		rootURI: pathToURI(rootDir),
	}

	initParams := map[string]interface{}{
		"processId": nil,
		"rootUri":   c.rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"callHierarchy": map[string]interface{}{},
				"typeHierarchy": map[string]interface{}{},
				"hover":         map[string]interface{}{},
			},
			"workspace": map[string]interface{}{
				"symbol": map[string]interface{}{},
			},
		},
	}
	if err := c.Call("initialize", initParams, nil); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("lsp: initialize: %w", err)
	}
	if err := c.Notify("initialized", map[string]interface{}{}); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("lsp: initialized notification: %w", err)
	}
	return c, nil
}

// Call sends a JSON-RPC request and blocks until the matching response
// arrives, decoding its result into out (which may be nil to discard it).
// Notifications received while waiting (e.g. publishDiagnostics) are skipped.
func (c *Client) Call(method string, params interface{}, out interface{}) error {
	id := atomic.AddInt64(&c.nextID, 1)
	if err := c.writeFrame(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		return fmt.Errorf("lsp: write %s: %w", method, err)
	}

	for {
		msg, err := c.readFrame()
		if err != nil {
			return fmt.Errorf("lsp: read response to %s: %w", method, err)
		}
		if msg.ID == nil || *msg.ID != id {
			// Server notification or an unrelated message — discard and keep waiting.
			continue
		}
		if msg.Error != nil {
			return fmt.Errorf("lsp: %s: %s (code %d)", method, msg.Error.Message, msg.Error.Code)
		}
		if out != nil && len(msg.Result) > 0 {
			if err := json.Unmarshal(msg.Result, out); err != nil {
				return fmt.Errorf("lsp: decode %s result: %w", method, err)
			}
		}
		return nil
	}
}

// Notify sends a JSON-RPC notification; no response is expected or read.
func (c *Client) Notify(method string, params interface{}) error {
	return c.writeFrame(jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

// DidOpen tells clangd to open and index a document, given its absolute path
// and contents. Most call/type hierarchy queries require the file to be open.
func (c *Client) DidOpen(path, text string) error {
	return c.Notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": TextDocumentItem{
			URI:        pathToURI(path),
			LanguageID: "cpp",
			Version:    1,
			Text:       text,
		},
	})
}

// WorkspaceSymbol runs workspace/symbol for the given query string.
func (c *Client) WorkspaceSymbol(query string) ([]SymbolInformation, error) {
	var result []SymbolInformation
	err := c.Call("workspace/symbol", WorkspaceSymbolParams{Query: query}, &result)
	return result, err
}

// Definition resolves the symbol at a position to its definition location(s).
// Used to jump from a workspace/symbol hit (which may be a declaration) to
// the actual definition, e.g. header declaration -> .cpp implementation.
func (c *Client) Definition(path string, pos Position) ([]Location, error) {
	var result []Location
	err := c.Call("textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(path)},
		Position:     pos,
	}, &result)
	return result, err
}

// PrepareCallHierarchy resolves the symbol at a position into call-hierarchy items.
func (c *Client) PrepareCallHierarchy(path string, pos Position) ([]CallHierarchyItem, error) {
	var result []CallHierarchyItem
	err := c.Call("textDocument/prepareCallHierarchy", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(path)},
		Position:     pos,
	}, &result)
	return result, err
}

// IncomingCalls returns who calls the given call-hierarchy item.
func (c *Client) IncomingCalls(item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	var result []CallHierarchyIncomingCall
	err := c.Call("callHierarchy/incomingCalls", callHierarchyItemParams{Item: item}, &result)
	return result, err
}

// OutgoingCalls returns what the given call-hierarchy item calls.
func (c *Client) OutgoingCalls(item CallHierarchyItem) ([]CallHierarchyOutgoingCall, error) {
	var result []CallHierarchyOutgoingCall
	err := c.Call("callHierarchy/outgoingCalls", callHierarchyItemParams{Item: item}, &result)
	return result, err
}

// DocumentSymbol returns all symbols defined in a file. clangd 18 returns
// the flat SymbolInformation[] shape (not the hierarchical DocumentSymbol[]
// shape some servers use) — confirmed empirically. Each entry's
// Location.Range covers its full extent (e.g. a class's range spans its
// whole body), which callers use to infer nesting by range containment.
func (c *Client) DocumentSymbol(path string) ([]SymbolInformation, error) {
	var result []SymbolInformation
	err := c.Call("textDocument/documentSymbol", DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(path)},
	}, &result)
	return result, err
}

// PrepareTypeHierarchy resolves the symbol at a position into type-hierarchy items.
func (c *Client) PrepareTypeHierarchy(path string, pos Position) ([]TypeHierarchyItem, error) {
	var result []TypeHierarchyItem
	err := c.Call("textDocument/prepareTypeHierarchy", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(path)},
		Position:     pos,
	}, &result)
	return result, err
}

// Supertypes returns the base classes of the given type-hierarchy item.
func (c *Client) Supertypes(item TypeHierarchyItem) ([]TypeHierarchyItem, error) {
	var result []TypeHierarchyItem
	err := c.Call("typeHierarchy/supertypes", typeHierarchyItemParams{Item: item}, &result)
	return result, err
}

// Subtypes returns the derived classes of the given type-hierarchy item.
func (c *Client) Subtypes(item TypeHierarchyItem) ([]TypeHierarchyItem, error) {
	var result []TypeHierarchyItem
	err := c.Call("typeHierarchy/subtypes", typeHierarchyItemParams{Item: item}, &result)
	return result, err
}

// Close shuts clangd down cleanly: shutdown request, exit notification, then
// waits for the process to exit.
func (c *Client) Close() error {
	_ = c.Call("shutdown", nil, nil)
	_ = c.Notify("exit", nil)
	_ = c.w.Close()
	return c.cmd.Wait()
}

func (c *Client) writeFrame(v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.w.Write([]byte(header)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

func (c *Client) readFrame() (*jsonrpcMessage, error) {
	contentLength := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("lsp: bad Content-Length %q: %w", v, err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("lsp: frame missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, err
	}
	var msg jsonrpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("lsp: decode frame: %w", err)
	}
	return &msg, nil
}

func pathToURI(absPath string) string {
	u := url.URL{Scheme: "file", Path: absPath}
	return u.String()
}

// URIToPath converts a file:// URI back to a filesystem path.
func URIToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("lsp: unsupported URI scheme %q", u.Scheme)
	}
	return u.Path, nil
}
