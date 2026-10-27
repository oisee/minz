package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Struct layout can be one byte, while every MIR value referring to it is
// an address. Direct constructor return is inspected before any later CALL;
// this specifically tests ABI/class validity, not returned-object lifetime.
func TestAggregateAddressClassesNative(t *testing.T) {
	const src = `struct S {v:u8}
fun make(x:u8)->S {var r:S=S{v:x}
if x==0 {r.v=0}else{r.v=x}
return r}
fun read(a:S)->u8{return a.v}
fun use_local(x:u8)->u8 {let a:S=S{v:x}
return a.v}`
	for _, opt := range []Options{{ContractOpt: true}, {ContractOpt: true, UseGrace: true}, {ContractOpt: true, UseGrace: true, OptSize: true}} {
		h, e := nanz.Parse(src, "aggregate-address.nanz")
		if e != nil {
			t.Fatal(e)
		}
		raw := hir.LowerModule(h)
		for _, f := range raw.Funcs {
			for _, p := range f.Contract.Params {
				if _, ok := p.Ty.(*mir2.StructTy); ok && p.Class != mir2.ClassPointer {
					t.Fatal("struct param class", p)
				}
			}
			for _, r := range f.Contract.Returns {
				if _, ok := r.Ty.(*mir2.StructTy); ok && r.Class != mir2.ClassPointer {
					t.Fatal("struct return class", r)
				}
			}
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if _, ok := i.Ty.(*mir2.StructTy); ok && i.Dst != mir2.NoReg && i.Cls != mir2.ClassPointer {
						t.Fatal("struct value class", i)
					}
				}
			}
		}
		h, e = nanz.Parse(src, "aggregate-address.nanz")
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, opt)
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		z := emulator.NewRemogattoZ80()
		if e = z.LoadMemory(0x8000, a.Binary); e != nil {
			t.Fatal(e)
		}
		for x := 0; x < 256; x++ {
			for _, name := range []string{"make", "read", "use_local"} {
				f := s.MIR2Module.FuncByName(name)
				r := emulator.Registers{PC: uint16(a.Symbols[name]), SP: 0xeff0}
				for pi, param := range f.Contract.Params {
					loc := assertParamLocation(param, s.Allocation)
					value := uint16(x)
					if pi > 0 {
						value = 0xc100
					}
					if pi == 0 && name == "read" {
						value = 0xc03d
						z.Memory()[value] = byte(x)
					}
					switch loc.Name {
					case "A":
						r.A = byte(value)
					case "C":
						r.BC = value
					case "HL":
						r.HL = value
					case "DE":
						r.DE = value
					default:
						if loc.Kind != mir2.LocMem {
							t.Fatal(loc)
						}
						slot := a.Symbols[mir2.Z80SpillLabel(f.Name, param.Reg)]
						z.Memory()[slot] = byte(value)
						if mir2.NativeSpillWidth(param.Ty) > 1 {
							z.Memory()[slot+1] = byte(value >> 8)
						}
					}
				}
				z.Memory()[0xeff0] = 0
				z.Memory()[0xeff1] = 0x70
				z.SetRegisters(r)
				done := false
				for n := 0; n < 1000; n++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					z.Step()
				}
				if !done {
					t.Fatal("native return", name, x)
				}
				got := z.GetRegisters().A
				if _, memoryResult := f.Contract.Returns[0].Ty.(*mir2.StructTy); name == "make" && memoryResult {
					got = z.Memory()[z.GetRegisters().HL]
				}
				if int(got) != x {
					t.Fatal("native aggregate address", name, x, got)
				}
			}
		}
	}
}
