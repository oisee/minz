package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// flagRetConds are the conditions a flag-return contract can name.
var flagRetConds = []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpLt, mir2.CmpGe}

// flagRetNatural is the unsigned byte comparison whose own flags are cond.
func flagRetNatural(cond mir2.CmpCond) mir2.CmpCond {
	switch cond {
	case mir2.CmpLt:
		return mir2.CmpUlt
	case mir2.CmpGe:
		return mir2.CmpUge
	}
	return cond
}

// flagRetOther is a comparison whose flags differ from cond.
func flagRetOther(cond mir2.CmpCond) mir2.CmpCond {
	switch cond {
	case mir2.CmpEq:
		return mir2.CmpUlt
	case mir2.CmpNe:
		return mir2.CmpUge
	case mir2.CmpLt:
		return mir2.CmpEq
	}
	return mir2.CmpNe
}

// buildFlagRetCallee adds p(x: u8) -> bool, returned in flags with cond
// meaning true. site selects what its return sites return:
//
//	natural:  ret cmp.<cond> x, 100
//	other:    ret cmp.<another condition> x, 100
//	ugt:      ret cmp.ugt x, 100 (C or Z both false)
//	byte:     a 0/1 block parameter joined from two constants
//	const:    ret 1 / ret 0 in two blocks
//	condRet:  cond_ret (x < 100) returning 0, else ret 1
//	frame:    natural comparison of x read back from a local frame
//	tailSame: ret call q(x), q returning in flags with cond
//	tailOther: ret call q(x), q returning in flags with another condition
//	tailByte: ret call q(x), q returning its bool in A
func buildFlagRetCallee(m *mir2.Module, cond mir2.CmpCond, site string) {
	addPred := func(name string, ret mir2.Return, cmp mir2.CmpCond) {
		q := m.AddFunc(name)
		q.Contract.Returns = []mir2.Return{ret}
		qb := mir2.NewBuilder(q)
		qb.SwitchToNewBlock("entry")
		qx := qb.Param("x", mir2.TyU8, mir2.ClassAcc)
		qk := qb.Const(100, mir2.TyU8, mir2.ClassGeneral)
		qb.Ret(qb.CmpWithSrcTy(cmp, qx, qk, mir2.ClassFlag, false, mir2.TyU8))
	}
	switch site {
	case "tailSame":
		addPred("q", mir2.Return{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: cond}, flagRetNatural(cond))
	case "tailOther":
		other := flagRetConds[(int(indexOfCond(cond))+1)%len(flagRetConds)]
		addPred("q", mir2.Return{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: other}, flagRetNatural(other))
	case "tailByte":
		addPred("q", mir2.Return{Ty: mir2.TyBool, Class: mir2.ClassAcc}, flagRetOther(cond))
	}
	p := m.AddFunc("p")
	p.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: cond}}
	b := mir2.NewBuilder(p)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	cmp100 := func(c mir2.CmpCond, v mir2.Reg) mir2.Reg {
		return b.CmpWithSrcTy(c, v, b.Const(100, mir2.TyU8, mir2.ClassGeneral), mir2.ClassFlag, false, mir2.TyU8)
	}
	switch site {
	case "natural":
		b.Ret(cmp100(flagRetNatural(cond), x))
	case "other":
		b.Ret(cmp100(flagRetOther(cond), x))
	case "ugt":
		b.Ret(cmp100(mir2.CmpUgt, x))
	case "byte":
		b.BrIf(cmp100(mir2.CmpUlt, x), "t", nil, "f", nil)
		b.SwitchToNewBlock("t")
		b.Jmp("j", b.Const(1, mir2.TyBool, mir2.ClassGeneral))
		b.SwitchToNewBlock("f")
		b.Jmp("j", b.Const(0, mir2.TyBool, mir2.ClassGeneral))
		j := b.SwitchToNewBlock("j")
		b.Ret(b.BlockParam(j, mir2.TyBool, mir2.ClassGeneral))
	case "const":
		b.BrIf(cmp100(mir2.CmpUlt, x), "t", nil, "f", nil)
		b.SwitchToNewBlock("t")
		b.Ret(b.Const(1, mir2.TyBool, mir2.ClassGeneral))
		b.SwitchToNewBlock("f")
		b.Ret(b.Const(0, mir2.TyBool, mir2.ClassGeneral))
	case "condRet":
		c := cmp100(mir2.CmpUlt, x)
		zero := b.Const(0, mir2.TyBool, mir2.ClassGeneral)
		b.Cur.Seal(&mir2.TermCondRet{Cond: c, Vals: []mir2.Reg{zero}, Then: "t"})
		b.SwitchToNewBlock("t")
		b.Ret(b.Const(1, mir2.TyBool, mir2.ClassGeneral))
	case "frame":
		slot := b.Alloca(1)
		b.Store(slot, x, mir2.TyU8)
		b.Ret(cmp100(flagRetNatural(cond), b.Load(slot, mir2.TyU8, mir2.ClassAcc)))
	case "tailSame", "tailOther", "tailByte":
		b.Ret(b.Call("q", []mir2.Reg{x}, mir2.TyBool, m.FuncByName("q").Contract.Returns[0].Class, mir2.CallAttrs{}))
	default:
		panic("unknown site " + site)
	}
}

