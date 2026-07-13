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

// KindIsCallable reports whether a display kind (as produced by kindName or
// stored on a Member) represents something invoked like a function — i.e.
// its rendered label should carry a trailing "()". "constructor" is included
// for forward-compatibility even though kindName currently folds
// SymbolKindConstructor into "method" before it reaches any Member; if that
// mapping ever changes, callers relying on this predicate need no update.
func KindIsCallable(kind string) bool {
	switch kind {
	case "method", "function", "constructor":
		return true
	default:
		return false
	}
}
