package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"strings"
)

func CanonicalPascalDecimalRuntime(name string) *Func {
	var code string
	var params []Param
	var ins []AsmOperand
	switch name {
	case "WriteI16":
		code = mir2.PascalSignedDecimalPrefix + " / " + strings.TrimPrefix(mir2.PascalUnsignedDecimalASM, "PUSH IX / ")
		params = []Param{{Name: "val", Ty: mir2.TyI16}}
		ins = []AsmOperand{{Name: "val"}}
	case "WriteU16":
		code = mir2.PascalUnsignedDecimalASM
		params = []Param{{Name: "val", Ty: mir2.TyU16}}
		ins = []AsmOperand{{Name: "val"}}
	case "_wr16_dig":
		code = mir2.PascalDecimalDigitASM
	default:
		return nil
	}
	return &Func{Name: name, Params: params, RetTy: mir2.TyVoid, Body: &Block{Body: []Stmt{&AsmStmt{Target: "z80", Code: code, ClobberAll: true, Ins: ins}, &ReturnStmt{}}}}
}
