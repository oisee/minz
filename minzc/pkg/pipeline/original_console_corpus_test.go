package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Original modules execute on native Z80: VM OpAsm is deliberately not an oracle.
func TestOriginalConsoleNative(t *testing.T) {
	paths := []string{"reports/showcase-src/2026-03-11/ex18_user_console_io.nanz", "reports/showcase-src/2026-03-13/ex18_user_console_io.nanz", "examples/zx/console_demo.nanz"}
	export := func(name string, b []byte) {
		if d := os.Getenv("CONSOLE_CORPUS_EXPORT"); d != "" {
			p := filepath.Join(d, name)
			os.MkdirAll(filepath.Dir(p), 0755)
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for i, path := range paths {
		for _, mode := range []struct {
			name string
			o    Options
		}{{"ordinary", Options{ContractOpt: true, AssertMode: "none"}}, {"grace-size", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}}} {
			t.Run(fmt.Sprintf("source%d/%s", i, mode.name), func(t *testing.T) {
				b, e := os.ReadFile("../../../" + path)
				if e != nil {
					t.Fatal(e)
				}
				h, e := nanz.ParseWithOpts(string(b), path, nanz.ParseOpts{BaseDir: filepath.Dir("../../../" + path), StdlibDir: "../../../stdlib"})
				if e != nil {
					t.Fatal(e)
				}
				s, e := CompileHIRSteps(h, mode.o)
				if e != nil {
					t.Fatal(e)
				}
				callerAssembly := calledMainTestAssembly(t, s)
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + callerAssembly)
				if e != nil || a == nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors)
				}
				prefix := fmt.Sprintf("source%d/%s", i, mode.name)
				export(prefix+".a80", []byte(callerAssembly))
				export(prefix+".bin", a.Binary)
				export(prefix+".mir", []byte(s.MIR2Module.Dump()))
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				prof := emulator.NewProfiler()
				z.SetProfiler(prof)
				for n := range z.Memory() {
					z.Memory()[n] = byte(n*41 + 29)
				}
				z.LoadMemory(0x8000, a.Binary)
				if i == 2 {
					for ch := 32; ch < 128; ch++ {
						for line := 0; line < 8; line++ {
							z.Memory()[0x3d00+(ch-32)*8+line] = byte((ch*17 + line*29) ^ 0x5a)
						}
					}
				}
				stdout, stderr := []byte{}, []byte{}
				ports := []uint16{}
				reads := []uint16{}
				z.SetIOHandlers(func(p uint16) byte {
					reads = append(reads, p)
					if i == 2 && len(reads) > 8 && byte(p>>8) == 0xfd {
						return 0xfe
					}
					return 0xff
				}, func(p uint16, v byte) {
					ports = append(ports, p)
					switch byte(p) {
					case 0x23:
						stdout = append(stdout, v)
					case 0x25:
						stderr = append(stderr, v)
					}
				})
				target := a.Symbols["main"]
				z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x97, F: 0x45, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
				before := append([]byte(nil), z.Memory()...)
				cycles := int64(0)
				depth := uint16(0xf000)
				done := false
				for n := 0; n < 2000000; n++ {
					if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
						done = true
						break
					}
					cycles += int64(z.Step())
					if z.GetSP() < depth {
						depth = z.GetSP()
					}
				}
				changed := []int{}
				for n, v := range z.Memory() {
					if v != before[n] {
						changed = append(changed, n)
					}
				}
				result := map[string]any{"source": path, "mode": mode.name, "bytes": len(a.Binary), "t_states": cycles, "returned": done, "registers": z.GetRegisters(), "min_sp": depth, "stdout": string(stdout), "stderr": string(stderr), "ports": ports, "reads": reads, "changed_addresses": changed}
				if i < 2 {
					if !done || string(stdout) != "AB\nsame thing!\n" || string(stderr) != "ERR\n" {
						t.Fatalf("native output: returned=%v stdout=%q stderr=%q", done, stdout, stderr)
					}
					for n, p := range ports {
						var v byte
						if n < 3 {
							v = stdout[n]
						} else if n < 7 {
							v = stderr[n-3]
						} else {
							v = stdout[n-4]
						}
						if byte(p>>8) != v {
							t.Fatal("OUT high byte", p, v)
						}
					}
					// Independent helper domains, additional to the original whole-main call.
					setArg := func(f *mir2.Func, v uint16) {
						l := s.Allocation.Locs[f.Contract.Params[0].Reg]
						if f.Contract.Params[0].Ty.Width() == 8 {
							if e := z.SetRegister8(l.Name, byte(v)); e != nil {
								t.Fatal(e)
							}
						} else {
							rr := z.GetRegisters()
							switch l.Name {
							case "HL":
								rr.HL = v
							case "DE":
								rr.DE = v
							case "BC":
								rr.BC = v
							default:
								t.Fatal(l)
							}
							z.SetRegisters(rr)
						}
					}
					callHelper := func(name string, v uint16) {
						var f *mir2.Func
						for _, ff := range s.MIR2Module.Funcs {
							if ff.Name == name {
								f = ff
							}
						}
						if f == nil {
							t.Fatal(name)
						}
						target := a.Symbols[name]
						z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
						z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, BC: 0x1234, DE: 0x5678, HL: 0x9abc})
						setArg(f, v)
						for step := 0; step < 10000; step++ {
							if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
								return
							}
							z.Step()
						}
						t.Fatal("helper return", name, v)
					}
					for v := 0; v < 256; v++ {
						stdout = nil
						stderr = nil
						callHelper("console_log", uint16(v))
						if len(stdout) != 1 || stdout[0] != byte(v) {
							t.Fatal("log", v, stdout)
						}
						callHelper("console_err", uint16(v))
						if len(stderr) != 1 || stderr[0] != byte(v) {
							t.Fatal("err", v, stderr)
						}
					}
					for length := 0; length < 256; length++ {
						for _, start := range []uint16{0x4000, 0xfffe} {
							payload := make([]byte, length)
							for j := range payload {
								payload[j] = byte(1 + (j*37)%255)
								z.Memory()[uint16(int(start)+j)] = payload[j]
							}
							z.Memory()[uint16(int(start)+length)] = 0
							stdout = nil
							stderr = nil
							callHelper("print_str", start)
							if string(stdout) != string(payload) || len(stderr) != 0 {
								t.Fatal("str", start, length, len(stdout))
							}
							for j := range payload {
								if z.Memory()[uint16(int(start)+j)] != payload[j] {
									t.Fatal("readonlystr")
								}
							}
						}
					}
					result["helper_calls"] = 1024
					for _, n := range changed {
						if n < int(depth) || n >= 0xf000 {
							t.Fatal("memory outside stack", n)
						}
					}
				} else {
					want := make([]byte, 0x1b00)
					for n := 0x1800; n < len(want); n++ {
						want[n] = 15
					}
					row, col, attr := 0, 0, byte(15)
					put := func(text string) {
						for _, ch := range []byte(text) {
							if ch == 10 {
								row++
								col = 0
								continue
							}
							for line := 0; line < 8; line++ {
								addr := ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col
								want[addr] = byte((int(ch)*17 + line*29) ^ 0x5a)
							}
							want[0x1800+row*32+col] = attr
							col++
						}
					}
					put("Hello from MinZ!\n\nStandalone ZX console\nNo ROM calls, no IM1\nJust font data at $3D00\n\n")
					attr = 6
					put("Score: 12345\n")
					attr = 3
					put("Lives: 3\n\n")
					attr = 15
					put("Press any key...\nYou pressed: a\n")
					mismatch := -1
					for n, v := range want {
						if z.Memory()[0x4000+n] != v {
							mismatch = 0x4000 + n
							break
						}
					}
					result["first_screen_mismatch"] = mismatch
					result["expected_row"] = row
					result["expected_col"] = col
					result["actual_row"] = z.Memory()[a.Symbols["zx__console__con_row"]]
					result["actual_col"] = z.Memory()[a.Symbols["zx__console__con_col"]]
					if !done || mismatch >= 0 || row != 11 || col != 0 || result["actual_row"] != byte(11) || result["actual_col"] != byte(0) {
						t.Fatalf("ZX corrected returned=%v screen mismatch=%04x", done, mismatch)
					}
					// The printable A key requires a SYMBOL SHIFT (row 7F) lookup
					// between row FD and FB; CAPS SHIFT is checked after the key scan.
					expectedReads := []uint16{
						0xfefe, 0xfdfe, 0xfbfe, 0xf7fe, 0xeffe, 0xdffe, 0xbffe, 0x7ffe,
						0xfefe, 0xfdfe, 0x7ffe, 0xfbfe, 0xf7fe, 0xeffe, 0xdffe, 0xbffe, 0x7ffe,
						0xfefe,
					}
					if fmt.Sprint(reads) != fmt.Sprint(expectedReads) || len(ports) != 0 {
						t.Fatal("keyboard/ports", reads, ports)
					}
					allowed := map[int]bool{}
					for n := 0x4000; n <= 0x5aff; n++ {
						allowed[n] = true
					}
					for n := int(depth); n < 0xf000; n++ {
						allowed[n] = true
					}
					for _, name := range []string{"zx__console__con_col", "zx__console__con_row", "zx__console__con_attr"} {
						allowed[int(a.Symbols[name])] = true
					}
					for n := range z.Memory() {
						if !allowed[n] && (z.Memory()[n] != before[n] || prof.WriteCount[n] != 0) {
							t.Fatalf("ZX write outside explicit screen/globals/stack: %04x", n)
						}
					}
					t.Logf("ZX corrected returned=%v screen mismatch=%04x cursor=%v/%v", done, mismatch, result["actual_row"], result["actual_col"])

				}
				out, _ := json.MarshalIndent(result, "", "  ")
				export(prefix+".json", out)
			})
		}
	}
}