func indexOfCond(c mir2.CmpCond) int {
	for i, x := range flagRetConds {
		if x == c {
			return i
		}
	}
	panic("condition")
}

// buildFlagRetCaller adds pick(x: u8) -> u8 around %r = call p(x), with use:
//
//	br:      br_if %r                        yes: ret 5        no: ret 9
//	negBr:   br_if (cmp.eq %r, 0)            yes: ret 5        no: ret 9
//	ret:     ret %r (pick returns its bool in A)
//	pick:    jmp @b; @b: br_if %r            (%r lives across blocks: byte home)
//	across:  %n = call noise(x); br_if %r    yes: ret %n       no: ret 9
//	liveA:   %a = x + 3 before the call; br_if %r  yes: ret %a  no: ret 9
//	neBr:    br_if (cmp.ne %r, 0)            yes: ret 5        no: ret 9
//	neCondRet, eqCondRet: cond_ret (cmp.ne/eq %r, 0) returning 9, else ret 5
//	eqExt:   %c = cmp.eq %r, 0; br_if %c     yes: ret ext %c   no: ret 9
func buildFlagRetCaller(m *mir2.Module, use string) (*mir2.Func, mir2.Reg, mir2.Reg, mir2.Reg) {
	if use == "across" {
		n := m.AddFunc("noise")
		n.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		nb := mir2.NewBuilder(n)
		nb.SwitchToNewBlock("entry")
		nx := nb.Param("x", mir2.TyU8, mir2.ClassAcc)
		nb.Ret(nb.Add(nx, nb.Const(7, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.ClassAcc))
	}
	f := m.AddFunc("pick")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	if use == "ret" {
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}}
	}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
	var a mir2.Reg
	if use == "liveA" {
		a = b.Add(x, b.Const(3, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.ClassAcc)
	}
	r := b.Call("p", []mir2.Reg{x}, mir2.TyBool, mir2.ClassFlag, mir2.CallAttrs{})
	cond := r
	yes := mir2.NoReg
	switch use {
	case "br":
	case "negBr", "eqExt":
		cond = b.CmpWithSrcTy(mir2.CmpEq, r, b.Const(0, mir2.TyBool, mir2.ClassGeneral), mir2.ClassFlag, false, mir2.TyBool)
		if use == "eqExt" {
			yes = b.Ext(cond, mir2.TyBool, mir2.TyU8, mir2.ClassAcc)
		}
	case "neBr":
		cond = b.CmpWithSrcTy(mir2.CmpNe, b.Const(0, mir2.TyBool, mir2.ClassGeneral), r, mir2.ClassFlag, false, mir2.TyBool)
	case "neCondRet", "eqCondRet":
		cc := mir2.CmpNe
		if use == "eqCondRet" {
			cc = mir2.CmpEq
		}
		cond = b.CmpWithSrcTy(cc, r, b.Const(0, mir2.TyBool, mir2.ClassGeneral), mir2.ClassFlag, false, mir2.TyBool)
		b.Cur.Seal(&mir2.TermCondRet{Cond: cond, Vals: []mir2.Reg{b.Const(9, mir2.TyU8, mir2.ClassAcc)}, Then: "yes"})
		b.SwitchToNewBlock("yes")
		b.Ret(b.Const(5, mir2.TyU8, mir2.ClassAcc))
		return f, x, r, a
	case "ret":
		b.Ret(r)
		return f, x, r, a
	case "pick":
		b.Jmp("b")
		b.SwitchToNewBlock("b")
	case "across":
		yes = b.Call("noise", []mir2.Reg{x}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	case "liveA":
		yes = a
	default:
		panic("unknown use " + use)
	}
	b.BrIf(cond, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	if yes == mir2.NoReg {
		yes = b.Const(5, mir2.TyU8, mir2.ClassAcc)
	}
	b.Ret(yes)
	b.SwitchToNewBlock("no")
	b.Ret(b.Const(9, mir2.TyU8, mir2.ClassAcc))
	return f, x, r, a
}

// compileFlagRet runs the production allocation sequence on every function.
func compileFlagRet(t *testing.T, m *mir2.Module, pbqp bool) (*mir2.AllocResult, string) {
	t.Helper()
	if e := mir2.Verify(m); e != nil {
		t.Fatal(e)
	}
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, f := range m.Funcs {
		mir2.PreallocCoalesce(f)
		lr := mir2.ComputeLiveness(f)
		far := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		if pbqp {
			far = mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})
		}
		if e := mir2.InsertAccSaves(f, far); e != nil {
			t.Fatal(e)
		}
		for r, loc := range far.Locs {
			ar.Locs[r] = loc
		}
		ar.Spilled = append(ar.Spilled, far.Spilled...)
	}
	asm, e := mir2.Z80Codegen(m, ar)
	if e != nil {
		t.Fatalf("%v\n%s", e, asm)
	}
	return ar, asm
}

