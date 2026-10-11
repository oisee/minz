package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructReturnLifetimeBufferedNative(t *testing.T) {
	data, e := os.ReadFile("../../../reports/struct-return-lifetime/source-call.nanz")
	if e != nil {
		t.Fatal(e)
	}
	text := string(data)
	save := func(mode, name string, v any) {
		root := os.Getenv("MINZ_STRUCT_BUFFER_ARTIFACT_DIR")
		if root == "" {
			return
		}
		dir := filepath.Join(root, "lifetime", mode)
		os.MkdirAll(dir, 0755)
		var b []byte
		switch x := v.(type) {
		case string:
			b = []byte(x)
		case []byte:
			b = x
		default:
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}

	source, e := nanz.Parse(text, "literal-width.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for mi, mode := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, UseGrace: true, AssertMode: "none"}, {ContractOpt: true, UseGrace: true, OptSize: true, AssertMode: "none"}} {
		m, e := nanz.Parse(text, "literal-width.nanz")
		if e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(m, mode)
		if e != nil {
			t.Fatal(e)
		}
		label := []string{"default", "grace-speed", "grace-size"}[mi]
		save(label, "source-hir.json", source)
		save(label, "raw.mir", hir.LowerModule(source).Dump())
		save(label, "final.mir", steps.MIR2Module.Dump())
		save(label, "target.a80", steps.Assembly)
		save(label, "allocation.json", steps.Allocation)
		program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(program.Errors) > 0 {
			t.Fatal(e, program.Errors)
		}
		save(label, "target.bin", program.Binary)
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, program.Binary)
		var results []map[string]any
		for _, pair := range [][2]uint16{{0, 1}, {1, 2}, {17, 83}, {128, 255}, {255, 0}} {
			x, y := pair[0], pair[1]
			for _, c := range []struct {
				name string
				want uint16
			}{{"delayed", x}, {"two_returns", uint16(byte(x + y))}} {

				args := []mir2.Value{{I: int64(x)}, {I: int64(y)}}
				for _, mod := range []*mir2.Module{hir.LowerModule(source), steps.MIR2Module} {
					v, e := mir2.NewVM(mod).Call(c.name, args)
					if e != nil || len(v) != 1 || v[0].I != int64(c.want) {
						t.Fatal("literal width VM", c, x, v, e)
					}
				}
				f := steps.MIR2Module.FuncByName(c.name)
				regs := emulator.Registers{PC: uint16(program.Symbols[c.name]), SP: 0xeff0}
				for pi, param := range f.Contract.Params {
					arg := []uint16{x, y}[pi]
					loc := assertParamLocation(param, steps.Allocation)
					switch loc.Name {
					case "A":
						regs.A = byte(arg)
					case "C":
						regs.BC = regs.BC&0xff00 | arg
					case "D":
						regs.DE = regs.DE&255 | arg<<8
					case "E":
						regs.DE = regs.DE&0xff00 | arg
					case "B":
						regs.BC = regs.BC&255 | arg<<8
					default:
						if loc.Kind != mir2.LocMem {
							t.Fatal(loc)
						}
						slot := program.Symbols[mir2.Z80SpillLabel(f.Name, param.Reg)]
						z.Memory()[slot] = byte(arg)
						if mir2.NativeSpillWidth(param.Ty) > 1 {
							z.Memory()[slot+1] = byte(arg >> 8)
						}
					}
				}

				z.Memory()[0xeff0] = 0
				z.Memory()[0xeff1] = 0x70
				z.SetRegisters(regs)
				done := false
				for n := 0; n < 10000; n++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					z.Step()
				}
				if !done {
					t.Fatal("native frame", c, x)
				}
				got := z.GetRegisters().A
				result := map[string]any{"entry": c.name, "x": x, "y": y, "want": c.want, "native": got, "returned_sp": z.GetSP()}
				results = append(results, result)
				if int64(got) != int64(c.want) {
					t.Fatalf("LIFETIME FAILURE %s %v", label, result)
				}

			}
		}
		save(label, "results.json", results)
	}
}

