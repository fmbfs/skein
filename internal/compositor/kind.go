package compositor

import "github.com/fmbfs/skein/internal/lsp"

// kindName maps an LSP SymbolKind to skein's display label.
func kindName(k lsp.SymbolKind) string {
	switch k {
	case lsp.SymbolKindNamespace:
		return "namespace"
	case lsp.SymbolKindClass:
		return "class"
	case lsp.SymbolKindMethod:
		return "method"
	case lsp.SymbolKindProperty, lsp.SymbolKindField:
		return "field"
	case lsp.SymbolKindConstructor:
		return "method"
	case lsp.SymbolKindFunction:
		return "function"
	case lsp.SymbolKindVariable, lsp.SymbolKindConstant:
		return "field"
	case lsp.SymbolKindStruct:
		return "struct"
	default:
		return "symbol"
	}
}
