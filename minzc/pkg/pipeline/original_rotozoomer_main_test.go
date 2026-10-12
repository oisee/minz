package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// The expected framebuffer is computed independently with explicit signed16
// stored intermediates (Book v7 section 3.2), never copied from VM execution.
func TestOriginalRotozoomerWholeMain(t *testing.T) {
	const packet = "../../../reports/rotozoomer-main/frozen/"
	read := func(n string) []byte {
		b, e := os.ReadFile(packet + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	src := read("source.nanz")
	current, e := os.ReadFile("../../../examples/nanz/rotozoomer.nanz")
	if e != nil || !bytes.Equal(src, current) {
		t.Fatal("source changed", e)
	}
	var f struct {
		SHA     string  `json:"source_sha256"`
		A       []int   `json:"axis_a"`
		B       []int   `json:"axis_b"`
		Palette [][]int `json:"palette"`
	}
	if e := json.Unmarshal(read("freeze.json"), &f); e != nil || f.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || len(f.A) != 32 || len(f.B) != 32 || len(f.Palette) != 6 {
		t.Fatal("freeze", e)
	}
	save := func(n string, v any) {
		if d := os.Getenv("MINZ_ROTO_MAIN_EXPORT"); d != "" {
			var b []byte
			switch x := v.(type) {
			case []byte:
				b = x
			case string:
				b = []byte(x)
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			p := filepath.Join(d, n)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	abs := func(x int16) int16 {
		if x < 0 {
			return -x
		}
		return x
	}
	texture := func(x, y int16) byte {
		d := abs(x-8) + abs(y-8)
		if d < 3 {
			return 5
		}
		if d < 6 {
			return 4
		}
		if (x/4+y/4)%2 == 0 {
			return 3
		}
		if (x^y)&2 == 0 {
			return 2
		}
		return 1
	}
	frames := make([][]byte, 32)
	for i := range frames {
		pixels := make([]byte, 256*192)
		a, b := int16(f.A[i]), int16(f.B[i])
		ur, vr := int16(18432)-a*128-b*96, int16(18432)+b*128-a*96
		for y := 0; y < 192; y++ {
			u, v := ur, vr
			for x := 0; x < 256; x++ {
				pixels[y*256+x] = texture((u/256)%16, (v/256)%16)
				u += a
				v -= b
			}
			ur += b
			vr += a
		}
		frames[i] = pixels
		save(fmt.Sprintf("expected/frame-%02d.bin", i), pixels)
	}
	// One ordered checker handles the exact 32 * (init + 6 palettes + 49152
	// pixels + save) source calls. It neither generates nor executes target IR.
	checker := func(prefix string, readCString func(int64) string) (func(string, []int64) int64, func()) {
		frame, stage, pixel := 0, 0, 0
		host := func(name string, args []int64) int64 {
			if frame >= 32 {
				t.Fatal("extra frame", name)
			}
			equal := func(w []int64) {
				if len(args) != len(w) {
					t.Fatal("host arity", name, args)
				}
				for i := range w {
					if args[i] != w[i] {
						t.Fatalf("%s frame%d stage%d pixel%d %s arg%d got%d want%d", prefix, frame, stage, pixel, name, i, args[i], w[i])
					}
				}
			}
			switch {
			case stage == 0:
				if name != "canvas_init" {
					t.Fatal("init order", name)
				}
				equal([]int64{256, 192, 0})
				stage++
			case stage <= 6:
				if name != "canvas_palette" {
					t.Fatal("palette order", name)
				}
				i := stage - 1
				equal([]int64{int64(i), int64(f.Palette[i][0]), int64(f.Palette[i][1]), int64(f.Palette[i][2])})
				stage++
			case pixel < 256*192:
				if name != "canvas_pixel" {
					t.Fatal("pixel order", name, pixel)
				}
				equal([]int64{int64(pixel % 256), int64(pixel / 256), int64(frames[frame][pixel])})
				pixel++
			default:
				if name != "canvas_save" || len(args) != 1 {
					t.Fatal("save order", name, args)
				}
				if path := readCString(args[0]); path != fmt.Sprintf("build/rotozoom-%d.png", frame) {
					t.Fatal("save path", path, frame)
				}
				frame++
				stage = 0
				pixel = 0
			}
			return 0
		}
		check := func() {
			if frame != 32 || stage != 0 || pixel != 0 {
				t.Fatal("incomplete main", prefix, frame, stage, pixel)
			}
			save(prefix+"/events-summary.json", map[string]any{"frames": 32, "pixel_calls": 32 * 256 * 192, "calls": 32 * (256*192 + 8), "result": "PASS"})
		}
		return host, check
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "rotozoomer.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			raw := hir.LowerModule(parse())
			s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			save(mode+"/raw.mir", raw.Dump())
			save(mode+"/final.mir", s.MIR2Module.Dump())
			save(mode+"/allocation.json", s.Allocation)
			for _, engine := range []struct {
				name string
				m    *mir2.Module
			}{{"raw", raw}, {"final", s.MIR2Module}} {
				vm := mir2.NewVM(engine.m)
				vm.MaxSteps = 1000000000
				allowed := map[int64]bool{}
				for n := 0; n < engine.m.Strings.Len(); n++ {
					p, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", n))
					if e != nil {
						t.Fatal(e)
					}
					for j := 0; j <= len(engine.m.Strings.At(n)); j++ {
						allowed[p+int64(j)] = true
					}
				}
				vm.EnsureHeap(65536)
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				host, check := checker(mode+"/"+engine.name, func(p int64) string {
					var b []byte
					for j := int64(0); j < 128; j++ {
						if !allowed[p+j] {
							t.Fatal("foreign VM CString", p+j)
						}
						v := vm.ReadHeap(p+j, 1)[0]
						if v == 0 {
							return string(b)
						}
						b = append(b, v)
					}
					t.Fatal("CString")
					return ""
				})
				for _, fn := range engine.m.Funcs {
					if fn.Attrs.IsExtern {
						name := fn.Name
						vm.Hosts[name] = func(v []mir2.Value) ([]mir2.Value, error) {
							args := []int64{}
							for _, x := range v {
								args = append(args, int64(uint16(x.I)))
							}
							return []mir2.Value{{I: host(name, args)}}, nil
						}
					}
				}
				v, e := vm.Call("main", nil)
				if e != nil || len(v) != 0 {
					t.Fatal(engine.name, e, v)
				}
				check()
				if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
					t.Fatal("VM RAM")
				}
				t.Log(engine.name, "32 frames PASS")
			}
			linked := s.Assembly
			for _, fn := range s.MIR2Module.Funcs {
				if fn.Attrs.IsExtern {
					linked = strings.Replace(linked, fn.Name+":", fn.Name+":\n RET", 1)
				}
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n" + linked)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			sym := func(n string) uint16 {
				n = strings.NewReplacer("$", "_", ".", "_", "@", "_").Replace(n)
				p, ok := a.Symbols[n]
				if !ok {
					t.Fatal("symbol", n)
				}
				return uint16(p)
			}
			save(mode+"/program.a80", "ORG 0x400\n"+linked)
			save(mode+"/program.bin", a.Binary)
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x400:], a.Binary)
			p := sym("main")
			copy(mem[0xc000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			want := append([]byte(nil), mem...)
			allowed := map[int64]bool{}
			for n := 0; n < s.MIR2Module.Strings.Len(); n++ {
				p := sym(fmt.Sprintf("_mir2_str_%d", n))
				for j := 0; j <= len(s.MIR2Module.Strings.At(n)); j++ {
					allowed[int64(p)+int64(j)] = true
				}
			}
			host, check := checker(mode+"/native", func(p int64) string {
				var b []byte
				for j := int64(0); j < 128; j++ {
					if !allowed[p+j] {
						t.Fatal("foreign native CString", p+j)
					}
					v := mem[p+j]
					if v == 0 {
						return string(b)
					}
					b = append(b, v)
				}
				t.Fatal("CString")
				return ""
			})
			externs := map[uint16]*mir2.Func{}
			for _, fn := range s.MIR2Module.Funcs {
				if fn.Attrs.IsExtern {
					p := sym(fn.Name)
					if externs[p] != nil {
						t.Fatal("extern alias")
					}
					externs[p] = fn
				}
			}
			z.SetRegisters(emulator.Registers{PC: 0xc000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			for n := 0; n < 1000000000 && z.GetPC() != 0xc003; n++ {
				if fn := externs[z.GetPC()]; fn != nil {
					r := z.GetRegisters()
					args := []int64{}
					for _, p := range fn.Contract.Params {
						l := assertParamLocation(p, s.Allocation)
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
							if l.Kind != mir2.LocMem {
								t.Fatal("formal", l)
							}
							addr := sym(l.Name)
							v = uint16(mem[addr]) | uint16(mem[addr+1])<<8
						}
						args = append(args, int64(v))
					}
					out := host(fn.Name, args)
					r.HL = uint16(out)
					ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					if ret == 0xc003 {
						if fn.Name != "canvas_save" || r.SP != 0xeffe {
							t.Fatal("loader return authority")
						}
					} else if ret < 0x400 || ret >= uint16(0x400+len(a.Binary)) {
						t.Fatal("foreign return", ret)
					}
					r.PC = ret
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if mem[z.GetPC()] == 0x76 {
					t.Fatal("unexpected HALT")
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xd000 || z.GetSP() > 0xf000 {
					t.Fatal("stack bounds")
				}
			}
			save(mode+"/registers.json", z.GetRegisters())
			save(mode+"/ram.bin", mem)
			if z.GetPC() != 0xc003 || z.GetSP() != 0xf000 {
				t.Fatal("termination", z.GetRegisters())
			}
			check()
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(want, mem) {
				for p := range mem {
					if mem[p] != want[p] {
						t.Fatalf("native RAM %04x got%x want%x", p, mem[p], want[p])
					}
				}
			}
			save(mode+"/summary.json", map[string]any{"result": "PASS", "frames": 32, "image_bytes": len(a.Binary), "T_states": z.GetCycles(), "full_ram": true, "minimum_sp": minSP, "host_registers": "preserved except u16 result"})
			t.Log("native 32 frames", len(a.Binary), z.GetCycles(), "PASS")
		})
	}
}
