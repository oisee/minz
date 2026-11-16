package mir2

import (
	"strings"
	"testing"
)

// looprotWhile builds
//
//	entry:            jmp head(n, 0)
//	head(i, s):       [load] ; cmp.ne i, 0 ; br_if body, exit(s)
//	body:             jmp head(i-1, s+i)
//	exit(r):          ret r
//
// With load, the header also reads a byte through the pointer parameter p
// and the condition tests that byte instead.
func looprotWhile(load bool) *Module {
	m := &Module{}
	f := m.AddFunc("w")
	f.nextReg = 100
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU16, Class: ClassPointer}, {Reg: 2, Ty: TyU16, Class: ClassPointer}}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	head := &Block{Label: "head", Params: []BlockParam{{Dst: 3, Ty: TyU16, Class: ClassPointer}, {Dst: 4, Ty: TyU16, Class: ClassPointer}}}
	tested := Reg(3)
	if load {
		head.Insts = append(head.Insts,
			&Inst{Op: OpLoad, Dst: 11, Src: [2]Reg{2, NoReg}, Ty: TyU8, Cls: ClassGeneral},
			&Inst{Op: OpConst, Dst: 5, Imm: 0, Ty: TyU8, Cls: ClassGeneral})
		tested = 11
	} else {
		head.Insts = append(head.Insts, &Inst{Op: OpConst, Dst: 5, Imm: 0, Ty: TyU16, Cls: ClassPointer})
	}
	srcTy := TyU16
	if load {
		srcTy = TyU8
	}
	head.Insts = append(head.Insts, &Inst{Op: OpCmp, Dst: 6, Src: [2]Reg{tested, 5}, Ty: TyBool, SrcTy: srcTy, Cond: CmpNe, Cls: ClassFlag})
	head.Term = &TermBrIf{Cond: 6, Then: "body", Else: "exit", ElseArgs: []Reg{4}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 7, Imm: 0, Ty: TyU16, Cls: ClassPointer}}, Term: &TermJmp{Target: "head", Args: []Reg{1, 7}}},
		head,
		{Label: "body", Insts: []*Inst{
			{Op: OpConst, Dst: 8, Imm: 1, Ty: TyU16, Cls: ClassPointer},
			{Op: OpSub, Dst: 9, Src: [2]Reg{3, 8}, Ty: TyU16, Cls: ClassPointer},
			{Op: OpAdd, Dst: 10, Src: [2]Reg{4, 3}, Ty: TyU16, Cls: ClassPointer},
		}, Term: &TermJmp{Target: "head", Args: []Reg{9, 10}}},
		{Label: "exit", Params: []BlockParam{{Dst: 12, Ty: TyU16, Class: ClassPointer}}, Term: &TermRet{Vals: []Reg{12}}},
	}
	return m
}

// looprotForward builds an if/else whose arms jump forward to a pure test
// block that is not a loop header:
//
//	entry:   cmp.ugt a, 5 ; br_if then, else
//	then:    jmp join(a-1)        (not adjacent to join)
//	else:    jmp join(a+1)        (falls through)
//	join(x): cmp.ne x, 0 ; br_if yes, no
func looprotForward() *Module {
	m := &Module{}
	f := m.AddFunc("fwd")
	f.nextReg = 100
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{
			{Op: OpConst, Dst: 2, Imm: 5, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpCmp, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyBool, SrcTy: TyU8, Cond: CmpUgt, Cls: ClassFlag},
		}, Term: &TermBrIf{Cond: 3, Then: "then", Else: "else"}},
		{Label: "then", Insts: []*Inst{
			{Op: OpConst, Dst: 4, Imm: 1, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpSub, Dst: 5, Src: [2]Reg{1, 4}, Ty: TyU8, Cls: ClassGeneral},
		}, Term: &TermJmp{Target: "join", Args: []Reg{5}}},
		{Label: "else", Insts: []*Inst{
			{Op: OpConst, Dst: 6, Imm: 1, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpAdd, Dst: 7, Src: [2]Reg{1, 6}, Ty: TyU8, Cls: ClassGeneral},
		}, Term: &TermJmp{Target: "join", Args: []Reg{7}}},
		{Label: "join", Params: []BlockParam{{Dst: 8, Ty: TyU8, Class: ClassGeneral}}, Insts: []*Inst{
			{Op: OpConst, Dst: 9, Imm: 0, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpCmp, Dst: 10, Src: [2]Reg{8, 9}, Ty: TyBool, SrcTy: TyU8, Cond: CmpNe, Cls: ClassFlag},
		}, Term: &TermBrIf{Cond: 10, Then: "yes", Else: "no"}},
		{Label: "yes", Insts: []*Inst{{Op: OpConst, Dst: 11, Imm: 1, Ty: TyU8, Cls: ClassAcc}}, Term: &TermRet{Vals: []Reg{11}}},
		{Label: "no", Insts: []*Inst{{Op: OpConst, Dst: 12, Imm: 2, Ty: TyU8, Cls: ClassAcc}}, Term: &TermRet{Vals: []Reg{12}}},
	}
	return m
}

