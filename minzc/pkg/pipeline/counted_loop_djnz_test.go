package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestCountedLoopDJNZVMZ80(t *testing.T) {
	for _, op := range []string{"<", "!="} {
		h := &hir.Module{Funcs: []*hir.Func{{Name: "probe", Params: []hir.Param{{Name: "i", Ty: mir2.TyU8}, {Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: hir.Blk(
			hir.Decl("sum", mir2.TyU8, hir.U8(0)),
			hir.While(&hir.BinExpr{Op: op, L: hir.Var("i", mir2.TyU8), R: hir.Var("n", mir2.TyU8), Ty: mir2.TyBool}, hir.Blk(
				hir.Assign(hir.Var("sum", mir2.TyU8), hir.Add(hir.Var("sum", mir2.TyU8), hir.Var("i", mir2.TyU8), mir2.TyU8)),
				hir.Assign(hir.Var("i", mir2.TyU8), hir.Add(hir.Var("i", mir2.TyU8), hir.U8(1), mir2.TyU8)))),
			hir.Ret(hir.Add(hir.Cast(hir.Var("sum", mir2.TyU8), mir2.TyU16), hir.Mul(hir.Cast(hir.Var("i", mir2.TyU8), mir2.TyU16), hir.U16(256), mir2.TyU16), mir2.TyU16)))}}}
		m := hir.LowerModule(h)
		if err := mir2.Verify(m); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, b := range m.Funcs[0].Blocks {
			if _, ok := b.Term.(*mir2.TermDJNZ); ok {
				found = true
			}
		}
		if !found {
			t.Fatal("counted loop must lower to TermDJNZ", op)
		}
		points := []int64{0, 1, 7, 8, 127, 128, 254, 255}
		for _, s := range points {
			for _, n := range points {
				i, sum := byte(s), byte(0)
				for (op == "<" && i < byte(n)) || (op == "!=" && i != byte(n)) {
					sum += i
					i++
				}
				want := int64(sum) + int64(i)*256
				got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: s}, {I: n}})
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("%s %d..%d: %v %v want %d", op, s, n, got, err, want)
				}
				for _, grace := range []bool{false, true} {
					measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none", UseHIRGrace: grace}, []int64{s, n}, nil, want)
				}
			}
		}
	}
	// i is read only below a conditional expression. It must remain an
	// up-counter, and read-only n must survive the body's byte arithmetic.
	h := &hir.Module{Funcs: []*hir.Func{{Name: "probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(
		hir.Decl("i", mir2.TyU8, hir.U8(0)), hir.Decl("s", mir2.TyU8, hir.U8(0)),
		hir.While(hir.Lt(hir.Var("i", mir2.TyU8), hir.Var("n", mir2.TyU8)), hir.Blk(
			hir.Assign(hir.Var("s", mir2.TyU8), hir.Add(hir.Var("s", mir2.TyU8), &hir.CondExpr{Cond: hir.Eq(hir.Var("i", mir2.TyU8), hir.U8(2)), Then: hir.Var("n", mir2.TyU8), Else: hir.U8(1), Ty: mir2.TyU8}, mir2.TyU8)),
			hir.Assign(hir.Var("i", mir2.TyU8), hir.Add(hir.Var("i", mir2.TyU8), hir.U8(1), mir2.TyU8)))),
		hir.Ret(hir.Var("s", mir2.TyU8)))}}}
	m := hir.LowerModule(h)
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int64{0, 1, 2, 3, 8, 128, 255} {
		want := n
		if n > 2 {
			want = 2*n - 1
		}
		want = int64(byte(want))
		got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: n}})
		if err != nil || got[0].I != want {
			t.Fatal("conditional IV use", n, got, err, want)
		}
		for _, grace := range []bool{false, true} {
			measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none", UseHIRGrace: grace}, []int64{n}, nil, want)
		}
	}
	// The exit needs old b while the back edge assigns b = a+b. A value
	// relocated out of A must survive both edges of TermDJNZ.
	h = &hir.Module{Funcs: []*hir.Func{{Name: "probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(
		hir.Decl("a", mir2.TyU8, hir.U8(0)), hir.Decl("b", mir2.TyU8, hir.U8(1)), hir.Decl("i", mir2.TyU8, hir.U8(0)),
		hir.While(hir.Lt(hir.Var("i", mir2.TyU8), hir.Var("n", mir2.TyU8)), hir.Blk(
			hir.Decl("temp", mir2.TyU8, hir.Add(hir.Var("a", mir2.TyU8), hir.Var("b", mir2.TyU8), mir2.TyU8)),
			hir.Assign(hir.Var("a", mir2.TyU8), hir.Var("b", mir2.TyU8)), hir.Assign(hir.Var("b", mir2.TyU8), hir.Var("temp", mir2.TyU8)),
			hir.Assign(hir.Var("i", mir2.TyU8), hir.Add(hir.Var("i", mir2.TyU8), hir.U8(1), mir2.TyU8)))),
		hir.Ret(hir.Var("a", mir2.TyU8)))}}}
	m = hir.LowerModule(h)
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int64{0, 1, 2, 3, 8, 127, 128, 254, 255} {
		a, b := byte(0), byte(1)
		for i := int64(0); i < n; i++ {
			a, b = b, a+b
		}
		got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: n}})
		if err != nil || got[0].I != int64(a) {
			t.Fatal("parallel state", n, got, err, a)
		}
		for _, grace := range []bool{false, true} {
			measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none", ContractOpt: true, UseHIRGrace: grace, UseGrace: grace}, []int64{n}, nil, int64(a))
		}
	}
}

