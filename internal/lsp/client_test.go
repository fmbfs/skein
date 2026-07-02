package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// jsonrpcResponse is the response shape a fake clangd server sends back.
type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

// pipeClients wires two Clients to each other via in-memory pipes: "client"
// is the Client under test, "server" plays clangd's role, reusing the exact
// same wire-format code (readFrame/writeFrame) as production.
func pipeClients() (client, server *Client) {
	cToS_r, cToS_w := io.Pipe()
	sToC_r, sToC_w := io.Pipe()

	client = &Client{w: cToS_w, r: bufio.NewReader(sToC_r)}
	server = &Client{w: sToC_w, r: bufio.NewReader(cToS_r)}
	return client, server
}

// respondOnce reads a single request frame on the server side and replies
// with result (or err, if non-nil, as a JSON-RPC error).
func respondOnce(t *testing.T, server *Client, result interface{}, rpcErr *rpcError) {
	t.Helper()
	go func() {
		msg, err := server.readFrame()
		if err != nil {
			return
		}
		var id int64
		if msg.ID != nil {
			id = *msg.ID
		}
		_ = server.writeFrame(jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
	}()
}

// drainOnce reads and discards a single frame on the server side (used for
// notifications, which expect no response).
func drainOnce(server *Client) {
	go func() {
		_, _ = server.readFrame()
	}()
}

func TestClient_Call_Success(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []SymbolInformation{{Name: "foo", Kind: SymbolKindFunction}}, nil)

	result, err := client.WorkspaceSymbol("foo")
	if err != nil {
		t.Fatalf("WorkspaceSymbol() error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "foo" {
		t.Errorf("WorkspaceSymbol() = %+v, want one symbol named foo", result)
	}
}

func TestClient_Call_RPCError(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, nil, &rpcError{Code: -32601, Message: "method not found"})

	_, err := client.WorkspaceSymbol("foo")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "method not found") {
		t.Errorf("error = %v, want it to mention the RPC error message", err)
	}
}

func TestClient_Call_SkipsUnrelatedMessages(t *testing.T) {
	client, server := pipeClients()
	go func() {
		// Read the client's request first (this is what unblocks its write).
		msg, err := server.readFrame()
		if err != nil {
			return
		}
		var id int64
		if msg.ID != nil {
			id = *msg.ID
		}
		// Write an unrelated/mismatched-id frame first — Call must skip it
		// and keep waiting — then the real, matching response.
		_ = server.writeFrame(jsonrpcResponse{JSONRPC: "2.0", ID: id + 1000, Result: "irrelevant"})
		_ = server.writeFrame(jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: []SymbolInformation{{Name: "bar"}}})
	}()

	result, err := client.WorkspaceSymbol("bar")
	if err != nil {
		t.Fatalf("WorkspaceSymbol() error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "bar" {
		t.Errorf("WorkspaceSymbol() = %+v, want one symbol named bar", result)
	}
}

func TestClient_Notify_DidOpen(t *testing.T) {
	client, server := pipeClients()
	drainOnce(server)

	if err := client.DidOpen("/a.cpp", "int main(){}"); err != nil {
		t.Fatalf("DidOpen() error: %v", err)
	}
}

func TestClient_Definition(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []Location{{URI: "file:///a.cpp", Range: Range{}}}, nil)

	got, err := client.Definition("/a.cpp", Position{Line: 1, Character: 2})
	if err != nil {
		t.Fatalf("Definition() error: %v", err)
	}
	if len(got) != 1 || got[0].URI != "file:///a.cpp" {
		t.Errorf("Definition() = %+v, want one location with URI file:///a.cpp", got)
	}
}

func TestClient_PrepareCallHierarchy(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []CallHierarchyItem{{Name: "foo", Kind: SymbolKindFunction}}, nil)

	got, err := client.PrepareCallHierarchy("/a.cpp", Position{})
	if err != nil {
		t.Fatalf("PrepareCallHierarchy() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "foo" {
		t.Errorf("PrepareCallHierarchy() = %+v, want one item named foo", got)
	}
}