var flagRetSites = []string{"natural", "other", "ugt", "byte", "const", "condRet", "frame", "tailSame", "tailOther", "tailByte"}

// Each return-site kind of a flag-return callee, under each caller use and
// compiled through the production allocation sequence, must agree with the
// MIR2 VM for every input byte.
func TestFlagReturnCallerUses(t *testing.T) {
	for _, alloc := range []string{"pbqp", "greedy"} {
		for _, cond := range flagRetConds {
			for _, site := range flagRetSites {
				for _, use := range []string{"br", "negBr", "neBr", "neCondRet", "eqCondRet", "eqExt", "ret", "pick", "across", "liveA"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", alloc, cond, site, use), func(t *testing.T) {
						m := &mir2.Module{}
						buildFlagRetCallee(m, cond, site)
						f, x, _, _ := buildFlagRetCaller(m, use)
						ar, asm := compileFlagRet(t, m, alloc == "pbqp")
						if testing.Verbose() {
							t.Log(asm)
						}
						runFlagMove(t, flagMoveFunc{m: m, f: f}, cond, ar.Locs[x].Name, asm)
					})
				}
			}
		}
	}
}

// A caller value held in A across the call is saved with PUSH AF. POP AF
// would restore the caller's flags over the result; the saved A must come
// back through a dead pair, or through the saved stack image when B, D and H
// hold live values too, whatever the result's home.
func TestFlagReturnCallerLiveA(t *testing.T) {
	homes := []mir2.PhysLoc{
		{Kind: mir2.LocFlag, Name: "F"}, {Kind: mir2.LocReg, Name: "B"},
		{Kind: mir2.LocReg, Name: "E"}, {Kind: mir2.LocMem, Name: "mem", Offset: 0xF000},
	}
	for extras := 0; extras <= 3; extras++ {
		for _, home := range homes {
			if home.Name == "B" && extras > 0 {
				home.Name = "L"
			}
			if home.Name == "E" && extras > 1 {
				home.Name = "IXL" // the pickup goes through A
				home.Kind = mir2.LocIXY8
			}
			for _, cond := range flagRetConds {
				t.Run(fmt.Sprintf("%d/%s/%s", extras, home.Name, cond), func(t *testing.T) {
					m := &mir2.Module{}
					buildFlagRetCallee(m, cond, "natural")
					ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
					p := m.FuncByName("p")
					mir2.PreallocCoalesce(p)
					par := mir2.PBQPAllocate(p, mir2.ComputeLiveness(p), mir2.Z80CostTable{})
					if e := mir2.InsertAccSaves(p, par); e != nil {
						t.Fatal(e)
					}
					for r, loc := range par.Locs {
						ar.Locs[r] = loc
					}
					reg := func(name string) mir2.PhysLoc { return mir2.PhysLoc{Kind: mir2.LocReg, Name: name} }
					// pick(x): e_i = x + x; a = x + 3; r = call p(x)
					//   br_if r, yes, no; yes: ret a + e_1 + ...; no: ret 9
					f := m.AddFunc("pick")
					f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
					b := mir2.NewBuilder(f)
					b.SwitchToNewBlock("entry")
					x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
					ar.Locs[x] = reg("C")
					var es []mir2.Reg
					for i := 0; i < extras; i++ {
						e := b.Add(x, x, mir2.TyU8, mir2.ClassGeneral)
						ar.Locs[e] = reg([]string{"B", "D", "H"}[i])
						es = append(es, e)
					}
					k := b.Const(3, mir2.TyU8, mir2.ClassGeneral)
					ar.Locs[k] = reg("E")
					a := b.Add(x, k, mir2.TyU8, mir2.ClassAcc)
					ar.Locs[a] = reg("A")
					r := b.Call("p", []mir2.Reg{x}, mir2.TyBool, mir2.ClassFlag, mir2.CallAttrs{})
					ar.Locs[r] = home
					if home.Kind == mir2.LocMem {
						ar.Spilled = append(ar.Spilled, r)
					}
					b.BrIf(r, "yes", nil, "no", nil)
					b.SwitchToNewBlock("yes")
					sum := a
					for _, e := range es {
						sum = b.Add(sum, e, mir2.TyU8, mir2.ClassAcc)
						ar.Locs[sum] = reg("A")
					}
					b.Ret(sum)
					b.SwitchToNewBlock("no")
					nine := b.Const(9, mir2.TyU8, mir2.ClassAcc)
					ar.Locs[nine] = reg("A")
					b.Ret(nine)
					if e := mir2.Verify(m); e != nil {
						t.Fatal(e)
					}
					asm, e := mir2.Z80Codegen(m, ar)
					if e != nil {
						t.Fatalf("%v\n%s", e, asm)
					}
					if !strings.Contains(asm, "PUSH AF") {
						t.Fatalf("A not saved across the call\n%s", asm)
					}
					if testing.Verbose() {
						t.Log(asm)
					}
					runFlagMove(t, flagMoveFunc{m: m, f: f}, cond, "C", asm)
				})
			}
		}
	}
}

