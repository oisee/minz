package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestZXHalfCellRotozoomerNative(t *testing.T) {
	src, e := os.ReadFile("../../../reports/zx-rotozoomer-32x48/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	var model struct {
		A       []int  `json:"axis_a"`
		B       []int  `json:"axis_b"`
		Texture []byte `json:"texture"`
	}
	data, e := os.ReadFile("../../../reports/zx-rotozoomer-32x48/model.json")
	if e != nil || json.Unmarshal(data, &model) != nil {
		t.Fatal(e)
	}
	save := func(name string, b []byte) {
		if d := os.Getenv("MINZ_ZX_ROTO_EXPORT"); d != "" {
			p := filepath.Join(d, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(src), "halfcell")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := hir.LowerModule(parse())
			s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			asm := "ORG 0x8000\n" + s.Assembly
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			save(mode+"/program.a80", []byte(asm))
			save(mode+"/program.bin", a.Binary)
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, a.Binary)
			mem := z.Memory()
			call := func(fn string, arg byte) int {
				mem[0xeff0] = 0
				mem[0xeff1] = 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[fn]), SP: 0xeff0, A: arg})
				start := z.GetCycles()
				for i := 0; i < 5000000; i++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						return z.GetCycles() - start
					}
					z.Step()
				}
				t.Fatal("call did notreturn", fn, z.GetRegisters())
				return 0
			}
			call("init_screen", 0)
			for addr := 0x4000; addr < 0x5800; addr++ {
				want := byte(0)
				if addr&0x400 != 0 {
					want = 255
				}
				if mem[addr] != want {
					t.Fatalf("bitmap %04x=%02x want%02x", addr, mem[addr], want)
				}
			}
			vmraw, vmfinal := mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)
			vmraw.EnsureHeap(65536)
			vmfinal.EnsureHeap(65536)
			cycles := []int{}
			for frame := 0; frame < 32; frame++ {
				want := make([]byte, 768)
				aa, bb := model.A[frame], model.B[frame]
				ha, hb := aa/2, bb/2
				sample := func(u, v int) byte {
					x := int(uint16(u)>>8) & 15
					y := int(uint16(v)>>8) & 15
					return model.Texture[y*16+x]
				}
				for y := 0; y < 24; y++ {
					for x := 0; x < 32; x++ {
						u := 32768 - aa*16 - bb*12 + x*aa + y*hb*2
						v := 32768 + bb*16 - aa*12 - x*bb + y*ha*2
						want[y*32+x] = 64 | sample(u, v)<<3 | sample(u+hb, v+ha)
					}
				}
				for _, vm := range []*mir2.VM{vmraw, vmfinal} {
					if _, e := vm.Call("draw_frame", []mir2.Value{{I: int64(frame)}}); e != nil {
						t.Fatal(e)
					}
					if !bytes.Equal(vm.ReadHeap(0x5800, 768), want) {
						t.Fatal("VM frame", frame)
					}
				}
				before := append([]byte(nil), mem...)
				cycles = append(cycles, call("draw_frame", byte(frame)))
				if !bytes.Equal(mem[0x5800:0x5b00], want) {
					for i, b := range want {
						if mem[0x5800+i] != b {
							t.Fatalf("frame%d attr%d got%02x want%02x", frame, i, mem[0x5800+i], b)
						}
					}
				}
				if !bytes.Equal(before[0x4000:0x5800], mem[0x4000:0x5800]) || !bytes.Equal(mem[0x8000:0x8000+len(a.Binary)], a.Binary) {
					t.Fatal("bitmap orcode modified")
				}
				save(fmt.Sprintf("%s/frame-%02d.scr", mode, frame), append([]byte(nil), mem[0x4000:0x5b00]...))
			}
			result, _ := json.MarshalIndent(map[string]any{"PASS": true, "frames": 32, "bytes": len(a.Binary), "T_states": cycles, "scope": "all768attributes andfixed6144bitmap perframe;raw/finalVM;exactreturnPC/SP;codeunchanged;noULAcontention"}, "", "  ")
			save(mode+"/result.json", result)
			t.Log(mode, len(a.Binary), "bytes frame0", cycles[0], "T;all32framesPASS")
		})
	}
}