func TestClient_IncomingCalls(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []CallHierarchyIncomingCall{
		{From: CallHierarchyItem{Name: "caller"}},
	}, nil)

	got, err := client.IncomingCalls(CallHierarchyItem{Name: "foo"})
	if err != nil {
		t.Fatalf("IncomingCalls() error: %v", err)
	}
	if len(got) != 1 || got[0].From.Name != "caller" {
		t.Errorf("IncomingCalls() = %+v, want one call from 'caller'", got)
	}
}

func TestClient_OutgoingCalls(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []CallHierarchyOutgoingCall{
		{To: CallHierarchyItem{Name: "callee"}},
	}, nil)

	got, err := client.OutgoingCalls(CallHierarchyItem{Name: "foo"})
	if err != nil {
		t.Fatalf("OutgoingCalls() error: %v", err)
	}
	if len(got) != 1 || got[0].To.Name != "callee" {
		t.Errorf("OutgoingCalls() = %+v, want one call to 'callee'", got)
	}
}

func TestClient_DocumentSymbol(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []SymbolInformation{{Name: "foo", Kind: SymbolKindClass}}, nil)

	got, err := client.DocumentSymbol("/a.cpp")
	if err != nil {
		t.Fatalf("DocumentSymbol() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "foo" {
		t.Errorf("DocumentSymbol() = %+v, want one symbol named foo", got)
	}
}