// The zero test of a flag-return call's result folds into the branch:
// cmp.ne %r, 0 becomes %r, cmp.eq %r, 0 becomes %r with the arms swapped.
// A cond_ret cannot swap arms and a comparison with another use stays.
// The VM result is the same before and after, and the Z80 agrees with it.
func TestFoldFlagReturnTests(t *testing.T) {
	folds := map[string]bool{"negBr": true, "neBr": true, "neCondRet": true, "eqCondRet": false, "eqExt": false}
	for _, cond := range flagRetConds {
		for _, use := range []string{"negBr", "neBr", "neCondRet", "eqCondRet", "eqExt"} {
			for _, flagCallee := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/%v", cond, use, flagCallee), func(t *testing.T) {
					m := &mir2.Module{}
					buildFlagRetCallee(m, cond, "natural")
					if !flagCallee {
						p := m.FuncByName("p")
						p.Contract.Returns[0] = mir2.Return{Ty: mir2.TyBool, Class: mir2.ClassAcc}
					}
					f, x, r, _ := buildFlagRetCaller(m, use)
					var before []int64
					vm := mir2.NewVM(m)
					for v := int64(0); v < 256; v++ {
						out, e := vm.CallFunc(f, []mir2.Value{{I: v}})
						if e != nil {
							t.Fatal(e)
						}
						before = append(before, out[0].I)
					}
					want := folds[use] && flagCallee
					if got := mir2.FoldFlagReturnTests(m); got != want {
						t.Fatalf("fold reported %v, want %v\n%s", got, want, m.Dump())
					}
					if e := mir2.Verify(m); e != nil {
						t.Fatal(e)
					}
					cmps := 0
					for _, b := range f.Blocks {
						for _, inst := range b.Insts {
							if inst.Op == mir2.OpCmp {
								cmps++
							}
						}
						switch term := b.Term.(type) {
						case *mir2.TermBrIf:
							if want && term.Cond != r {
								t.Fatalf("branch on r%d, want the call result r%d\n%s", term.Cond, r, m.Dump())
							}
						case *mir2.TermCondRet:
							if want && term.Cond != r {
								t.Fatalf("cond_ret on r%d, want the call result r%d\n%s", term.Cond, r, m.Dump())
							}
						}
					}
					if want == (cmps != 0) {
						t.Fatalf("%d comparisons left after fold=%v\n%s", cmps, want, m.Dump())
					}
					vm = mir2.NewVM(m)
					for v := int64(0); v < 256; v++ {
						out, e := vm.CallFunc(f, []mir2.Value{{I: v}})
						if e != nil {
							t.Fatal(e)
						}
						if out[0].I != before[v] {
							t.Fatalf("x=%d: VM %d after the fold, %d before\n%s", v, out[0].I, before[v], m.Dump())
						}
					}
					if !flagCallee {
						return
					}
					ar, asm := compileFlagRet(t, m, true)
					runFlagMove(t, flagMoveFunc{m: m, f: f}, cond, ar.Locs[x].Name, asm)
				})
			}
		}
	}
}

