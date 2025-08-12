package minz_binding

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../src -I${SRCDIR}/../../../../src/tree_sitter -std=c11
#include <tree_sitter/parser.h>

// Forward declaration
TSLanguage *tree_sitter_minz();
*/
import "C"
import "unsafe"

// Language returns the Tree-sitter language for MinZ
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_minz())
}