func TestCountedLoopC89AndRangeShape(t *testing.T) {
	h, err := c89.CompileWithOpts(`unsigned short probe(unsigned char n){unsigned char i,bit; unsigned short s=0;for(i=0;i<n;i++){for(bit=0;bit<8;bit++)s+=1;}return s+i;}`, "counted.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(h)
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, b := range m.Funcs[0].Blocks {
		if _, ok := b.Term.(*mir2.TermDJNZ); ok {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("C counted loops: got %d DJNZ, want 2", count)
	}
	for _, n := range []int64{0, 1, 8, 128, 255} {
		got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: n}})
		if err != nil || got[0].I != n*9 {
			t.Fatal(n, got, err)
		}
		measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none"}, []int64{n}, nil, n*9)
	}
	for _, control := range []bool{false, true} {
		body := hir.Blk(hir.Assign(hir.Var("s", mir2.TyU8), hir.Add(hir.Var("s", mir2.TyU8), hir.Var("i", mir2.TyU8), mir2.TyU8)))
		if control {
			body.Body = append([]hir.Stmt{hir.Continue()}, body.Body...)
		}
		h = &hir.Module{Funcs: []*hir.Func{{Name: "probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(hir.Decl("s", mir2.TyU8, hir.U8(0)), &hir.ForRangeStmt{Var: "i", Start: hir.U8(0), End: hir.Var("n", mir2.TyU8), Body: body}, hir.Ret(hir.Var("s", mir2.TyU8)))}}}
		m = hir.LowerModule(h)
		count = 0
		for _, b := range m.Funcs[0].Blocks {
			if _, ok := b.Term.(*mir2.TermDJNZ); ok {
				count++
			}
		}
		if (!control && count != 1) || (control && count != 0) {
			t.Fatal("range fallback", control, count)
		}
		for _, n := range []int64{0, 1, 8, 255} {
			want := int64(byte(n * (n - 1) / 2))
			if control {
				want = 0
			}
			got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: n}})
			if err != nil || got[0].I != want {
				t.Fatal("range", control, n, got, err, want)
			}
			measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none"}, []int64{n}, nil, want)
		}
	}
}

