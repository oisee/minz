package mir2

import (
	"encoding/binary"
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Constant rematerialisation (F6): an OpConst vregs whose every use the Z80
// codegen already serves from an immediate must not reserve a physical
// register for its whole live range.

// runConstRematProbe assembles a module, runs "probe" on the MIR2 VM and on the
// Z80 emulator, and returns the "out" word each backend produced.  Inputs and
// outputs travel through globals so neither backend depends on a calling
// convention.
func runConstRematProbe(t *testing.T, m *Module, asm string, in uint16) (uint16, uint16) {
	t.Helper()
	vm := NewVM(m)
	vm.WriteHeapSlice(vm.GlobalAddr("in"), []byte{byte(in), byte(in >> 8)})
	if _, err := vm.Call("probe", nil); err != nil {
		t.Fatal(err)
	}
	got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("out"), 2))

	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v\n%s", err, a.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	binary.LittleEndian.PutUint16(mem[a.Symbols["in"]:], in)
	binary.LittleEndian.PutUint16(mem[a.Symbols["out"]:], 0xbeef)
	binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
	z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["probe"]), SP: 0xeffe})
	for i := 0; i < 2000000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	regs := z.GetRegisters()
	if regs.PC != 0xf100 || regs.SP != 0xf000 {
		t.Fatalf("native run did not return: PC/SP %04x/%04x\n%s", regs.PC, regs.SP, asm)
	}
	return got, binary.LittleEndian.Uint16(mem[a.Symbols["out"]:])
}

// pairConstSetup matches a 0/1 constant materialised into a register pair
// (LD BC,0 / LD IX,1 / …) — the loop-invariant setup F6 removes.
var pairConstSetup = regexp.MustCompile(`(?m)^\s*LD (BC|DE|HL|IX|IY), [01]\s*$`)

// buildConstRematLoop builds `out = n + (n + (n-1) + … + 1)`.  zero and one are
// only ever used as immediates: cmp.ne i,zero in the header and sub i,one in
// the body, both of which the Z80 codegen folds (OR of the pair halves, DEC rr).
// The accumulator starts at the loaded count, so the two constants are the only
// word constants in the function.  When keepRegisterUse is set, the exit block
// additionally stores `one` to a global word — a use that needs the constant in
// a register.
func buildConstRematLoop(m *Module, keepRegisterUse bool) (zero, one Reg) {
	f := m.AddFunc("probe")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	head := b.SwitchToNewBlock("head")
	body := b.SwitchToNewBlock("body")
	exit := b.SwitchToNewBlock("exit")

	b.SwitchTo(entry)
	n := b.Load(b.AddrOf("in", ClassPointer), TyU16, ClassIndex)
	zero = b.Const(0, TyU16, ClassIndex)
	one = b.Const(1, TyU16, ClassIndex)
	b.Jmp("head", n, n)

	b.SwitchTo(head)
	i := b.BlockParam(head, TyU16, ClassIndex)
	a := b.BlockParam(head, TyU16, ClassPointer)
	cond := b.Cmp(CmpNe, i, zero, ClassFlag, false)
	b.BrIf(cond, "body", nil, "exit", []Reg{a})

	b.SwitchTo(body)
	sum := b.Add(a, i, TyU16, ClassPointer)
	next := b.Sub(i, one, TyU16, ClassIndex)
	b.Jmp("head", next, sum)

	b.SwitchTo(exit)
	r := b.BlockParam(exit, TyU16, ClassPointer)
	b.Store(b.AddrOf("out", ClassPointer), r, TyU16)
	if keepRegisterUse {
		b.Store(b.AddrOf("one_out", ClassPointer), one, TyU16)
	}
	b.Ret()
	return zero, one
}

func constRematModule(keepRegisterUse bool) (*Module, Reg, Reg) {
	m := &Module{Name: "constremat"}
	m.Globals = []Global{
		{Name: "in", Ty: TyU16, Init: make([]byte, 2)},
		{Name: "out", Ty: TyU16, Init: make([]byte, 2)},
	}
	if keepRegisterUse {
		m.Globals = append(m.Globals, Global{Name: "one_out", Ty: TyU16, Init: make([]byte, 2)})
	}
	zero, one := buildConstRematLoop(m, keepRegisterUse)
	return m, zero, one
}