func looprotCodegen(t *testing.T, m *Module, opts Z80CodegenOptions) string {
	t.Helper()
	if err := Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
	for _, f := range m.Funcs {
		for r, loc := range Allocate(f, ComputeLiveness(f), Z80CostTable{}).Locs {
			ar.Locs[r] = loc
		}
	}
	asm, err := Z80Codegen(m, ar, opts)
	if err != nil {
		t.Fatal(err)
	}
	return asm
}

// funcBody returns the text of one function, up to the next function header.
func funcBody(asm, name string) string {
	start := strings.Index(asm, "\n"+name+":\n")
	if start < 0 {
		return ""
	}
	body := asm[start+1:]
	if end := strings.Index(body, "\n; fun "); end >= 0 {
		body = body[:end]
	}
	return body
}

func TestLoopRotationRepeatsHeaderTestAtLatch(t *testing.T) {
	asm := funcBody(looprotCodegen(t, looprotWhile(false), Z80CodegenOptions{}), "w")
	if !strings.Contains(asm, "; loop rotation: test of head") {
		t.Fatalf("back edge not rotated:\n%s", asm)
	}
	if strings.Contains(asm, "JRS .w_head\n") {
		t.Fatalf("rotated latch still jumps to the header:\n%s", asm)
	}
	// The latch ends in the repeated test: a conditional branch back into
	// the body. The exit edge carries a copy (exit(s) into the return pair),
	// so it leaves through the header's exit trampoline.
	latch := asm[strings.Index(asm, "; loop rotation"):]
	if !strings.Contains(latch, "JRS NZ, .w_body") {
		t.Fatalf("repeated test does not branch to the body:\n%s", asm)
	}
	if n := strings.Count(asm, "OR L"); n != 2 {
		t.Fatalf("want the header test twice, got %d:\n%s", n, asm)
	}
	// Both copies of the test share the header's exit trampoline.
	if n := strings.Count(asm, ".w_trmp0"); n != 3 || strings.Contains(asm, ".w_trmp1") {
		t.Fatalf("want one exit trampoline shared by both copies of the test:\n%s", asm)
	}
}

func TestLoopRotationOffSwitchKeepsJump(t *testing.T) {
	asm := funcBody(looprotCodegen(t, looprotWhile(false), Z80CodegenOptions{DisableLoopRotation: true}), "w")
	if strings.Contains(asm, "loop rotation") || !strings.Contains(asm, "JRS .w_head") {
		t.Fatalf("--no-loop-rotation still rotates:\n%s", asm)
	}
}

func TestLoopRotationRefusedInSizeMode(t *testing.T) {
	asm := funcBody(looprotCodegen(t, looprotWhile(false), Z80CodegenOptions{OptimizeForSize: true}), "w")
	if strings.Contains(asm, "loop rotation") || !strings.Contains(asm, "JRS .w_head") {
		t.Fatalf("size mode rotated a test longer than the JR it replaces:\n%s", asm)
	}
}

func TestLoopRotationRefusesHeaderLoad(t *testing.T) {
	m := looprotWhile(true)
	asm := funcBody(looprotCodegen(t, m, Z80CodegenOptions{}), "w")
	if strings.Contains(asm, "loop rotation") || !strings.Contains(asm, "JRS .w_head") {
		t.Fatalf("header with a memory load was rotated:\n%s", asm)
	}
}

func TestLoopRotationRefusesForwardEdge(t *testing.T) {
	m := looprotForward()
	asm := funcBody(looprotCodegen(t, m, Z80CodegenOptions{}), "fwd")
	if strings.Contains(asm, "loop rotation") {
		t.Fatalf("forward jump to a test block was rotated:\n%s", asm)
	}
	if !strings.Contains(asm, "JRS .fwd_join") {
		t.Fatalf("fixture lost its forward jump:\n%s", asm)
	}
}

func TestAsmByteSize(t *testing.T) {
	for text, want := range map[string]int{
		"    LD A, D\n    OR E\n":                     2,
		"    LD A, H\n    OR L\n    JRS NZ, .x_body\n": 4,
		"    SBC HL, DE    ; 15T\n":                    2,
		"    JP Z, .far\n    CP 7\n":                  5,
	} {
		if got, ok := asmByteSize(text); !ok || got != want {
			t.Fatalf("asmByteSize(%q) = %d, %v; want %d", text, got, ok, want)
		}
	}
	if _, ok := asmByteSize("    NOT_AN_INSTRUCTION A\n"); ok {
		t.Fatal("unknown instruction measured")
	}
}
