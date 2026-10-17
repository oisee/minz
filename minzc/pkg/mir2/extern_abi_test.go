package mir2

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"reflect"
	"strings"
	"testing"
)

func TestZ80ExternalParameterStorage(t *testing.T) {
	cases := []struct {
		name  string
		types []Ty
		want  []string
	}{{"two-pointers", []Ty{TyPtr, TyPtr}, []string{"HL", "DE"}}, {"five-words", []Ty{TyU16, TyU16, TyU16, TyU16, TyU16}, []string{"HL", "DE", "BC", "IX", "IY"}}, {"mixed", []Ty{TyU8, TyPtr, TyU8, TyU16, TyU8}, []string{"A", "HL", "B", "DE", "C"}}, {"overflow", []Ty{TyU16, TyU16, TyU16, TyU16, TyU16, TyU16}, []string{"HL", "DE", "BC", "IX", "IY", "_spill_host_r6"}}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &Func{Name: "host", Attrs: FuncAttrs{IsExtern: true}}
			for n, ty := range c.types {
				f.Contract.Params = append(f.Contract.Params, Param{Reg: Reg(n + 1), Ty: ty, Class: standardParamClass(ty, n)})
			}
			locs, e := Z80ExternParamLocations(f)
			if e != nil {
				t.Fatal(e)
			}
			for n, w := range c.want {
				if locs[Reg(n+1)].Name != w {
					t.Fatal(n, locs, w)
				}
			}
			ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
			if !reflect.DeepEqual(ar.Locs, locs) {
				t.Fatal("public allocation")
			}
		})
	}
	f := &Func{Name: "pins", Attrs: FuncAttrs{IsExtern: true}, Contract: Contract{Params: []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}, {Reg: 2, Ty: TyU8, Class: ClassRegH}}}}
	locs, e := Z80ExternParamLocations(f)
	if e != nil || locs[1].Name != "DE" || locs[2].Name != "H" {
		t.Fatal(locs, e)
	}
	f.Contract.Params[0].ClassHard = true
	if _, e = Z80ExternParamLocations(f); e == nil {
		t.Fatal("overlap explicitHL/H")
	}
	b, _ := json.Marshal(f.Contract.Params[0])
	var clone map[string]any
	if json.Unmarshal(b, &clone) != nil || clone["ClassHard"] != true {
		t.Fatal("ClassHard serialization")
	}
	if !strings.Contains((&Module{Funcs: []*Func{f}}).Dump(), "[pointer]!") {
		t.Fatal("ClassHard dump")
	}
}
func TestZ80ExternalFiveWordsAndSpillNative(t *testing.T) {
	for _, count := range []int{2, 5, 6} {
		m := &Module{}
		host := m.AddFunc("host")
		host.Attrs.IsExtern = true
		hb := NewBuilder(host)
		for n := 0; n < count; n++ {
			hb.Param("arg", TyU16, standardParamClass(TyU16, n))
		}
		main := m.AddFunc("main")
		b := NewBuilder(main)
		b.SwitchTo(main.NewBlock("entry"))
		args := []Reg{}
		for n := 0; n < count; n++ {
			args = append(args, b.Const(int64(0x1234+n*257), TyU16, ClassPointer))
		}
		b.Call("host", args, TyVoid, ClassGeneral, CallAttrs{})
		b.Ret()
		combined := &AllocResult{Locs: map[Reg]PhysLoc{}}
		for _, f := range m.Funcs {
			ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
			for r, l := range ar.Locs {
				combined.Locs[r] = l
			}
			combined.Spilled = append(combined.Spilled, ar.Spilled...)
		}
		// The test-owned loader CALLs main and checks its return address/SP.
		text, e := Z80Codegen(m, combined, Z80CodegenOptions{CalledMain: true})
		if e != nil {
			t.Fatal(e)
		} // test-owned RET at the external label, immutable emitted caller
		text = strings.Replace(text, "host:", "host:\n RET", 1)
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		z.LoadMemory(0x8000, a.Binary)
		z.LoadMemory(0x7000, []byte{0xcd, byte(a.Symbols["main"]), byte(a.Symbols["main"] >> 8), 0})
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
		seen := false
		for n := 0; n < 10000 && z.GetPC() != 0x7003; n++ {
			if int(z.GetPC()) == a.Symbols["host"] {
				seen = true
				r := z.GetRegisters()
				for j, p := range host.Contract.Params {
					l := combined.Locs[p.Reg]
					var v uint16
					switch l.Name {
					case "HL":
						v = r.HL
					case "DE":
						v = r.DE
					case "BC":
						v = r.BC
					case "IX":
						v = r.IX
					case "IY":
						v = r.IY
					default:
						addr := a.Symbols[l.Name]
						v = uint16(z.Memory()[addr]) | uint16(z.Memory()[addr+1])<<8
					}
					if v != uint16(0x1234+j*257) {
						t.Fatal(count, j, l, v)
					}
				}
			}
			z.Step()
		}
		if !seen || z.GetPC() != 0x7003 || z.GetSP() != 0xf000 {
			t.Fatal("return/SP", count, z.GetRegisters())
		}
	}
}