func TestConstRematImmediateOnlyConstantGetsNoRegister(t *testing.T) {
	m, zero, one := constRematModule(false)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	f := m.Funcs[0]
	lr := ComputeLiveness(f)
	ar := PBQPAllocate(f, lr, Z80CostTable{})
	for r, name := range map[Reg]string{zero: "zero", one: "one"} {
		if loc := ar.Locs[r]; loc.Kind != LocRemat {
			t.Errorf("%s (immediate-only const) reserved %v; want rematerialisation", name, loc)
		}
	}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if setup := pairConstSetup.FindAllString(asm, -1); len(setup) != 0 {
		t.Errorf("constant materialised into a held pair: %v\n%s", setup, asm)
	}
	for _, push := range []string{"PUSH IX", "PUSH IY", "PUSH BC"} {
		if strings.Contains(asm, push) {
			t.Errorf("%s saves a reserved constant pair:\n%s", push, asm)
		}
	}
	for _, n := range []uint16{0, 1, 2, 5, 255, 256} {
		want := uint16(uint32(n) + uint32(n)*uint32(n+1)/2)
		vm, z80 := runConstRematProbe(t, m, asm, n)
		if vm != want || z80 != want {
			t.Fatalf("n=%d: VM %d, Z80 %d, want %d\n%s", n, vm, z80, want, asm)
		}
	}
}

// Negative control: the same constants, but `one` is also stored to a global
// word in the exit block.  That use reads the constant's home, so the
// allocation must keep a physical location and the code must stay correct.
func TestConstRematRegisterNeedingUseKeepsLocation(t *testing.T) {
	m, zero, one := constRematModule(true)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	f := m.Funcs[0]
	lr := ComputeLiveness(f)
	ar := PBQPAllocate(f, lr, Z80CostTable{})
	if loc := ar.Locs[one]; loc.Kind == LocRemat {
		t.Fatalf("const with a register-needing use was rematerialised: %v", loc)
	}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v\n%s", err, a.Errors, asm)
	}
	vm := NewVM(m)
	for _, n := range []uint16{0, 1, 4, 100} {
		vm.WriteHeapSlice(vm.GlobalAddr("in"), []byte{byte(n), byte(n >> 8)})
		vm.WriteHeapSlice(vm.GlobalAddr("one_out"), []byte{0xef, 0xbe})
		if _, err := vm.Call("probe", nil); err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("one_out"), 2)); got != 1 {
			t.Fatalf("n=%d: stored constant %d, want 1\n%s", n, got, asm)
		}
		want := uint16(uint32(n) + uint32(n)*uint32(n+1)/2)
		if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("out"), 2)); got != want {
			t.Fatalf("n=%d: sum %d, want %d\n%s", n, got, want, asm)
		}
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	binary.LittleEndian.PutUint16(mem[a.Symbols["in"]:], 9)
	binary.LittleEndian.PutUint16(mem[a.Symbols["one_out"]:], 0xbeef)
	binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
	z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["probe"]), SP: 0xeffe})
	for i := 0; i < 2000000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	if got := binary.LittleEndian.Uint16(mem[a.Symbols["one_out"]:]); got != 1 {
		t.Fatalf("native stored constant %d, want 1\n%s", got, asm)
	}
	if got := binary.LittleEndian.Uint16(mem[a.Symbols["out"]:]); got != 54 {
		t.Fatalf("native sum %d, want 54\n%s", got, asm)
	}
	_ = zero
}

// A constant used as a pointer base is read out of its home register; it must
// keep a physical location and the dereference must see the right memory.
func TestConstRematPointerBaseConstKeepsLocation(t *testing.T) {
	const src, dst = 0x6000, 0x6100
	m := &Module{Name: "constremat_ptr", Globals: []Global{
		{Name: "out", Ty: TyU16, Init: make([]byte, 2)},
	}}
	f := m.AddFunc("probe")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	base := b.Const(src, TyU16, ClassPointer)
	v := b.Load(base, TyU8, ClassGeneral)
	w := b.Load(b.Const(dst, TyU16, ClassPointer), TyU8, ClassGeneral)
	sum := b.Add(v, w, TyU8, ClassGeneral)
	ext := b.Ext(sum, TyU8, TyU16, ClassPointer)
	b.Store(b.AddrOf("out", ClassPointer), ext, TyU16)
	b.Ret()
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	lr := ComputeLiveness(f)
	ar := PBQPAllocate(f, lr, Z80CostTable{})
	for _, r := range []Reg{base} {
		if loc := ar.Locs[r]; loc.Kind == LocRemat {
			t.Errorf("pointer-base const rematerialised: %v", loc)
		}
	}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v\n%s", err, a.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	mem[src], mem[dst] = 0x2a, 0x11
	binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
	z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["probe"]), SP: 0xeffe})
	for i := 0; i < 100000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	if regs := z.GetRegisters(); regs.PC != 0xf100 || regs.SP != 0xf000 {
		t.Fatalf("native run did not return: PC/SP %04x/%04x\n%s", regs.PC, regs.SP, asm)
	}
	if got := binary.LittleEndian.Uint16(mem[a.Symbols["out"]:]); got != 0x3b {
		t.Fatalf("native sum %d, want 59\n%s", got, asm)
	}
}
