package mir2

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

// exactLeaf returns a parameterless MIR function named name; the proofs below
// read its emitted body from the assembly text given to them.
func exactLeaf(m *Module, name string, ret *Return) *Func {
	f := m.AddFunc(name)
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	if ret != nil {
		v := b.Const(1, ret.Ty, ret.Class)
		b.Ret(v)
		f.Contract.Returns = []Return{*ret}
	} else {
		b.Ret()
	}
	return f
}

func writeList(w map[string]bool) string {
	var out []string
	for name := range w {
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestExactWritesAcceptWholeBodies(t *testing.T) {
	m := &Module{}
	u8 := exactLeaf(m, "leaf", &Return{Ty: TyU8, Class: ClassAcc})
	for _, tc := range []struct{ name, body, want string }{
		{"multi-block", `
    LD A, (counter)
    CP 5
    JRS NC, .leaf_big
    INC A
    RET
.leaf_big:
    LD B, 0
    LD A, B
    RET
`, "A,B,F"},
		{"loop with DJNZ", `
    LD B, 4
    XOR A
.leaf_loop:
    ADD A, B
    DJNZ .leaf_loop
    RET
`, "A,B,F"},
		{"conditional RET", `
    LD A, (counter)
    AND A
    RET Z
    LD C, A
    RET
`, "A,C,F"},
		{"EXX swaps three pairs", `
    EXX
    LD B, 1
    EXX
    LD A, 1
    RET
`, "A,B,C,D,E,H,L"},
		{"EX AF, AF'", `
    EX AF, AF'
    LD A, 1
    EX AF, AF'
    RET
`, "A,F"},
		{"balanced stack and frame reads", `
    PUSH BC
    LD HL, 0
    ADD HL, SP
    EX (SP), HL
    POP BC
    LD (counter), A
    RET
`, "B,C,F,H,L"},
		{"data after the last RET", `
    LD A, 1
    RET
    DB 1, 2, 3
`, "A"},
	} {
		w, reason := exactWritesReason(u8, "\nleaf:\n"+strings.TrimPrefix(tc.body, "\n")+"\nnext:\n    RET\n")
		if reason != "" || writeList(w) != tc.want {
			t.Errorf("%s: writes %q reason %q, want %q", tc.name, writeList(w), reason, tc.want)
		}
	}
	// Any single return location; void and flag results included.
	for _, ret := range []*Return{nil, {Ty: TyU16, Class: ClassPointer}, {Ty: TyBool, Class: ClassFlag, FlagCond: CmpEq}} {
		m := &Module{}
		f := exactLeaf(m, "leaf", ret)
		if _, reason := exactWritesReason(f, "\nleaf:\n    LD HL, 1\n    XOR A\n    RET\n"); reason != "" {
			t.Errorf("return %+v rejected: %s", ret, reason)
		}
	}
}

func TestExactWritesRejectUnprovenBodies(t *testing.T) {
	m := &Module{}
	f := exactLeaf(m, "leaf", &Return{Ty: TyU8, Class: ClassAcc})
	for _, tc := range []struct{ name, body, reason string }{
		{"tail jump", "    LD A, 1\n    JP other\n", "jumps out"},
		{"conditional tail jump", "    AND A\n    JP Z, other\n    RET\n", "jumps out"},
		{"runtime call", "    CALL __mul8\n    RET\n", "calls"},
		{"RST", "    RST 0x10\n    RET\n", "calls"},
		{"indirect jump", "    JP (HL)\n", "jumps out"},
		{"port output", "    OUT (0xFE), A\n    RET\n", "port"},
		{"port input", "    IN A, (0xFE)\n    RET\n", "port"},
		{"HALT", "    HALT\n    RET\n", "interrupt"},
		{"DI", "    DI\n    RET\n", "interrupt"},
		{"SP replacement", "    LD SP, HL\n    RET\n", "SP"},
		{"POP of the return address", "    POP HL\n    JP (HL)\n", "POP below"},
		{"unbalanced PUSH", "    PUSH BC\n    RET\n", "RET with pushed"},
		{"unbalanced join", "    AND A\n    JR Z, .leaf_j\n    PUSH BC\n.leaf_j:\n    POP BC\n    RET\n", "unbalanced"},
		{"EX (SP) on the return address", "    EX (SP), HL\n    RET\n", "EX (SP)"},
		{"falls into the next function", "    LD A, 1\n", "falls off"},
		{"store into its own code", "    LD (.leaf_k+1), A\n.leaf_k:\n    LD A, 0\n    RET\n", "store into code"},
		{"I register", "    LD I, A\n    RET\n", "I/R"},
		{"unknown instruction", "    FOO A\n    RET\n", "unknown"},
		{"TSMC slot", "    LD (_tsmc_leaf_r1_0+1), A\n    RET\n", "TSMC"},
		{"patch slot EQU", "    LD HL, 0\nleaf$dst$imm  EQU $-2\n    RET\n", "falls off"},
	} {
		w, reason := exactWritesReason(f, "\nleaf:\n"+tc.body+"\nnext:\n    RET\n")
		if reason == "" || !strings.Contains(reason, tc.reason) {
			t.Errorf("%s: writes %q reason %q, want rejection %q", tc.name, writeList(w), reason, tc.reason)
		}
	}
	body := "\nleaf:\n    LD A, 1\n    RET\n"
	// The MIR side refuses what the body cannot show.
	m2 := &Module{}
	asm := exactLeaf(m2, "leaf", &Return{Ty: TyU8, Class: ClassAcc})
	asm.Entry().Insts = append([]*Inst{{Op: OpAsm}}, asm.Entry().Insts...)
	if _, reason := exactWritesReason(asm, body); reason != "asm" {
		t.Errorf("asm leaf: reason %q", reason)
	}
	m3 := &Module{}
	p := m3.AddFunc("leaf")
	b := NewBuilder(p)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassAcc)
	b.Ret(x)
	p.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	if _, reason := exactWritesReason(p, body); reason != "params" {
		t.Errorf("param leaf: reason %q", reason)
	}
	m4 := &Module{}
	c := exactLeaf(m4, "leaf", nil)
	c.Entry().Insts = append([]*Inst{{Op: OpCall, Sym: "other"}}, c.Entry().Insts...)
	if _, reason := exactWritesReason(c, body); reason != "calls" {
		t.Errorf("calling leaf: reason %q", reason)
	}
	// The #245 proof stays available for --no-exact-clobbers and is narrower.
	if _, ok := emittedLeafByteWrites(f, "\nleaf:\n    AND A\n    RET Z\n    LD A, 1\n    RET\n"); ok {
		t.Error("legacy proof accepted a conditional RET")
	}
}

// Z80Codegen re-proves every narrowed callee in the second emission and keeps
// the original product when that emission adds a write.
func TestExactClobbersFallBackWhenSecondEmissionAddsWrite(t *testing.T) {
	m := &Module{}
	exactLeaf(m, "leaf", &Return{Ty: TyU8, Class: ClassAcc})
	caller := m.AddFunc("caller")
	cb := NewBuilder(caller)
	cb.SwitchToNewBlock("entry")
	cb.CallVoid("leaf", nil, CallAttrs{})
	cb.Ret()
	const original = "\ncaller:\n    PUSH DE\n    CALL leaf\n    POP DE\n    RET\nleaf:\n    LD A, 1\n    RET\n"
	saved := z80CodegenEmit
	defer func() { z80CodegenEmit = saved }()
	for _, tc := range []struct {
		name, second string
		accept       bool
	}{
		{"same writes", "\ncaller:\n    CALL leaf\n    RET\nleaf:\n    LD A, 1\n    RET\n", true},
		{"adds a write", "\ncaller:\n    CALL leaf\n    RET\nleaf:\n    LD D, 1\n    LD A, 1\n    RET\n", false},
		{"loses the proof", "\ncaller:\n    CALL leaf\n    RET\nleaf:\n    LD A, 1\n    JP other\n", false},
	} {
		var caches []map[string]map[string]bool
		z80CodegenEmit = func(_ *Module, _ *AllocResult, cache map[string]map[string]bool, _ ...Z80CodegenOptions) (string, error) {
			caches = append(caches, cache)
			if cache == nil {
				return original, nil
			}
			return tc.second, nil
		}
		got, err := Z80Codegen(m, &AllocResult{Locs: map[Reg]PhysLoc{}})
		if err != nil {
			t.Fatal(err)
		}
		if len(caches) != 2 || writeList(caches[1]["leaf"]) != "A" {
			t.Fatalf("%s: emissions %v", tc.name, caches)
		}
		if (got == tc.second) != tc.accept || (got == original) == tc.accept {
			t.Errorf("%s: accepted=%v want %v", tc.name, got == tc.second, tc.accept)
		}
	}
	// --no-exact-clobbers keeps the #245 proof: a multi-block body is not narrowed.
	z80CodegenEmit = func(_ *Module, _ *AllocResult, cache map[string]map[string]bool, _ ...Z80CodegenOptions) (string, error) {
		if cache != nil {
			t.Fatal("legacy proof narrowed a multi-block leaf")
		}
		return "\nleaf:\n    AND A\n    JR Z, .leaf_z\n    LD A, 1\n.leaf_z:\n    RET\n", nil
	}
	if _, err := Z80Codegen(m, &AllocResult{Locs: map[Reg]PhysLoc{}}, Z80CodegenOptions{NoExactClobbers: true}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(exactClobberNames, "IXH") {
		t.Fatal("index halves must be tracked")
	}
}
