package c89

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// PromoteStructReturns retains the standalone pass API used by C ABI audits.
// Imported source modules defer the production pass until callers are known.
func PromoteStructReturns(m *hir.Module)           { hir.PromoteStructReturns(m) }
func isShallowScalarStruct(st *mir2.StructTy) bool { return hir.IsShallowScalarStruct(st) }