// buildByteRetModule builds p(x: u8) -> bool returning cmp.<cmp> x, 100 in
// A, and pick(x: u8) -> u8 calling it as use says:
//
//	br:       br_if (cmp.ne %r, 0)            yes: ret 5   no: ret 9
//	stored:   yes: ret ext %r                   (a value use)
//	jmpArg:   jmp @j(%r); @j(%b): br_if %b      (a block argument)
//	addr:     br as above, plus addr_of @p
//	asm:      br as above, plus inline asm "CALL p"
//	pass:     pick branches on q(x), q(x) -> bool returning call p(x)
//	passVal:  pick returns ext q(x), q returning call p(x)
func buildByteRetModule(cmp mir2.CmpCond, name, use string) (*mir2.Module, *mir2.Func, mir2.Reg) {
	m := &mir2.Module{}
	p := m.AddFunc(name)
	p.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}}
	pb := mir2.NewBuilder(p)
	pb.SwitchToNewBlock("entry")
	px := pb.Param("x", mir2.TyU8, mir2.ClassAcc)
	pb.Ret(pb.CmpWithSrcTy(cmp, px, pb.Const(100, mir2.TyU8, mir2.ClassGeneral), mir2.ClassFlag, false, mir2.TyU8))
	callee := name
	if use == "pass" || use == "passVal" {
		q := m.AddFunc("q")
		q.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}}
		qb := mir2.NewBuilder(q)
		qb.SwitchToNewBlock("entry")
		qx := qb.Param("x", mir2.TyU8, mir2.ClassAcc)
		qb.Ret(qb.Call(name, []mir2.Reg{qx}, mir2.TyBool, mir2.ClassAcc, mir2.CallAttrs{}))
		callee = "q"
	}
	f := m.AddFunc("pick")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
	switch use {
	case "addr":
		b.AddrOf(name, mir2.ClassPointer)
	case "asm":
		b.Asm("    CALL "+name, nil, nil, mir2.ClassSet(0), mir2.TyVoid, mir2.ClassGeneral)
	}
	r := b.Call(callee, []mir2.Reg{x}, mir2.TyBool, mir2.ClassAcc, mir2.CallAttrs{})
	yes := mir2.NoReg
	cond := r
	switch use {
	case "stored", "passVal":
		yes = b.Ext(r, mir2.TyBool, mir2.TyU8, mir2.ClassAcc)
	case "jmpArg":
		b.Jmp("j", r)
		j := b.SwitchToNewBlock("j")
		cond = b.BlockParam(j, mir2.TyBool, mir2.ClassGeneral)
	default:
		cond = b.CmpWithSrcTy(mir2.CmpNe, r, b.Const(0, mir2.TyBool, mir2.ClassGeneral), mir2.ClassFlag, false, mir2.TyBool)
	}
	b.BrIf(cond, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	if yes == mir2.NoReg {
		yes = b.Const(5, mir2.TyU8, mir2.ClassAcc)
	}
	b.Ret(yes)
	b.SwitchToNewBlock("no")
	b.Ret(b.Const(9, mir2.TyU8, mir2.ClassAcc))
	return m, f, x
}

