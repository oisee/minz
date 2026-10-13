package mir2

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func runWideMemoryCode(t *testing.T, asm string, value uint32, address uint16) []byte {
	t.Helper()
	res, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, res.Binary); err != nil {
		t.Fatal(err)
	}
	mem := z.Memory()
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, value)
	for i, v := range data {
		mem[uint16(uint32(address)+uint32(i))] = v
	}
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
	for i := 0; !z.IsHalted(); i++ {
		if i > 20000 {
			t.Fatal("did not halt")
		}
		z.Step()
	}
	if z.GetSP() != 0xfff0 {
		t.Fatalf("unbalanced SP=%04x", z.GetSP())
	}
	// Read both register banks without depending on emulator shadow API.
	if dir := os.Getenv("MINZ_U32_ARTIFACTS"); dir != "" && (strings.Contains(t.Name(), "ffff/u32/HL/spill/12345678") || strings.Contains(t.Name(), "CallerSpillSnapshot/") && strings.HasSuffix(t.Name(), "ffffffff")) {
		os.MkdirAll(dir, 0755)
		name := strings.ReplaceAll(t.Name(), "/", "-")
		os.WriteFile(filepath.Join(dir, name+".a80"), []byte(asm), 0644)
		os.WriteFile(filepath.Join(dir, name+".bin"), res.Binary, 0644)
	}
	return append([]byte(nil), mem[0x6100:0x6104]...)
}

