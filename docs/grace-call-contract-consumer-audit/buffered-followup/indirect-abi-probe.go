package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

func main() {
	target := &mir2.Func{Name: "fallible", Attrs: mir2.FuncAttrs{FallibleCY: true}, Contract: mir2.Contract{Params: []mir2.Param{{Ty: mir2.TyU8}}, Returns: []mir2.Return{{Ty: mir2.TyU8}}}}
	i := &mir2.Inst{Op: mir2.OpCallIndirect, Ty: mir2.TyU8, IndirectParamTys: []mir2.Ty{mir2.TyU8}, IndirectReturnTys: []mir2.Ty{mir2.TyU8}}
	fmt.Printf("typed ordinary -> fallible target validation: %v\n", mir2.ValidateIndirectTargetABI(i, target))
	target.Attrs.FallibleCY = false
	target.Contract.Params[0].Ty = mir2.TyU16
	fmt.Printf("different-width target validation: %v\n", mir2.ValidateIndirectTargetABI(i, target))
	m := &mir2.Module{Name: "zeroarg typed ABI"}
	f := m.AddFunc("caller")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.AddrOf("external", mir2.ClassPointer)
	r := b.CallIndirect(p, nil, mir2.TyU8, mir2.ClassAcc)
	b.Cur.Insts[len(b.Cur.Insts)-1].IndirectReturnTys = []mir2.Ty{mir2.TyU8}
	b.Ret(r)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8}}
	fmt.Println(m.Dump())
}