// The contract pass converts a predicate only when every call result is a
// branch condition (or returned by another converted predicate), chooses
// the condition the comparison leaves in the flags, and patches each call's
// result class. Refused predicates keep their byte return.
func TestApplyFlagReturnContracts(t *testing.T) {
	type tc struct {
		cmp     mir2.CmpCond
		name    string
		use     string
		chosen  map[string]mir2.CmpCond
		refused map[string]string
	}
	cases := []tc{
		{mir2.CmpEq, "p", "br", map[string]mir2.CmpCond{"p": mir2.CmpEq}, nil},
		{mir2.CmpNe, "p", "br", map[string]mir2.CmpCond{"p": mir2.CmpNe}, nil},
		{mir2.CmpUlt, "p", "br", map[string]mir2.CmpCond{"p": mir2.CmpLt}, nil},
		{mir2.CmpUge, "p", "br", map[string]mir2.CmpCond{"p": mir2.CmpGe}, nil},
		{mir2.CmpUgt, "p", "br", map[string]mir2.CmpCond{"p": mir2.CmpNe}, nil},
		{mir2.CmpUlt, "p", "stored", nil, map[string]string{"p": "result used as a value"}},
		{mir2.CmpUlt, "p", "jmpArg", nil, map[string]string{"p": "result used as a value"}},
		{mir2.CmpUlt, "p", "addr", nil, map[string]string{"p": "address taken"}},
		{mir2.CmpUlt, "p", "asm", nil, map[string]string{"p": "named in inline asm"}},
		{mir2.CmpUlt, "main", "br", nil, map[string]string{"main": "entry point"}},
		{mir2.CmpUge, "p", "pass", map[string]mir2.CmpCond{"p": mir2.CmpGe, "q": mir2.CmpGe}, nil},
		{mir2.CmpUge, "p", "passVal", nil, map[string]string{"p": "result used as a value", "q": "result used as a value"}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%s/%s", c.cmp, c.name, c.use), func(t *testing.T) {
			m, f, x := buildByteRetModule(c.cmp, c.name, c.use)
			rep, e := mir2.ApplyFlagReturnContracts(m, nil)
			if e != nil {
				t.Fatal(e)
			}
			if len(rep.Chosen) != len(c.chosen) {
				t.Fatalf("chosen %v, want %v (refused %v)", rep.Chosen, c.chosen, rep.Refused)
			}
			for name, cond := range c.chosen {
				if rep.Chosen[name] != cond {
					t.Fatalf("%s: chosen %v, want %v", name, rep.Chosen[name], cond)
				}
				ret := m.FuncByName(name).Contract.Returns[0]
				if ret.Class != mir2.ClassFlag || ret.FlagCond != cond {
					t.Fatalf("%s: contract %+v", name, ret)
				}
			}
			for name, reason := range c.refused {
				if rep.Refused[name] != reason {
					t.Fatalf("%s: refused %q, want %q", name, rep.Refused[name], reason)
				}
				if m.FuncByName(name).Contract.Returns[0].Class == mir2.ClassFlag {
					t.Fatalf("%s: refused but returns in flags", name)
				}
			}
			for _, g := range m.Funcs {
				for _, b := range g.Blocks {
					for _, inst := range b.Insts {
						_, flag := rep.Chosen[inst.Sym]
						if inst.Op == mir2.OpCall && flag != (inst.Cls == mir2.ClassFlag) {
							t.Fatalf("call %s in %s has class %v", inst.Sym, g.Name, inst.Cls)
						}
						if inst.Op == mir2.OpCmp && g == f && len(c.chosen) != 0 {
							t.Fatal("zero test not folded")
						}
					}
				}
			}
			if c.use == "asm" || c.name == "main" {
				return // the template's CALL and the entry run outside this harness
			}
			ar, asm := compileFlagRet(t, m, true)
			runFlagMove(t, flagMoveFunc{m: m, f: f}, c.cmp, ar.Locs[x].Name, asm)
		})
	}
}

// A pinned module converts exactly the pinned predicates; a pinned
// predicate that is not eligible is an error, never a silent change.
func TestApplyFlagReturnContractsPinned(t *testing.T) {
	m, _, _ := buildByteRetModule(mir2.CmpUlt, "p", "br")
	rep, e := mir2.ApplyFlagReturnContracts(m, map[string]mir2.CmpCond{})
	if e != nil || len(rep.Chosen) != 0 || rep.Refused["p"] != "not chosen for this module" {
		t.Fatalf("empty pin: %v %v", rep, e)
	}
	m, _, _ = buildByteRetModule(mir2.CmpUlt, "p", "br")
	rep, e = mir2.ApplyFlagReturnContracts(m, map[string]mir2.CmpCond{"p": mir2.CmpNe})
	if e != nil || rep.Chosen["p"] != mir2.CmpNe || m.FuncByName("p").Contract.Returns[0].FlagCond != mir2.CmpNe {
		t.Fatalf("pinned condition: %v %v", rep, e)
	}
	m, _, _ = buildByteRetModule(mir2.CmpUlt, "p", "stored")
	if _, e = mir2.ApplyFlagReturnContracts(m, map[string]mir2.CmpCond{"p": mir2.CmpLt}); e == nil {
		t.Fatal("ineligible pinned predicate accepted")
	}
}