func TestZ80WideMemoryPointerBanksAndSpills(t *testing.T) {
	for _, address := range []uint16{0x6000, 0xffff} {
		for _, ty := range []Ty{TyU24, TyU32} {
			for _, ptr := range []string{"HL", "DE", "BC", "IX", "IY", "spill"} {
				for _, home := range []string{"HL", "DE", "BC", "spill"} {
					for _, value := range []uint32{0, 1, 0xffff, 0x10000, 0x12345678, 0x80000000, 0xffffffff} {
						t.Run(fmt.Sprintf("%04x/%s/%s/%s/%08x", address, ty, ptr, home, value), func(t *testing.T) {
							var out strings.Builder
							ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: ptr}, 2: {Kind: LocDWord, Name: home}}}
							if ptr == "spill" {
								ar.Locs[1] = PhysLoc{Kind: LocMem}
							}
							if home == "spill" {
								ar.Locs[2] = PhysLoc{Kind: LocMem}
							}
							f := &Func{Name: "memory"}
							g := &z80cg{sb: &out, ar: ar, fn: f, physOverride: map[Reg]string{}, regInfo: map[Reg]RegInfo{1: {Ty: TyPtr}, 2: {Ty: ty}}}
							g.emit("ORG 0x8000")
							// Deliberately unrelated shadow pointers expose incorrect EXX addressing.
							g.emit("EXX")
							g.emit("LD HL,0x6200")
							g.emit("LD DE,0x6300")
							g.emit("LD BC,0x6400")
							g.emit("EXX")
							if ptr == "spill" {
								g.emitf("LD HL,%d", address)
								g.emit("LD (_spill_memory_r1),HL")
							} else {
								g.emitf("LD %s,%d", ptr, address)
							}
							g.genWideMemory(&Inst{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: ty})
							g.emit("LD IY,0x6100")
							ar.Locs[3] = PhysLoc{Kind: LocReg, Name: "IY"}
							storePtr := Reg(3)
							g.genWideMemory(&Inst{Op: OpStore, Src: [2]Reg{storePtr, 2}, Ty: ty})
							g.emit("DI")
							g.emit("HALT")
							g.emit("_spill_memory_r1: DW 0")
							g.emit("_spill_memory_r2: DB 0,0,0,0")
							if g.err != nil {
								t.Fatal(g.err)
							}
							got := binary.LittleEndian.Uint32(runWideMemoryCode(t, out.String(), value, address))
							want := value
							if ty.Width() == 24 {
								want &= 0xffffff
							}
							// Independent VM load/store oracle uses raw memory values rather than
							// the native bank representation.
							m := &Module{}
							fn := m.AddFunc("copy")
							b := NewBuilder(fn)
							b.SwitchToNewBlock("entry")
							p := b.Const(int64(address), TyPtr, ClassPointer)
							v := b.Load(p, ty, ClassDWord)
							q := b.Const(0x6100, TyPtr, ClassPointer)
							b.Store(q, v, ty)
							b.Ret()
							vm := NewVM(m)
							vm.EnsureHeap(65536)
							data := make([]byte, 4)
							binary.LittleEndian.PutUint32(data, value)
							for i, v := range data {
								vm.WriteHeapSlice(int64(uint16(uint32(address)+uint32(i))), []byte{v})
							}
							if _, err := vm.Call("copy", nil); err != nil {
								t.Fatal(err)
							}
							if vg := binary.LittleEndian.Uint32(vm.ReadHeap(0x6100, 4)); vg != want {
								t.Fatalf("VM=%08x want=%08x", vg, want)
							}
							if got != want {
								t.Fatalf("got=%08x want=%08x\n%s", got, want, out.String())
							}
						})
					}
				}
			}
		}
	}

	t.Log("672 independent VM+Z80 memory checks PASS (including 0xffff wrap); native SP=fff0")
}
func TestZ80U32CallerSpillSnapshot(t *testing.T) {
	for _, home := range []string{"spill", "HL", "DE", "BC"} {
		for _, value := range []uint32{0, 1, 0xffff, 0x10000, 0x7fffffff, 0x80000000, 0xfffffffe, 0xffffffff} {
			t.Run(fmt.Sprintf("%s/%08x", home, value), func(t *testing.T) {
				m := &Module{Name: "snapshots"}
				f := m.AddFunc("wide")
				f.Contract.Returns = []Return{{Ty: TyU32, Class: ClassDWord}}
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				live := b.Const(int64(value), TyU32, ClassDWord)
				result := b.Call("smash", nil, TyU32, ClassDWord, CallAttrs{})
				out := b.Add(live, result, TyU32, ClassDWord)
				b.Ret(out)
				callee := m.AddFunc("smash")
				callee.Attrs.IsExtern = true
				callee.Contract.Returns = []Return{{Ty: TyU32, Class: ClassDWord}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{live: {Kind: LocMem}, result: {Kind: LocMem}, out: {Kind: LocDWord, Name: "HL"}}}
				if home != "spill" {
					ar.Locs[live] = PhysLoc{Kind: LocDWord, Name: home}
				}
				code, err := Z80Codegen(m, ar)
				if err != nil {
					t.Fatal(err)
				}
				// Clobber both words of the caller home deliberately. It models shared
				// absolute homes in recursive activations, without changing semantic input.
				symbol := Z80SpillLabel("wide", live)
				clobber := fmt.Sprintf("LD HL,0xdead\nLD (%s),HL\nLD HL,0xbeef\nLD (%s+2),HL\n", symbol, symbol)
				if home != "spill" {
					clobber = "LD HL,0xdead\nLD DE,0xdead\nLD BC,0xdead\nEXX\nLD HL,0xbeef\nLD DE,0xbeef\nLD BC,0xbeef\nEXX\n"
				}
				asm := "ORG 0x8000\nLD SP,0xfff0\nCALL wide\nLD (0x6100),HL\nEXX\nLD (0x6102),HL\nEXX\nDI\nHALT\n" + code +
					"\n" + clobber + "LD HL,3\nEXX\nLD HL,1\nEXX\nRET\n"
				got := binary.LittleEndian.Uint32(runWideMemoryCode(t, asm, 0, 0x6000))
				want := value + uint32(0x10003)
				if got != want {
					t.Fatalf("got=%08x want=%08x", got, want)
				}
				// VM executes the same MIR function with a semantic callee returning 0x10003.
				callee.Attrs.IsExtern = false
				c := NewBuilder(callee)
				c.SwitchToNewBlock("entry")
				r := c.Const(0x10003, TyU32, ClassDWord)
				c.Ret(r)
				vm, err := NewVM(m).Call("wide", nil)
				if err != nil || len(vm) != 1 || uint32(vm[0].I) != want {
					t.Fatalf("VM %v %v want=%08x", vm, err, want)
				}
			})
		}
	}

	t.Log("32 modulo-2^32 VM+Z80 caller checks PASS: spill/HL/DE/BC; both banks clobbered; native SP=fff0")
}
