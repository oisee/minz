package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestWordMaskLanesNativeAllWords(t *testing.T) {
	for _, mask := range []int64{0, 255, 65280, 4095, 61695, 65535} {
		for _, pair := range []string{"HL", "DE", "BC"} {
			m := &Module{Globals: []Global{{Name: "saved_byte", Ty: TyU8}}}
			f := m.AddFunc("mask")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			word := b.Param("word", TyU16, ClassPair)
			live := b.Param("live", TyU8, ClassAcc)
			k := b.Const(mask, TyU16, ClassPair)
			r := b.And(word, k, TyU16, ClassPair)
			addr := b.AddrOf("saved_byte", ClassPointer)
			b.Store(addr, live, TyU8)
			b.Ret(r)
			f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{word: {Kind: LocReg, Name: pair}, live: {Kind: LocReg, Name: "A"}, k: {Kind: LocReg, Name: "IX"}, r: {Kind: LocReg, Name: pair}, addr: {Kind: LocReg, Name: "IY"}}}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(asm, "AND 255") || strings.Contains(asm, "AND 0\n") {
				t.Fatal("redundant mask lane", asm)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
			if err != nil || len(a.Errors) != 0 {
				t.Fatal(err, a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			for x := 0; x < 65536; x++ {
				reg := emulator.Registers{A: byte(x ^ 0xa5), F: byte(x >> 8), PC: uint16(a.Symbols["mask"]), SP: 0xeffe}
				switch pair {
				case "HL":
					reg.HL = uint16(x)
				case "DE":
					reg.DE = uint16(x)
				case "BC":
					reg.BC = uint16(x)
				}
				mem[0xeffe], mem[0xefff] = 0, 0xf1
				mem[a.Symbols["saved_byte"]] = 0
				z.SetRegisters(reg)
				for n := 0; n < 100 && z.GetPC() != 0xf100; n++ {
					z.Step()
				}
				got := z.GetRegisters()
				if got.HL != uint16(x&int(mask)) || mem[a.Symbols["saved_byte"]] != reg.A || got.PC != 0xf100 || got.SP != 0xf000 {
					t.Fatal(mask, pair, x, got, reg, asm)
				}
			}
		}
	}
}

func TestWordMaskLaneFlagRefusals(t *testing.T) {
	for _, mode := range []string{"asm", "attribute-asm", "flag-param", "flag-return", "carry-cmp", "fallible", "direct-branch", "forwarded-branch", "conditional-call"} {
		t.Run(mode, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("test")
			b := NewBuilder(f)
			entry := b.SwitchToNewBlock("entry")
			x := b.Param("x", TyU16, ClassPair)
			k := b.Const(255, TyU16, ClassPair)
			r := b.And(x, k, TyU16, ClassPair)
			b.Ret(r)
			switch mode {
			case "asm":
				entry.Insts = append(entry.Insts, &Inst{Op: OpAsm})
			case "attribute-asm":
				f.Attrs.HasAsm = true
			case "flag-param":
				f.Contract.Params[0].Class = ClassFlag
			case "flag-return":
				f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassFlag}}
			case "carry-cmp":
				entry.Insts = append(entry.Insts, &Inst{Op: OpCmp, Cond: CmpSubCarry, Dst: f.AllocReg(), Ty: TyBool})
			case "fallible":
				f.Attrs.FallibleCY = true
			case "direct-branch":
				entry.Term = &TermBrIf{Cond: r, Then: "entry", Else: "entry"}
			case "forwarded-branch":
				dst := f.NewBlock("dst")
				v := b.BlockParam(dst, TyU16, ClassGeneral)
				entry.Term = &TermJmp{Target: "dst", Args: []Reg{r}}
				dst.Term = &TermCondRet{Cond: v, Then: "entry"}
			case "conditional-call":
				entry.Insts = append(entry.Insts, &Inst{Op: OpCallCond})
			}
			if wordMaskFlagsUnobserved(f, r) {
				t.Fatal("unsafe flag elision admitted")
			}
		})
	}
	// Values compared explicitly are safe: their CMP emits fresh flags.
	m := &Module{}
	f := m.AddFunc("explicit")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU16, ClassPair)
	k := b.Const(255, TyU16, ClassPair)
	r := b.And(x, k, TyU16, ClassPair)
	zero := b.Const(0, TyU16, ClassPair)
	cmp := b.CmpWithSrcTy(CmpNe, r, zero, ClassFlag, false, TyU16)
	b.Ret(cmp)
	if !wordMaskFlagsUnobserved(f, r) {
		t.Fatal("explicit value comparison refused")
	}
}
