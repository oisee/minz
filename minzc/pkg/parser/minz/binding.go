package minz

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_minz();
import "C"
import "unsafe"

// Language returns the tree-sitter language for MinZ
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_minz())
}