func TestCountedLoopProofAndFallback(t *testing.T) {
	// ForRange currently binds the raw start register at the end's type.
	// A narrow register must not be compared directly with a wide endpoint.
	for _, startTy := range []mir2.Ty{mir2.TyU8, mir2.TyU16} {
		h := &hir.Module{Funcs: []*hir.Func{{Name: "probe", RetTy: mir2.TyU16, Body: hir.Blk(
			hir.Decl("s", mir2.TyU16, hir.U16(0)),
			&hir.ForRangeStmt{Var: "i", Start: &hir.IntLitExpr{Val: 250, Ty: startTy}, End: hir.U16(260), Body: hir.Blk(hir.Assign(hir.Var("s", mir2.TyU16), hir.Add(hir.Var("s", mir2.TyU16), hir.U16(1), mir2.TyU16)))},
			hir.Ret(hir.Var("s", mir2.TyU16)))}}}
		m := hir.LowerModule(h)
		if err := mir2.Verify(m); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, b := range m.Funcs[0].Blocks {
			if _, ok := b.Term.(*mir2.TermDJNZ); ok {
				found = true
			}
		}
		if found != (startTy == mir2.TyU16) {
			t.Fatal("range start width", startTy, found)
		}
		got, err := mir2.NewVM(m).Call("probe", nil)
		if err != nil || got[0].I != 10 {
			t.Fatal(startTy, got, err)
		}
		measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none"}, nil, nil, 10)
	}
	for _, tc := range []struct {
		name     string
		ty       mir2.Ty
		lo, hi   int64
		mode     string
		wantDJNZ bool
	}{
		{"wide255", mir2.TyU16, 65280, 65535, "", true},
		{"wide256", mir2.TyU16, 0, 256, "", false},
		{"narrowStep", mir2.TyU16, 250, 260, "narrowStep", false},
		{"narrowLiteral", mir2.TyU16, 250, 256, "narrowLiteral", false},
		{"wideUnknown", mir2.TyU16, 0, 8, "unknown", false},
		{"signed", mir2.TyI8, 0, 8, "", false},
		{"mutateBound", mir2.TyU8, 0, 8, "bound", false},
		{"mutateIV", mir2.TyU8, 0, 8, "iv", false},
		{"mutateIVBit", mir2.TyU8, 0, 8, "bit", false},
		{"continue", mir2.TyU8, 0, 8, "continue", false},
		{"break", mir2.TyU8, 0, 8, "break", false},
		{"return", mir2.TyU8, 0, 8, "return", false},
		{"call", mir2.TyU8, 0, 8, "call", false},
		{"volatile", mir2.TyU8, 0, 8, "volatile", false},
		{"memoryBound", mir2.TyU8, 0, 8, "global", false},
		{"index", mir2.TyU8, 0, 8, "index", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lit := func(n int64) hir.Expr { return &hir.IntLitExpr{Val: n, Ty: tc.ty} }
			iv := func() hir.Expr { return hir.Var("i", tc.ty) }
			body := hir.Blk()
			switch tc.mode {
			case "bound":
				body.Body = append(body.Body, hir.Assign(hir.Var("n", tc.ty), lit(7)))
			case "iv":
				body.Body = append(body.Body, hir.Assign(iv(), lit(6)))
			case "bit":
				body.Body = append(body.Body, hir.Assign(&hir.BitExpr{X: iv(), Bit: 0}, lit(1)))
			case "continue":
				body.Body = append(body.Body, hir.Continue())
			case "break":
				body.Body = append(body.Body, hir.Break())
			case "return":
				body.Body = append(body.Body, hir.Ret(iv()))
			case "call":
				body.Body = append(body.Body, &hir.ExprStmt{Expr: hir.Call("identity", tc.ty, iv())})
			case "index":
				body.Body = append(body.Body, &hir.ExprStmt{Expr: &hir.IndexExpr{Base: hir.Addr("data"), Idx: iv(), ElemTy: mir2.TyU8}})
			}
			stepTy := tc.ty
			if tc.mode == "narrowStep" {
				stepTy = mir2.TyU8
			}
			body.Body = append(body.Body, hir.Assign(iv(), hir.Add(iv(), lit(1), stepTy)))
			bound := hir.Expr(lit(tc.hi))
			if tc.mode == "narrowLiteral" {
				bound = &hir.IntLitExpr{Val: 256, Ty: mir2.TyU8}
			}
			if tc.mode == "bound" || tc.mode == "unknown" {
				bound = hir.Var("n", tc.ty)
			}
			if tc.mode == "global" {
				bound = hir.Var("limit", tc.ty)
			}
			if tc.mode == "volatile" {
				bound = &hir.VarRefExpr{Name: "n", Ty: tc.ty, Volatile: true}
			}
			h := &hir.Module{Globals: []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 256}}}, Funcs: []*hir.Func{
				{Name: "identity", Params: []hir.Param{{Name: "x", Ty: tc.ty}}, RetTy: tc.ty, Body: hir.Blk(hir.Ret(hir.Var("x", tc.ty)))},
				{Name: "probe", Params: []hir.Param{{Name: "n", Ty: tc.ty}}, RetTy: tc.ty, Body: hir.Blk(hir.Decl("i", tc.ty, lit(tc.lo)), hir.While(hir.Lt(iv(), bound), body), hir.Ret(iv()))},
			}}
			if tc.mode == "global" {
				h.Globals = append(h.Globals, mir2.Global{Name: "limit", Ty: tc.ty, Init: []byte{8}})
			}
			m := hir.LowerModule(h)
			if err := mir2.Verify(m); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, b := range m.FuncByName("probe").Blocks {
				if _, ok := b.Term.(*mir2.TermDJNZ); ok {
					found = true
				}
			}
			if found != tc.wantDJNZ {
				t.Fatalf("DJNZ=%v want %v", found, tc.wantDJNZ)
			}
			if tc.wantDJNZ {
				got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: tc.hi}})
				if err != nil || got[0].I != tc.hi {
					t.Fatal(got, err)
				}
				measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none"}, []int64{tc.hi}, nil, tc.hi)
			}
		})
	}
}
