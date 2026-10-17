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
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Executes imported SQLite and TUI ASM, with the existing real SQLite port
// adapter. The expected output/column stream is frozen independently.
func TestOriginalSQLiteImportedMainNative(t *testing.T) {
	testOriginalSQLitePortMain(t, "sqlite_test.nanz", "sqlite-import-main", "MINZ_SQLITE_IMPORT_EXPORT")
}

func TestOriginalSAPMaraPortMainNative(t *testing.T) {
	testOriginalSQLitePortMain(t, "sap_mara_cpm.nanz", "sap-mara-port-main", "MINZ_SAP_MARA_PORT_EXPORT")
}

func TestOriginalZSQLPortMainNative(t *testing.T) {
	testOriginalSQLitePortMain(t, "zsql.nanz", "zsql-port-main", "MINZ_ZSQL_PORT_EXPORT")
}

func testOriginalSQLitePortMain(t *testing.T, sourceName, packetName, exportEnv string) {
	t.Helper()
	packet := "../../../reports/" + packetName + "/frozen/"
	read := func(n string) []byte {
		b, e := os.ReadFile(packet + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var f struct {
		Sources  map[string]string
		SHA      string            `json:"stdout_sha256"`
		Texts    []string          `json:"column_texts"`
		Commands []int             `json:"commands"`
		Inputs   []string          `json:"input_lines"`
		Globals  map[string][]byte `json:"final_globals"`
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
	wantOutput := read("stdout.bin")
	if fmt.Sprintf("%x", sha256.Sum256(wantOutput)) != f.SHA {
		t.Fatal("stdout freeze")
	}
	save := func(n string, v any) {
		if d := os.Getenv(exportEnv); d != "" {
			var b []byte
			switch x := v.(type) {
			case string:
				b = []byte(x)
			case []byte:
				b = x
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
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(string(read(sourceName)), sourceName, nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			asm := "ORG 0x8000\n" + calledMainTestAssembly(t, s)
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			sym := func(n string) uint16 {
				p, ok := a.Symbols[n]
				if !ok {
					t.Fatal("symbol", n)
				}
				return uint16(p)
			}
			save(mode+"/program.a80", asm)
			save(mode+"/program.bin", a.Binary)
			save(mode+"/final.mir", s.MIR2Module.Dump())
			save(mode+"/allocation.json", s.Allocation)
			for _, clobber := range []bool{false, true} {
				prefix := fmt.Sprintf("%s/clobber-%t", mode, clobber)
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				z.Hooks = nil
				mem := z.Memory()
				copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
				copy(mem[0x8000:], a.Binary)
				p := sym("main")
				copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
				want := append([]byte(nil), mem...)
				for name, b := range f.Globals {
					copy(want[sym(name):], b)
				}
				inputIndex := 0
				var inputWant []byte
				if len(f.Inputs) > 0 {
					inputWant = make([]byte, 80)
					for _, line := range f.Inputs {
						inputWant[0] = 78
						inputWant[1] = byte(len(line))
						copy(inputWant[2:], []byte(line))
						inputWant[2+len(line)] = 13
						if len(line) > 0 {
							inputWant[2+len(line)] = 0
						}
					}
					copy(want[sym("_inbuf"):], inputWant)
				}
				host := &sqliteNativeHost{t: t}
				defer func() {
					if host.rows != nil {
						host.rows.Close()
					}
					if host.db != nil {
						host.db.Close()
					}
				}()
				var ports []map[string]any
				z.SetIOHandlers(func(p uint16) byte {
					if byte(p>>8) != z.GetRegisters().A {
						t.Fatal("IN high", p)
					}
					v := host.read(p)
					ports = append(ports, map[string]any{"in": int(p), "value": int(v)})
					return v
				}, func(p uint16, v byte) {
					ports = append(ports, map[string]any{"out": int(p), "value": int(v)})
					host.write(p, v)
				})
				output := []byte{}
				received := 0
				recv := sym("sql__sqlite___sql_recv_str")
				buf := sym("sql__sqlite___sql_buf")
				columnIndex, offset := 0, 0
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
				minSP := uint16(0xf000)
				for n := 0; n < 2000000 && z.GetPC() != 0x7003; n++ {
					pc := z.GetPC()
					if pc == 5 {
						r := z.GetRegisters()
						sp := r.SP
						ret := uint16(mem[sp]) | uint16(mem[sp+1])<<8
						if mem[ret-3] != 0xcd || mem[ret-2] != 5 || mem[ret-1] != 0 {
							t.Fatal("BDOS CALL authority", ret)
						}
						authorized := false
						if len(f.Inputs) == 0 {
							puts := sym("tui__render__tui_puts")
							authorized = ret >= puts && ret < puts+32 && byte(r.BC) == 2
						} else {
							for _, l := range a.Listing {
								if l.Address == int(ret)-3 && len(l.Bytes) == 3 && l.Bytes[0] == 0xcd {
									authorized = true
								}
							}
						}
						if !authorized {
							t.Fatal("BDOS caller authority", ret, r)
						}
						switch byte(r.BC) {
						case 10:
							if inputIndex >= len(f.Inputs) || r.DE != sym("_inbuf") || mem[r.DE] != 78 {
								t.Fatal("input authority", r, inputIndex)
							}
							line := []byte(f.Inputs[inputIndex])
							inputIndex++
							mem[r.DE+1] = byte(len(line))
							copy(mem[r.DE+2:], line)
							mem[int(r.DE)+2+len(line)] = 13
						case 2:
							v := byte(r.DE)
							if len(output) >= len(wantOutput) {
								t.Fatal("extra stdout", v)
							}
							if v != wantOutput[len(output)] {
								save(prefix+"/wrong-stdout.bin", output)
								t.Fatalf("stdout byte%d got%02x want%02x", len(output), v, wantOutput[len(output)])
							}
							output = append(output, v)
						default:
							t.Fatal("BDOS function", r.BC)
						}
						if clobber {
							r.A = 0xa5
							r.F = 0xff
							r.BC = 0x1234
							r.DE = 0x5678
							r.HL = 0x9abc
							r.IX = 0x2468
							r.IY = 0x1357
						}
						r.PC = ret
						r.SP = sp + 2
						z.SetRegisters(r)
						continue
					}
					if pc != 0x7000 && (pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary)) {
						t.Fatal("foreign PC", pc)
					}
					if mem[pc] == 0x77 && pc >= recv && pc < recv+32 {
						r := z.GetRegisters()
						if columnIndex >= len(f.Texts) {
							t.Fatal("extra receive")
						}
						w := append([]byte(f.Texts[columnIndex]), 0)
						if r.HL != buf+uint16(offset) || r.A != w[offset] {
							t.Fatal("receive", columnIndex, offset, r)
						}
						want[r.HL] = r.A
						received++
						offset++
						if offset == len(w) {
							columnIndex++
							offset = 0
						}
					}
					if mem[pc] == 0x76 {
						t.Fatal("unexpected HALT")
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
					if minSP < 0xe000 || z.GetSP() > 0xf000 {
						t.Fatal("stack bounds")
					}
				}
				save(prefix+"/commands.json", host.commands)
				save(prefix+"/ports.json", ports)
				save(prefix+"/stdout.bin", output)
				save(prefix+"/ram.bin", mem)
				save(prefix+"/registers.json", z.GetRegisters())
				if inputIndex != len(f.Inputs) {
					t.Fatal("input count", inputIndex)
				}
				if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 || !bytes.Equal(output, wantOutput) || columnIndex != len(f.Texts) || offset != 0 || len(host.commands) != len(f.Commands) || host.db != nil || host.rows != nil {
					t.Fatal("incomplete main", z.GetRegisters(), columnIndex, len(host.commands))
				}
				for i, c := range host.commands {
					if c["command"].(int) != f.Commands[i] {
						t.Fatal("command order", i, c)
					}
				}
				copy(want[minSP:0xf000], mem[minSP:0xf000])
				for _, l := range a.Listing {
					if strings.HasPrefix(l.Label, "_spill_") {
						copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
					}
				}
				if !bytes.Equal(want, mem) {
					for p := range mem {
						if mem[p] != want[p] {
							t.Fatalf("RAM %04x got%x want%x", p, mem[p], want[p])
						}
					}
				}
				save(prefix+"/summary.json", map[string]any{"result": "PASS", "bytes": len(a.Binary), "T_states": z.GetCycles(), "stdout_bytes": len(output), "received_bytes": received, "full_ram": true, "minimum_sp": minSP, "BDOS_clobber": clobber})
				t.Log(clobber, len(a.Binary), z.GetCycles(), len(output), "original main native PASS")
			}
		})
	}
}
