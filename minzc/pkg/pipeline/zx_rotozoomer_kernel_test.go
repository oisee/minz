package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestZXHalfCellRotozoomerKernelNative(t *testing.T) {
	src, e := os.ReadFile("../../../reports/zx-rotozoomer-kernel/source.nanz")
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
		if d := os.Getenv("MINZ_ZX_ROTO_KERNEL_EXPORT"); d != "" {
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
			// Spill homes are mutable data inside the assembled image; exempt
			// exactly their declared DB/DW ranges, not surrounding instructions.
			spillBytes := map[int]bool{}
			for _, line := range strings.Split(asm, "\n") {
				fields := strings.Fields(line)
				if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
					continue
				}
				name := strings.TrimSuffix(fields[0], ":")
				base, ok := a.Symbols[name]
				if !ok {
					t.Fatal("spill symbol", name)
				}
				width := 1
				if fields[1] == "DW" {
					width = 2
				} else if fields[1] != "DB" {
					t.Fatal("spill declaration", line)
				}
				values := strings.Join(fields[2:], "")
				for _, value := range strings.Split(values, ",") {
					if _, err := strconv.Atoi(value); err != nil {
						t.Fatal("spill data", line)
					}
				}
				for j := 0; j < width*len(strings.Split(values, ",")); j++ {
					spillBytes[base+j] = true
				}
			}
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
			call("init_texture", 0)
			for addr := 0x4000; addr < 0x5800; addr++ {
				want := byte(0)
				if addr&0x400 != 0 {
					want = 255
				}
				if mem[addr] != want {
					t.Fatalf("bitmap %04x=%02x want%02x", addr, mem[addr], want)
				}
			}
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
				before := append([]byte(nil), mem...)
				cycles = append(cycles, call("draw_frame", byte(frame)))
				if !bytes.Equal(mem[0x5800:0x5b00], want) {
					for i, b := range want {
						if mem[0x5800+i] != b {
							t.Fatalf("frame%d attr%d got%02x want%02x", frame, i, mem[0x5800+i], b)
						}
					}
				}
				if !bytes.Equal(before[0x4000:0x5800], mem[0x4000:0x5800]) {
					t.Fatal("bitmap modified")
				}
				for _, name := range []string{"step_a", "step_minus_b", "step_half_a", "step_half_b", "row_step_u", "row_step_v", "origin_u", "origin_v"} {
					base := a.Symbols[name]
					spillBytes[base] = true
					spillBytes[base+1] = true
				}
				for j, b := range a.Binary {
					if !spillBytes[0x8000+j] && mem[0x8000+j] != b {
						t.Fatalf("immutable code/data changed at %04x", 0x8000+j)
					}
				}
				save(fmt.Sprintf("%s/frame-%02d.scr", mode, frame), append([]byte(nil), mem[0x4000:0x5b00]...))
			}
			// The generated walker must still sample RAM, not replay frames.
			for i := 0; i < 256; i++ {
				mem[0xc000+i] = 5
			}
			call("draw_frame", 9)
			if !bytes.Equal(mem[0x5800:0x5b00], bytes.Repeat([]byte{64 | 5<<3 | 5}, 768)) {
				t.Fatal("kernel ignored mutable texture")
			}
			result, _ := json.MarshalIndent(map[string]any{"PASS": true, "frames": 32, "bytes": len(a.Binary), "T_states": cycles, "scope": "all768attributes andfixed6144bitmap perframe;nativeASMkernel;exactreturnPC/SP;codeunchanged;noULAcontention"}, "", "  ")
			save(mode+"/result.json", result)
			t.Log(mode, len(a.Binary), "bytes frame0", cycles[0], "T;all32framesPASS")
		})
	}
}
