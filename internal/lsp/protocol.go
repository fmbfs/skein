// Package lsp implements a thin LSP client that speaks to a clangd
// subprocess over stdio (JSON-RPC). It is a consumer of clangd, not a
// reimplementation of any analysis.
//
// See docs/SPEC.md section 11 for the list of LSP methods used.
package lsp

import "encoding/json"

// jsonrpcRequest is an outgoing JSON-RPC 2.0 request.
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcNotification is an outgoing JSON-RPC 2.0 notification (no ID, no response expected).
type jsonrpcNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcMessage is the shape used to decode any incoming frame from clangd,
// whether it is a response, a server-to-client request, or a notification.
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Position is a zero-indexed line/character position in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a span between two positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location identifies a range inside a document, addressed by file:// URI.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem is the payload for textDocument/didOpen.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentPositionParams is the common params shape for position-based requests.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// WorkspaceSymbolParams is the params for workspace/symbol.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// SymbolKind mirrors the LSP SymbolKind enum (subset skein cares about).
type SymbolKind int

const (
	SymbolKindNamespace   SymbolKind = 3
	SymbolKindClass       SymbolKind = 5
	SymbolKindMethod      SymbolKind = 6
	SymbolKindProperty    SymbolKind = 7
	SymbolKindField       SymbolKind = 8
	SymbolKindConstructor SymbolKind = 9
	SymbolKindFunction    SymbolKind = 12
	SymbolKindVariable    SymbolKind = 13
	SymbolKindConstant    SymbolKind = 14
	SymbolKindStruct      SymbolKind = 23
)

// SymbolInformation is a workspace/symbol result entry.
type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// CallHierarchyItem identifies a function/method for call hierarchy queries.
//
// Data is an opaque, server-defined blob (clangd uses it as a resolve key).
// It must be round-tripped untouched when passing an item back into
// incomingCalls/outgoingCalls — clangd silently returns an empty result
// (not an error) if it's dropped, rather than rejecting the request.
type CallHierarchyItem struct {
	Name           string          `json:"name"`
	Kind           SymbolKind      `json:"kind"`
	URI            string          `json:"uri"`
	Range          Range           `json:"range"`
	SelectionRange Range           `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
	Detail         string          `json:"detail,omitempty"`
}

// CallHierarchyIncomingCall is one entry from callHierarchy/incomingCalls.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyOutgoingCall is one entry from callHierarchy/outgoingCalls.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

// callHierarchyItemParams wraps a single item, used by incoming/outgoingCalls requests.
type callHierarchyItemParams struct {
	Item CallHierarchyItem `json:"item"`
}

// TypeHierarchyItem identifies a class/struct for type hierarchy queries.
// Same round-trip-the-Data caveat as CallHierarchyItem applies.
type TypeHierarchyItem struct {
	Name           string          `json:"name"`
	Kind           SymbolKind      `json:"kind"`
	URI            string          `json:"uri"`
	Range          Range           `json:"range"`
	SelectionRange Range           `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
}

// typeHierarchyItemParams wraps a single item, used by supertypes/subtypes requests.
type typeHierarchyItemParams struct {
	Item TypeHierarchyItem `json:"item"`
}

// DocumentSymbolParams is the params for textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}
