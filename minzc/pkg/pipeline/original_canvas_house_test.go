package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type canvasHouseEvent struct {
	Name string
	Args []int64
}

func canvasHouseExpected() []canvasHouseEvent {
	return []canvasHouseEvent{
		{"canvas_init", []int64{256, 192, 0}}, {"canvas_clear", []int64{0}}, {"canvas_fill_rect", []int64{0, 0, 256, 96, 1}}, {"canvas_fill_rect", []int64{0, 96, 256, 96, 4}}, {"canvas_circle", []int64{200, 40, 25, 14}}, {"canvas_fill_rect", []int64{60, 70, 60, 50, 2}}, {"canvas_rect", []int64{60, 70, 60, 50, 15}}, {"canvas_fill_rect", []int64{80, 95, 15, 25, 6}}, {"canvas_line", []int64{55, 70, 90, 40, 15}}, {"canvas_line", []int64{90, 40, 125, 70, 15}}, {"canvas_fill_rect", []int64{65, 78, 10, 10, 13}}, {"canvas_fill_rect", []int64{170, 85, 6, 35, 6}}, {"canvas_circle", []int64{173, 75, 20, 4}}}
}
func TestOriginalCanvasHouseWholeScene(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/canvas_house.nanz")
	if e != nil {
		t.Fatal(e)
	}
	freeze, e := os.ReadFile("../../../reports/canvas-house-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var fr struct {
		SHA string `json:"source_sha256"`
	}
	if json.Unmarshal(freeze, &fr) != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != fr.SHA {
		t.Fatal("sourcefreeze")
	}
	save := func(n string, v any) {
		if root := os.Getenv("MINZ_CANVAS_HOUSE_ARTIFACTS"); root != "" {
			p := filepath.Join(root, n)
			os.MkdirAll(filepath.Dir(p), 0755)
			var b []byte
			switch x := v.(type) {
			case []byte:
				b = x
			case string:
				b = []byte(x)
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "canvas_house.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		opt := Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"}
		raw := hir.LowerModule(parse())
		s, e := CompileHIRSteps(parse(), opt)
		if e != nil {
			t.Fatal(e)
		}
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		callerAssembly := calledMainTestAssembly(t, s)
		save(mode+"/unlinked.a80", callerAssembly)
		save(mode+"/allocation.json", s.Allocation)
		for _, entry := range []string{"draw_scene", "main"} {
			expected := canvasHouseExpected()
			for _, engine := range []struct {
				name string
				m    *mir2.Module
			}{{"raw", raw}, {"final", s.MIR2Module}} {
				vm := mir2.NewVM(engine.m)
				ref := mir2.RegisterCanvasHosts(vm)
				events := []canvasHouseEvent{}
				path := ""
				for _, f := range engine.m.Funcs {
					if f.Attrs.IsExtern {
						name := f.Name
						original := vm.Hosts[name]
						if original == nil {
							t.Fatal("unbound", name)
						}
						vm.Hosts[name] = func(args []mir2.Value) ([]mir2.Value, error) {
							vals := []int64{}
							for _, v := range args {
								vals = append(vals, v.I)
							}
							if name == "canvas_save" {
								p := vals[0]
								for n := 0; n < 64; n++ {
									v := vm.ReadHeap(p+int64(n), 1)
									if len(v) != 1 {
										t.Fatal("savebounds")
									}
									if v[0] == 0 {
										break
									}
									path += string(v[0])
								}
								return []mir2.Value{{I: 0}}, nil
							}
							events = append(events, canvasHouseEvent{name, vals})
							return original(args)
						}
					}
				}
				v, e := vm.Call(entry, nil)
				if e != nil {
					t.Fatal(mode, engine.name, entry, e)
				}
				if entry == "draw_scene" && (len(v) != 1 || v[0].I != 0) {
					t.Fatal(v)
				}
				if !reflect.DeepEqual(events, expected) {
					save(mode+"/"+entry+"/"+engine.name+"-wrong-events.json", events)
					t.Fatal("VMevents", mode, entry, engine.name, events)
				}
				if entry == "main" && path != "build/canvas_house.png" {
					t.Fatal("savepath", path)
				}
				pixels := ref.C.Pixels
				if !bytes.Equal(pixels, canvasHousePixels(expected)) {
					t.Fatal("independent framebuffer", mode, engine.name, entry)
				}
				save(mode+"/"+entry+"/"+engine.name+"-pixels.bin", pixels)
				save(mode+"/"+entry+"/"+engine.name+"-events.json", events)
				if root := os.Getenv("MINZ_CANVAS_HOUSE_ARTIFACTS"); root != "" {
					name := filepath.Join(root, mode, entry, engine.name+"-frame.png")
					if e := ref.C.SavePNG(name); e != nil {
						t.Fatal(e)
					}
				}
			}
			linked := callerAssembly
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					label := f.Name + ":"
					if strings.Count(linked, label) != 1 {
						t.Fatal("extern label", f.Name)
					}
					linked = strings.Replace(linked, label, label+"\n RET ; test-owned extern entry", 1)
				}
			}
			asm := "ORG 0x100\n" + linked
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x100:], a.Binary)
			p := a.Symbols[entry]
			copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			before := append([]byte(nil), mem...)
			nativeEvents := []canvasHouseEvent{}
			nativeVM := mir2.NewVM(&mir2.Module{})
			nativeCanvas := mir2.RegisterCanvasHosts(nativeVM)
			savePath := ""
			externs := map[uint16]*mir2.Func{}
			for _, f := range s.MIR2Module.Funcs {
				if f.Attrs.IsExtern {
					externs[uint16(a.Symbols[f.Name])] = f
				}
			}
			param := func(p mir2.Param, r emulator.Registers) int64 {
				loc := assertParamLocation(p, s.Allocation)
				switch loc.Kind {
				case mir2.LocMem:
					addr, ok := a.Symbols[loc.Name]
					if !ok {
						t.Fatal("spill symbol", loc)
					}
					return int64(uint16(mem[addr]) | uint16(mem[addr+1])<<8)
				case mir2.LocReg, mir2.LocIXY:
					switch loc.Name {
					case "HL":
						return int64(r.HL)
					case "DE":
						return int64(r.DE)
					case "BC":
						return int64(r.BC)
					case "IX":
						return int64(r.IX)
					case "IY":
						return int64(r.IY)
					}
				}
				t.Fatal("externhome", p, loc)
				return 0
			}
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			done := false
			for step := 0; step < 100000; step++ {
				pc := z.GetPC()
				if pc == 0x7003 {
					done = true
					break
				}
				if f := externs[pc]; f != nil {
					r := z.GetRegisters()
					vals := []int64{}
					for _, p := range f.Contract.Params {
						vals = append(vals, param(p, r))
					}
					if f.Name == "canvas_save" {
						for n := 0; n < 64; n++ {
							v := mem[int(vals[0])+n]
							if v == 0 {
								break
							}
							savePath += string(v)
						}
					} else {
						nativeEvents = append(nativeEvents, canvasHouseEvent{f.Name, vals})
						vs := []mir2.Value{}
						for _, v := range vals {
							vs = append(vs, mir2.Value{I: v})
						}
						if _, e := nativeVM.Hosts[f.Name](vs); e != nil {
							t.Fatal("device", e)
						}
					}
					r.A = 0x91
					r.F = 0xd7
					r.BC = 0xdead
					r.DE = 0xbeef
					r.IX = 0x1357
					r.IY = 0x2468
					r.HL = 0 // u16 host result; unknown external ABI clobbers primary scratch
					r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
					t.Fatal("fetch", pc)
				}
				if mem[pc] == 0x76 {
					if entry != "main" {
						t.Fatal("unexpected halt")
					}
					done = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 {
					t.Fatal("stack")
				}
			}
			save(mode+"/"+entry+"/native-events.json", nativeEvents)
			save(mode+"/"+entry+"/program.a80", asm)
			save(mode+"/"+entry+"/program.bin", a.Binary)
			save(mode+"/"+entry+"/registers.json", z.GetRegisters())
			if !done || z.GetSP() != 0xf000 {
				t.Fatal("terminationSP", mode, entry, z.GetRegisters())
			}
			if !bytes.Equal(nativeCanvas.C.Pixels, canvasHousePixels(expected)) {
				t.Fatal("native framebuffer")
			}
			save(mode+"/"+entry+"/native-pixels.bin", nativeCanvas.C.Pixels)
			if root := os.Getenv("MINZ_CANVAS_HOUSE_ARTIFACTS"); root != "" {
				name := filepath.Join(root, mode, entry, "native-frame.png")
				if e := nativeCanvas.C.SavePNG(name); e != nil {
					t.Fatal(e)
				}
			}
			if !reflect.DeepEqual(nativeEvents, expected) {
				t.Fatal("nativeevents", mode, entry, nativeEvents)
			}
			if entry == "main" && savePath != "build/canvas_house.png" {
				t.Fatal("nativepath", savePath)
			}
			want := append([]byte(nil), before...)
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(want, mem) {
				for p := range mem {
					if want[p] != mem[p] {
						t.Fatalf("RAM%04x", p)
					}
				}
			}
			save(mode+"/"+entry+"/summary.json", map[string]any{"events": len(nativeEvents), "path": savePath, "full_ram": true, "SP": z.GetSP(), "bytes": len(a.Binary), "T_states": z.GetCycles(), "external_clobber_stress": true})
			t.Log(mode, entry, "raw/final/native ordered events PASS")
		}
	}
}
