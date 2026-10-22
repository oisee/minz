package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"strings"
	"testing"
)

func TestLeafByteCallPreservesUnwrittenByteVMZ80(t *testing.T) {
	m := &Module{}
	leaf := m.AddFunc("leaf")
	b := NewBuilder(leaf)
	b.SwitchToNewBlock("entry")
	x := b.Const(0x34, TyU8, ClassGeneral)
	one := b.Const(1, TyU8, ClassGeneral)
	next := b.BinOp(OpAdd, x, one, TyU8, ClassGeneral)
	mask := b.Const(0x55, TyU8, ClassGeneral)
	value := b.BinOp(OpXor, next, mask, TyU8, ClassAcc)
	b.Ret(value)
	leaf.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	caller := m.AddFunc("caller")
	c := NewBuilder(caller)
	c.SwitchToNewBlock("entry")
	keep := c.Param("keep", TyU8, ClassCounter)
	hi := c.Call("leaf", nil, TyU8, ClassAcc, CallAttrs{})
	combined := c.BinOp(OpXor, keep, hi, TyU8, ClassAcc)
	c.Ret(combined)
	caller.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Name: "C"}, one: {Name: "E"}, next: {Name: "D"}, mask: {Name: "E"}, value: {Name: "A"}, keep: {Name: "B"}, hi: {Name: "A"}, combined: {Name: "A"}}}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asm, "PUSH BC") {
		t.Fatalf("leaf does not write B; unnecessary caller save:\n%s", asm)
	}
	vm := NewVM(m)
	for low := 0; low < 256; low++ {
		want := byte(0x60 ^ low)
		values, e := vm.Call("caller", []Value{{I: int64(low)}})
		got := runRAFragment(t, "JP caller\n"+asm, emulator.Registers{BC: uint16(low) << 8})
		if e != nil || values[0].I != int64(want) || got.A != want {
			t.Fatal(low, values, e, got, want)
		}
	}
}

func TestEmittedLeafByteWriteProofRejectsUnmodelledEffects(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("leaf")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	v := b.Const(7, TyU8, ClassAcc)
	b.Ret(v)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	for _, code := range []string{"LD A,7\nRET", "INC B\nLD A,B\nRET", "PUSH BC\nPOP BC\nRET", "LD (1234),A\nRET", "JR NZ,elsewhere\nRET", "CALL other\nRET", "LD I,A\nRET", "EXX\nRET", "RET Z\nRET"} {
		writes, ok := emittedLeafByteWrites(f, "\nleaf:\n"+code+"\n")
		want := code == "LD A,7\nRET" || code == "INC B\nLD A,B\nRET"
		if ok != want {
			t.Fatalf("%q accepted=%v want%v", code, ok, want)
		}
		if want && (!writes["A"] || writes["B"] != (code == "INC B\nLD A,B\nRET")) {
			t.Fatal(code, writes)
		}
	}
}
