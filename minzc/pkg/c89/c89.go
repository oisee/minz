// Package c89 implements a C89/C99 frontend for the MinZ compiler.
//
// Architecture:
//
//	C source → modernc.org/cc/v4.Translate() → cc.AST → lower.go → *hir.Module
//
// The heavy lifting (preprocessing, parsing, type checking, implicit conversions)
// is handled by modernc.org/cc. This package only needs to walk the typed AST
// and emit HIR nodes.
//
// See docs/adr/0024-c89-frontend-strategy.md for the full design.
// See docs/adr/0025-struct-return-promotion.md for struct→tuple promotion.
// See docs/adr/0026-c-stdlib-print-variadics.md for @print and variadics.
package c89