func TestStructReturnNestedAndRecursiveNative(t *testing.T) {
	const source = `struct Inner {a:u8,b:u16}
struct Outer {pad:u8,inner:Inner,tail:u8}
global scratch:u8=0
fun build(x:u8)->Outer {var r:Outer=Outer{pad:7,inner:Inner{a:x,b:u16(x)+256},tail:9}
if x==0 {r.pad=8}else{r.pad=7}
return r}
fun disturb(y:u8)->u8 {let q:Outer=Outer{pad:y,inner:Inner{a:y,b:u16(y)+512},tail:y}
scratch=q.inner.a
return q.pad}
fun nested(x:u8,y:u8)->u16 {let a:Outer=build(x)
let q:u8=disturb(y)
return a.inner.b+u16(a.inner.a)}
fun recurse(n:u8)->Inner {var keep:Inner=Inner{a:n,b:u16(n)+256}
if n>0 {let child:Inner=recurse(n-1)
keep.b=keep.b+child.b}
return keep}
fun recursive(n:u8,y:u8)->u16 {let a:Inner=recurse(n)
let q:u8=disturb(y)
return a.b+u16(a.a)}
fun indirect(f:fun(u8)->Outer,x:u8,y:u8)->u16 {let a:Outer=f(x)
let q:u8=disturb(83)
return a.inner.b+u16(a.inner.a)}
fun invoke(x:u8,y:u8)->u16{return indirect(build,x,y)}`
	for modeIndex, opt := range []Options{{ContractOpt: true}, {ContractOpt: true, UseGrace: true}, {ContractOpt: true, UseGrace: true, OptSize: true}} {
		h, e := nanz.Parse(source, "nested-result.nanz")
		if e != nil {
			t.Fatal(e)
		}
		raw := hir.LowerModule(h)
		h, e = nanz.Parse(source, "nested-result.nanz")
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
		if root := os.Getenv("MINZ_STRUCT_BUFFER_ARTIFACT_DIR"); root != "" {
			dir := filepath.Join(root, "nested", []string{"default", "grace-speed", "grace-size"}[modeIndex])
			if e := os.MkdirAll(dir, 0755); e != nil {
				t.Fatal(e)
			}
			for name, payload := range map[string][]byte{"source.nanz": []byte(source), "raw.mir": []byte(raw.Dump()), "final.mir": []byte(s.MIR2Module.Dump()), "target.a80": []byte(s.Assembly), "target.bin": a.Binary} {
				if e := os.WriteFile(filepath.Join(dir, name), payload, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
		mutable := map[int]bool{}
		scratchAddress, found := a.Symbols["scratch"]
		if !found {
			t.Fatal("missing declared scratch global")
		}
		mutable[scratchAddress] = true
		for _, l := range a.Listing {
			if strings.HasPrefix(l.Label, "_spill_") || l.Label == "scratch" {
				for j := range l.Bytes {
					mutable[l.Address+j] = true
				}
			}
		}
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, a.Binary)
		for _, x := range []uint16{0, 1, 2, 3, 7, 15, 127, 128, 255} {
			for _, name := range []string{"nested", "invoke", "recursive"} {
				if e = z.LoadMemory(0x8000, a.Binary); e != nil {
					t.Fatal(e)
				}
				if name == "recursive" && x > 7 {
					continue
				}
				want := 2*x + 256
				if name == "recursive" {
					want = uint16((int(x)+1)*256 + int(x)*(int(x)+1)/2 + int(x))
				}
				for _, m := range []*mir2.Module{raw, s.MIR2Module} {
					v, e := mir2.NewVM(m).Call(name, []mir2.Value{{I: int64(x)}, {I: 83}})
					if e != nil || len(v) != 1 || v[0].I != int64(want) {
						t.Fatal("VM nested", name, x, v, e, want)
					}
				}
				f := s.MIR2Module.FuncByName(name)
				r := emulator.Registers{PC: uint16(a.Symbols[name]), SP: 0xeff0}
				for pi, p := range f.Contract.Params {
					val := []uint16{x, 83}[pi]
					loc := assertParamLocation(p, s.Allocation)
					switch loc.Name {
					case "A":
						r.A = byte(val)
					case "C":
						r.BC = r.BC&0xff00 | val
					case "D":
						r.DE = r.DE&255 | val<<8
					case "E":
						r.DE = r.DE&0xff00 | val
					case "B":
						r.BC = r.BC&255 | val<<8
					default:
						if loc.Kind != mir2.LocMem {
							t.Fatal(loc)
						}
						slot := a.Symbols[mir2.Z80SpillLabel(f.Name, p.Reg)]
						z.Memory()[slot] = byte(val)
					}
				}
				z.Memory()[0xeff0] = 0
				z.Memory()[0xeff1] = 0x70
				z.SetRegisters(r)
				done := false
				before := append([]byte(nil), z.Memory()...)
				minSP := uint16(0xeff0)
				for n := 0; n < 100000; n++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("stack bounds", name, x)
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}

				}
				if !done {
					t.Fatalf("native nested return %s %d pc%x SP%x regs%+v", name, x, z.GetPC(), z.GetSP(), z.GetRegisters())
				}
				if z.Memory()[scratchAddress] != 83 {
					t.Fatalf("native scratch %s x%d: got %d want 83", name, x, z.Memory()[scratchAddress])
				}
				copy(before[int(minSP):0xeff0], z.Memory()[int(minSP):0xeff0])
				for p := range mutable {
					before[p] = z.Memory()[p]
				}
				if !bytes.Equal(before, z.Memory()) {
					for p, v := range before {
						if z.Memory()[p] != v {
							t.Fatalf("unexpected native memory %s x%d addr%x old%x new%x", name, x, p, v, z.Memory()[p])
						}
					}
				}

				if e = checkAssertZ80Result(z, hir.Assert{FuncName: name, Expected: int64(want)}, f, map[string]*hir.Func{name: h.FuncByName(name)}); e != nil {
					t.Fatal("native nested", name, x, e)
				}
			}
		}
	}
}