func TestClient_PrepareTypeHierarchy(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []TypeHierarchyItem{{Name: "Foo", Kind: SymbolKindClass}}, nil)

	got, err := client.PrepareTypeHierarchy("/a.cpp", Position{})
	if err != nil {
		t.Fatalf("PrepareTypeHierarchy() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Foo" {
		t.Errorf("PrepareTypeHierarchy() = %+v, want one item named Foo", got)
	}
}

func TestClient_Supertypes(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []TypeHierarchyItem{{Name: "Base"}}, nil)

	got, err := client.Supertypes(TypeHierarchyItem{Name: "Foo"})
	if err != nil {
		t.Fatalf("Supertypes() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Base" {
		t.Errorf("Supertypes() = %+v, want one item named Base", got)
	}
}

func TestClient_Subtypes(t *testing.T) {
	client, server := pipeClients()
	respondOnce(t, server, []TypeHierarchyItem{{Name: "Derived"}}, nil)

	got, err := client.Subtypes(TypeHierarchyItem{Name: "Foo"})
	if err != nil {
		t.Fatalf("Subtypes() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Derived" {
		t.Errorf("Subtypes() = %+v, want one item named Derived", got)
	}
}

func TestClient_Call_WriteError(t *testing.T) {
	client, _ := pipeClients()
	// Close the client's write end so writeFrame fails immediately.
	_ = client.w.Close()

	_, err := client.WorkspaceSymbol("foo")
	if err == nil {
		t.Fatal("expected an error when the write side is closed, got nil")
	}
}

func TestClient_Call_ReadError(t *testing.T) {
	client, server := pipeClients()
	go func() {
		_, _ = server.readFrame()
		_ = server.w.Close() // close before responding -> client read fails
	}()

	_, err := client.WorkspaceSymbol("foo")
	if err == nil {
		t.Fatal("expected a read error, got nil")
	}
}

func TestWriteFrame(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{w: nopWriteCloser{&buf}}
	if err := c.writeFrame(jsonrpcRequest{JSONRPC: "2.0", ID: 1, Method: "foo"}); err != nil {
		t.Fatalf("writeFrame() error: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "Content-Length: ") {
		t.Fatalf("expected frame to start with Content-Length header, got: %q", out)
	}
	if !strings.Contains(out, "\r\n\r\n") {
		t.Fatalf("expected header/body separator, got: %q", out)
	}
	body := out[strings.Index(out, "\r\n\r\n")+4:]
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("body is not valid JSON: %v, body=%q", err, body)
	}
	if decoded["method"] != "foo" {
		t.Errorf("decoded method = %v, want foo", decoded["method"])
	}
}

func TestReadFrame_MissingContentLength(t *testing.T) {
	c := &Client{r: bufio.NewReader(strings.NewReader("\r\n"))}
	if _, err := c.readFrame(); err == nil {
		t.Error("expected an error for a frame with no Content-Length header, got nil")
	}
}

func TestReadFrame_BadContentLength(t *testing.T) {
	c := &Client{r: bufio.NewReader(strings.NewReader("Content-Length: notanumber\r\n\r\n"))}
	if _, err := c.readFrame(); err == nil {
		t.Error("expected an error for a non-numeric Content-Length, got nil")
	}
}

func TestReadFrame_TruncatedBody(t *testing.T) {
	c := &Client{r: bufio.NewReader(strings.NewReader("Content-Length: 100\r\n\r\n{}"))}
	if _, err := c.readFrame(); err == nil {
		t.Error("expected an error when the body is shorter than Content-Length, got nil")
	}
}

func TestReadFrame_HeaderReadEOF(t *testing.T) {
	c := &Client{r: bufio.NewReader(strings.NewReader(""))}
	if _, err := c.readFrame(); err == nil {
		t.Error("expected an EOF error reading headers, got nil")
	}
}

func TestReadFrame_InvalidJSON(t *testing.T) {
	body := "not json"
	frame := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	c := &Client{r: bufio.NewReader(strings.NewReader(frame))}
	if _, err := c.readFrame(); err == nil {
		t.Error("expected an error for invalid JSON body, got nil")
	}
}

func TestReadFrame_ValidRoundTrip(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":5,"result":{"ok":true}}`
	frame := "Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	c := &Client{r: bufio.NewReader(strings.NewReader(frame))}
	msg, err := c.readFrame()
	if err != nil {
		t.Fatalf("readFrame() error: %v", err)
	}
	if msg.ID == nil || *msg.ID != 5 {
		t.Errorf("msg.ID = %v, want 5", msg.ID)
	}
}

func TestPathToURI(t *testing.T) {
	got := pathToURI("/foo/bar.cpp")
	want := "file:///foo/bar.cpp"
	if got != want {
		t.Errorf("pathToURI() = %q, want %q", got, want)
	}
}

func TestURIToPath(t *testing.T) {
	got, err := URIToPath("file:///foo/bar.cpp")
	if err != nil {
		t.Fatalf("URIToPath() error: %v", err)
	}
	if got != "/foo/bar.cpp" {
		t.Errorf("URIToPath() = %q, want %q", got, "/foo/bar.cpp")
	}
}

func TestURIToPath_UnsupportedScheme(t *testing.T) {
	if _, err := URIToPath("http://example.com/foo"); err == nil {
		t.Error("expected an error for a non-file URI scheme, got nil")
	}
}

func TestURIToPath_InvalidURI(t *testing.T) {
	if _, err := URIToPath("://bad uri"); err == nil {
		t.Error("expected an error for an unparsable URI, got nil")
	}
}

func TestPathToURI_URIToPath_RoundTrip(t *testing.T) {
	original := "/some/deep/path/file.h"
	uri := pathToURI(original)
	got, err := URIToPath(uri)
	if err != nil {
		t.Fatalf("URIToPath() error: %v", err)
	}
	if got != original {
		t.Errorf("round trip = %q, want %q", got, original)
	}
}

// TestClient_Close exercises the real subprocess shutdown path using `cat`
// as a stand-in for clangd: it echoes whatever is written to its stdin back
// to stdout (so the shutdown request round-trips as its own "response",
// which Close ignores the result of anyway), then exits cleanly on EOF once
// stdin is closed.
func TestClient_Close(t *testing.T) {
	cmd := exec.Command("cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	c := &Client{cmd: cmd, w: stdin, r: bufio.NewReader(stdout)}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

// nopWriteCloser adapts an io.Writer to io.WriteCloser for tests that don't
// care about the close behaviour.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
