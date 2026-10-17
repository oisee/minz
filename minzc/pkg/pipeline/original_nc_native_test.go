package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestOriginalNCWholeMainNative(t *testing.T) {
	const packet = "../../../reports/nc-whole-main/frozen/"
	read := func(n string) []byte {
		b, e := os.ReadFile(packet + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var f struct {
		Sources   map[string]string `json:"sources"`
		DiskSHA   string            `json:"disk_sha256"`
		EventsSHA string            `json:"terminal_oracle_sha256"`
		Keys      []int64           `json:"input_keys"`
		Globals   map[string]int64  `json:"final_globals"`
		FS        map[string]int64  `json:"expected_filesystem"`
		Buffers   map[string][]byte `json:"final_memory_buffers"`
		Scalars   map[string]int64  `json:"final_memory_scalars"`
	}
	if e := json.Unmarshal(read("freeze.json"), &f); e != nil {
		t.Fatal(e)
	}
	for p, w := range f.Sources {
		b, e := os.ReadFile("../../../" + p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != w {
			t.Fatal("source freeze", p, e)
		}
	}
	disk := read("disk.img")
	b := read("terminal-events.json")
	if fmt.Sprintf("%x", sha256.Sum256(disk)) != f.DiskSHA || fmt.Sprintf("%x", sha256.Sum256(b)) != f.EventsSHA {
		t.Fatal("input freeze")
	}
	var expected []commanderEvent
	if e := json.Unmarshal(b, &expected); e != nil {
		t.Fatal(e)
	}
	save := func(name string, v any) {
		if dir := os.Getenv("MINZ_NC_NATIVE_EXPORT"); dir != "" {
			p := filepath.Join(dir, name)
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
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(string(read("nc.nanz")), "nc.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			linked := calledMainTestAssembly(t, s)
			for _, fn := range s.MIR2Module.Funcs {
				if fn.Attrs.IsExtern {
					linked = strings.Replace(linked, fn.Name+":", fn.Name+":\n RET", 1)
				}
			}
			asm := "ORG 0x0400\n" + linked
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			if len(a.Binary) > 0xc000-0x400 {
				t.Fatal("image overlaps test driver")
			}
			sym := func(name string) uint16 {
				name = strings.NewReplacer("$", "_", ".", "_", "@", "_").Replace(name)
				v, ok := a.Symbols[name]
				if !ok {
					v, ok = a.Symbols[strings.ToUpper(name)]
				}
				if !ok {
					t.Fatal("symbol", name)
				}
				return uint16(v)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0) // Flat test RAM, not the emulator default ZX ROM region.
			z.LoadMemory(0x400, a.Binary)
			main := sym("main")
			z.LoadMemory(0xc000, []byte{0xcd, byte(main), byte(main >> 8), 0x76})
			z.SetRegisters(emulator.Registers{PC: 0xc000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			initial := make([]byte, 65536)
			for i := range initial {
				initial[i] = z.GetMemory(uint16(i))
			}
			want := append([]byte(nil), initial...)
			externs := map[uint16]*mir2.Func{}
			for _, fn := range s.MIR2Module.Funcs {
				if fn.Attrs.IsExtern {
					pc := sym(fn.Name)
					if externs[pc] != nil {
						t.Fatal("extern alias")
					}
					externs[pc] = fn
				}
			}
			mutable := map[uint16]bool{}
			for _, g := range s.MIR2Module.Globals {
				if !g.IsConst {
					for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
						mutable[sym(g.Name)+uint16(i)] = true
					}
				}
			}
			var events []commanderEvent
			var reads []int64
			keys := 0
			minSP := uint16(0xf000)
			done := false
			for step := 0; step < 5000000; step++ {
				pc := z.GetPC()
				if pc == 0xc003 {
					done = true
					break
				}
				if fn := externs[pc]; fn != nil {
					r := z.GetRegisters()
					args := []int64{}
					for _, p := range fn.Contract.Params {
						loc := assertParamLocation(p, s.Allocation)
						var v uint16
						switch loc.Name {
						case "A":
							v = uint16(r.A)
						case "B":
							v = r.BC >> 8
						case "C":
							v = r.BC & 255
						case "D":
							v = r.DE >> 8
						case "E":
							v = r.DE & 255
						case "H":
							v = r.HL >> 8
						case "L":
							v = r.HL & 255
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
							if loc.Kind != mir2.LocMem {
								t.Fatal("formal", loc)
							}
							addr := sym(loc.Name)
							v = uint16(z.GetMemory(addr))
							if p.Ty.Width() > 8 {
								v |= uint16(z.GetMemory(addr+1)) << 8
							}
						}
						args = append(args, int64(v))
					}
					// Only the original extern names in this program have host authority.
					name := fn.Name
					out := int64(0)
					hasResult := len(fn.Contract.Returns) > 0
					evArgs := []any{}
					for _, v := range args {
						evArgs = append(evArgs, float64(v))
					}
					if strings.HasSuffix(name, "disk_read") {
						if len(args) != 4 || args[0] != 0 || args[1] != int64(sym("fs__fat12__win")) || (args[2] != 0 && args[2] != 2) || args[3] != 1 {
							t.Fatal("native disk authority", name, args)
						}
						reads = append(reads, args[2])
						for i, b := range disk[args[2]*512 : (args[2]+1)*512] {
							p := uint16(args[1]) + uint16(i)
							z.SetMemory(p, b)
							want[p] = b
						}
					} else if strings.HasSuffix(name, "disk_write") {
						t.Fatal("unexpected native disk write")
					} else {
						switch name {
						case "tui_puts":
							if len(args) != 1 {
								t.Fatal("puts arity")
							}
							var text strings.Builder
							terminated := false
							for i := int64(0); i < 256; i++ {
								p := args[0] + i
								if p < 0 || p >= 65536 {
									t.Fatal("CString bounds")
								}
								b := z.GetMemory(uint16(p))
								if b == 0 {
									terminated = true
									break
								}
								text.WriteByte(b)
							}
							if !terminated {
								t.Fatal("CString termination")
							}
							evArgs = []any{text.String()}
						case "tui_read_key":
							if keys >= len(f.Keys) {
								t.Fatal("key exhausted")
							}
							out = f.Keys[keys]
							keys++
						case "tui_width":
							out = 80
						case "tui_height":
							out = 10
						case "tui_goto", "tui_color", "tui_reset", "tui_clear", "tui_putch":
						default:
							t.Fatal("unknown native host", name)
						}
						events = append(events, commanderEvent{name, evArgs})
					}
					// Host contract preserves physical registers except the declared return.
					if hasResult {
						loc := mir2.ReturnLocation(fn.Contract.Returns[0].Class, fn.Contract.Returns[0].Ty)
						switch loc {
						case "A":
							r.A = byte(out)
						case "HL":
							r.HL = uint16(out)
						default:
							t.Fatal("return ABI", loc)
						}
					}
					ret := uint16(z.GetMemory(r.SP)) | uint16(z.GetMemory(r.SP+1))<<8
					if ret == 0xc003 && name == "tui_reset" && r.SP == 0xeffe {
						// Original main tail-calls reset, retaining the loader CALL authority.
					} else if ret < 0x400 || ret >= uint16(0x400+len(a.Binary)) {
						t.Fatal("foreign extern return", name, ret)
					}
					r.PC = ret
					r.SP += 2
					z.SetRegisters(r)
					continue
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xd000 || z.GetSP() > 0xf000 {
					t.Fatal("stack bounds")
				}
			}
			save(mode+"/program.a80", asm)
			save(mode+"/program.bin", a.Binary)
			save(mode+"/events.json", events)
			save(mode+"/registers.json", z.GetRegisters())
			save(mode+"/allocation.json", s.Allocation)
			save(mode+"/final.mir", s.MIR2Module.Dump())
			state := map[string]int64{}
			for _, name := range []string{"cursor", "top", "n_entries", "panel_h"} {
				state[name] = int64(z.GetMemory(sym(name)))
			}
			save(mode+"/final-state.json", state)
			for _, name := range []string{"namebuf", "sizebuf", "dirent"} {
				n := 14
				if name == "sizebuf" {
					n = 8
				}
				if name == "dirent" {
					n = 32
				}
				data := make([]byte, n)
				for i := range data {
					data[i] = z.GetMemory(sym(name) + uint16(i))
				}
				save(mode+"/"+name+".bin", data)
			}
			imageAfter := make([]byte, len(a.Binary))
			for i := range imageAfter {
				imageAfter[i] = z.GetMemory(uint16(0x400 + i))
			}
			save(mode+"/image-after.bin", imageAfter)
			save(mode+"/disk-reads.json", reads)
			if !done || z.GetPC() != 0xc003 || z.GetSP() != 0xf000 {
				t.Fatal("main termination", done, z.GetRegisters())
			}
			if !reflect.DeepEqual(events, expected) {
				for i := 0; i < len(events) && i < len(expected); i++ {
					if !reflect.DeepEqual(events[i], expected[i]) {
						t.Fatalf("native event%d got%v want%v", i, events[i], expected[i])
					}
				}
				t.Fatal("event count", len(events), len(expected))
			}
			if keys != len(f.Keys) || !reflect.DeepEqual(reads, []int64{0, 2}) {
				t.Fatal("native input", keys, reads)
			}
			for name, v := range f.Globals {
				if z.GetMemory(sym(name)) != byte(v) {
					t.Fatal("global", name)
				}
			}
			for name, v := range f.FS {
				p := sym("fs__fat12__" + name)
				got := int64(z.GetMemory(p))
				for _, g := range s.MIR2Module.Globals {
					if g.Name == "fs__fat12__"+name && mir2.ByteWidth(g.Ty) == 2 {
						got |= int64(z.GetMemory(p+1)) << 8
					}
				}
				if got != v {
					t.Fatal("filesystem", name, got, v)
				}
			}
			mem := make([]byte, 65536)
			for i := range mem {
				mem[i] = z.GetMemory(uint16(i))
			}
			if !bytes.Equal(mem[sym("fs__fat12__win"):int(sym("fs__fat12__win"))+512], disk[1024:1536]) {
				t.Fatal("native window")
			}
			// Compiler-created spill cells are precise assembler listing spans.
			for _, line := range a.Listing {
				if strings.HasPrefix(line.Label, "_spill_") {
					for i := 0; i < len(line.Bytes); i++ {
						mutable[uint16(line.Address+i)] = true
					}
				}
			}
			// Independent final bytes cover every module-global object. Unused
			// objects retain declaration initialization; only actual compiler
			// spill spans and observed stack bytes have operational contents.
			for _, gl := range s.MIR2Module.Globals {
				addr := int(sym(gl.Name))
				width := mir2.ByteWidth(gl.Ty)
				data := make([]byte, width)
				copy(data, gl.Init)
				if buf, ok := f.Buffers[gl.Name]; ok {
					if len(buf) != width {
						t.Fatal("buffer model width", gl.Name)
					}
					copy(data, buf)
				}
				if value, ok := f.Scalars[gl.Name]; ok {
					for i := 0; i < width; i++ {
						data[i] = byte(uint64(value) >> uint(8*i))
					}
				}
				copy(want[addr:addr+width], data)
			}
			for _, line := range a.Listing {
				if strings.HasPrefix(line.Label, "_spill_") {
					copy(want[line.Address:line.Address+len(line.Bytes)], mem[line.Address:line.Address+len(line.Bytes)])
				}
			}
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			if !bytes.Equal(mem, want) {
				for p := range mem {
					if mem[p] != want[p] {
						t.Fatalf("native full RAM %04x got%02x want%02x", p, mem[p], want[p])
					}
				}
			}

			save(mode+"/ram.bin", mem)
			save(mode+"/summary.json", map[string]any{"events": len(events), "image_bytes": len(a.Binary), "T_states": z.GetCycles(), "PC": z.GetPC(), "SP": z.GetSP(), "minimum_sp": minSP})
			t.Logf("native whole main %dB %dT, events%d PC/SP restored", len(a.Binary), z.GetCycles(), len(events))
		})
	}
}
