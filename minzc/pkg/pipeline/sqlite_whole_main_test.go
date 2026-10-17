package pipeline

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	_ "modernc.org/sqlite"
)

// Test-owned strict protocol adapter backed by actual SQLite. Its expected
// commands and stdout were frozen before execution, independently of compiler.
type sqliteNativeHost struct {
	t                          *testing.T
	db                         *sql.DB
	rows                       *sql.Rows
	vals                       []any
	sent, recv                 []byte
	data, hi, status, lo, high byte
	commands                   []map[string]any
}

func (h *sqliteNativeHost) read(p uint16) byte {
	switch byte(p) {
	case 0x43:
		return h.status
	case 0x45:
		if len(h.recv) > 0 {
			v := h.recv[0]
			h.recv = h.recv[1:]
			return v
		}
		return h.lo
	case 0x47:
		return h.high
	}
	h.t.Fatal("unknown input", p)
	return 0
}
func (h *sqliteNativeHost) write(p uint16, v byte) {
	if byte(p>>8) != v {
		h.t.Fatal("OUT high", p, v)
	}
	switch byte(p) {
	case 0x45:
		h.data = v
		h.sent = append(h.sent, v)
		return
	case 0x47:
		h.hi = v
		return
	case 0x41:
	default:
		h.t.Fatal("unknown output", p)
	}
	e := map[string]any{"command": int(v)}
	text := func() string {
		if len(h.sent) == 0 || h.sent[len(h.sent)-1] != 0 {
			h.t.Fatal("unterminated input", h.sent)
		}
		return string(h.sent[:len(h.sent)-1])
	}
	switch v {
	case 1:
		s := text()
		e["string"] = s
		var err error
		h.db, err = sql.Open("sqlite", s)
		if err != nil {
			h.t.Fatal(err)
		}
		h.db.SetMaxOpenConns(1)
		if err = h.db.Ping(); err != nil {
			h.t.Fatal(err)
		}
		h.status = 1
		e["result"] = 1
	case 2:
		e["handle"] = int(h.data)
		if h.data != 1 {
			h.t.Fatal("db handle", h.data)
		}
		h.db.Close()
		h.db = nil
		h.status = 0
		e["result"] = 0
	case 3, 8:
		s := text()
		e["string"] = s
		e["handle"] = int(h.hi)
		if h.hi != 1 {
			h.t.Fatal("db handle", h.hi)
		}
		if v == 8 {
			if _, err := h.db.Exec(s); err != nil {
				h.t.Fatal(err)
			}
			h.status = 0
			e["result"] = 0
		} else {
			var err error
			h.rows, err = h.db.Query(s)
			if err != nil {
				h.t.Fatal(err)
			}
			h.status = 1
			e["result"] = 1
		}
	case 4:
		e["handle"] = int(h.data)
		if h.data != 1 || h.rows == nil {
			h.t.Fatal("stmt", h.data)
		}
		h.status = 0
		if h.rows.Next() {
			cols, err := h.rows.Columns()
			if err != nil {
				h.t.Fatal(err)
			}
			h.vals = make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range ptrs {
				ptrs[i] = &h.vals[i]
			}
			if err = h.rows.Scan(ptrs...); err != nil {
				h.t.Fatal(err)
			}
			h.status = 1
		}
		e["result"] = int(h.status)
	case 5, 6:
		e["handle"] = int(h.data)
		e["column"] = int(h.hi)
		if h.data != 1 {
			h.t.Fatal("stmt", h.data)
		}
		if v == 5 {
			s := ""
			if int(h.hi) < len(h.vals) {
				s = fmt.Sprint(h.vals[h.hi])
			}
			h.recv = append([]byte(s), 0)
			e["text"] = s
		} else {
			var n int64
			if int(h.hi) < len(h.vals) {
				n, _ = h.vals[h.hi].(int64)
			}
			h.lo = byte(n)
			h.high = byte(n >> 8)
			e["integer"] = n
		}
	case 7:
		e["handle"] = int(h.data)
		if h.data != 1 {
			h.t.Fatal("stmt", h.data)
		}
		h.rows.Close()
		h.rows = nil
		h.status = 0
		e["result"] = 0
	default:
		h.t.Fatal("command", v)
	}
	h.commands = append(h.commands, e)
	h.sent = nil
}
func TestSQLiteOriginalWholeMain(t *testing.T) {
	src, err := os.ReadFile("../../../examples/nanz/sql_test6.nanz")
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := os.ReadFile("../../../reports/sqlite-whole-main/frozen/freeze.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Stdout       string            `json:"stdout"`
		Commands     []map[string]any  `json:"commands"`
		Dependencies map[string]string `json:"source_dependencies"`
	}
	if err = json.Unmarshal(frozen, &oracle); err != nil {
		t.Fatal(err)
	}
	for p, want := range oracle.Dependencies {
		lookup := "../../../" + p
		// Provider code is retained audit evidence; this test executes its own
		// SQLite host, not either CLI provider. Preserve frozen audit hashes
		// while continuing to pin the actual source and library dependencies.
		if p == "minzc/cmd/mze/sqlite_ports.go" || p == "minzc/cmd/mzx/sqlite_ports.go" {
			lookup = "../../../reports/sqlite-whole-main/frozen/" + strings.ReplaceAll(p, "/", "__")
		}
		d, e := os.ReadFile(lookup)
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(d)) != want {
			t.Fatal("freeze", p)
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.ParseWithOpts(string(src), "sql_test6.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			o := Options{ContractOpt: true, AssertMode: "none"}
			if mode != "ordinary" {
				o.UseGrace = true
				o.UseHIRGrace = true
				o.OptSize = mode == "grace-size"
			}
			s, err := CompileHIRSteps(h, o)
			if err != nil {
				t.Fatal(err)
			}
			callerAssembly := calledMainTestAssembly(t, s)
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + callerAssembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatal(err, a.Errors)
			}
			for _, clobber := range []bool{false, true} {
				t.Run(fmt.Sprint(clobber), func(t *testing.T) {
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					z.LoadMemory(0x100, a.Binary)
					main := a.Symbols["main"]
					z.LoadMemory(0x7000, []byte{0xcd, byte(main), byte(main >> 8), 0x76})
					z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, IX: 0x1357, IY: 0x2468})
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
							t.Fatal("IN high A", p, z.GetRegisters().A)
						}
						v := host.read(p)
						ports = append(ports, map[string]any{"direction": "in", "port": int(p), "value": int(v)})
						return v
					}, func(p uint16, v byte) {
						ports = append(ports, map[string]any{"direction": "out", "port": int(p), "value": int(v)})
						host.write(p, v)
					})
					var output []byte
					bdos := func(fn byte, de uint16) (byte, uint16, bool) {
						if fn != 2 {
							t.Fatal("BDOS", fn)
						}
						output = append(output, byte(de))
						if clobber {
							r := z.GetRegisters()
							r.A = 0xa5
							r.F = 0xff
							r.BC = 0x1234
							r.DE = 0x5678
							r.HL = 0x9abc
							r.IX = 0x2468
							r.IY = 0x1357
							z.SetRegisters(r)
						}
						return 0xa5, 0xc3c3, true
					}
					before := append([]byte(nil), z.Memory()...)
					minSP := uint16(0xf000)
					done := false
					var received []byte
					buf := uint16(a.Symbols["sql__sqlite___sql_buf"])
					recv := uint16(a.Symbols["sql__sqlite___sql_recv_str"])
					expectedReceived := []byte("Alice\x00Bob\x00")
					var nativeT int
					for i := 0; i < 1000000; i++ {
						pc := z.GetPC()
						if pc == 0x7003 && z.GetSP() == 0xf000 {
							done = true
							break
						}
						if pc == 5 {
							r := z.GetRegisters()
							sp := r.SP
							ret := uint16(z.Memory()[sp]) | uint16(z.Memory()[sp+1])<<8
							puts := uint16(a.Symbols["tui__render__tui_puts"])
							if ret < puts || ret >= puts+32 || z.Memory()[ret-3] != 0xcd || z.Memory()[ret-2] != 5 || z.Memory()[ret-1] != 0 {
								t.Fatal("BDOS caller authority", ret)
							}
							a, hl, _ := bdos(byte(r.BC), r.DE)
							r = z.GetRegisters()
							r.A = a
							r.HL = hl
							r.PC = uint16(z.Memory()[sp]) | uint16(z.Memory()[sp+1])<<8
							r.SP = sp + 2
							z.SetRegisters(r)
							continue
						}
						if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
							t.Fatalf("foreign PC %04x SP %04x stdout %q", pc, z.GetSP(), output)
						}
						if z.Memory()[pc] == 0x77 && pc >= recv && pc < recv+32 {
							r := z.GetRegisters()
							wantOffset := len(received)
							if wantOffset >= 6 {
								wantOffset -= 6
							}
							if len(received) >= len(expectedReceived) || r.HL != buf+uint16(wantOffset) || r.A != expectedReceived[len(received)] {
								t.Fatalf("receive write pc%04x address%04x value%02x index%d", pc, r.HL, r.A, len(received))
							}
							received = append(received, r.A)
						}
						nativeT += z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
					}
					if dir := os.Getenv("MINZ_SQLITE_MAIN_EXPORT"); dir != "" {
						dir = filepath.Join(dir, mode, fmt.Sprint(clobber))
						if err = os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						payload := map[string]any{"stdout": string(output), "commands": host.commands, "done": done, "registers": z.GetRegisters(), "min_sp": minSP, "bytes": len(a.Binary), "native_t_states_excluding_test_bdos": nativeT, "received": received, "ports": ports, "options": o}
						b, _ := json.MarshalIndent(payload, "", "  ")
						os.WriteFile(filepath.Join(dir, "result.json"), b, 0644)
						os.WriteFile(filepath.Join(dir, "main.a80"), []byte("ORG 0x100\n"+callerAssembly), 0644)
						os.WriteFile(filepath.Join(dir, "main.bin"), a.Binary, 0644)
						os.WriteFile(filepath.Join(dir, "before.ram"), before, 0644)
						os.WriteFile(filepath.Join(dir, "after.ram"), z.Memory(), 0644)
						os.WriteFile(filepath.Join(dir, "final.mir"), []byte(s.MIR2Module.Dump()), 0644)
					}
					var statusReads []byte
					for _, e := range ports {
						if e["direction"] == "in" && byte(e["port"].(int)) == 0x43 {
							statusReads = append(statusReads, byte(e["value"].(int)))
						}
					}
					if !bytes.Equal(statusReads, []byte{1, 0, 0, 0, 1, 1, 1, 0, 0}) {
						t.Fatal("status read results", statusReads)
					}
					actual, _ := json.Marshal(host.commands)
					want, _ := json.Marshal(oracle.Commands)
					if !done || string(output) != oracle.Stdout || !bytes.Equal(actual, want) {
						t.Fatalf("main returned=%v stdout=%q commands=%s expected=%s", done, output, actual, want)
					}
					if !bytes.Equal(received, expectedReceived) {
						t.Fatal("receive sequence", received)
					}
					expectedBuffer := []byte("Bob\x00e\x00")
					for i, v := range before {
						if i >= int(minSP) && i < 0xf000 {
							continue
						}
						want := v
						if i >= int(buf) && i < int(buf)+len(expectedBuffer) {
							want = expectedBuffer[i-int(buf)]
						}
						if z.Memory()[i] != want {
							t.Fatalf("RAM address%04x got%02x want%02x", i, z.Memory()[i], want)
						}
					}
					t.Logf("main %d B %d native T excluding host BDOS stdout %d SP %04x minSP %04x", len(a.Binary), nativeT, len(output), z.GetSP(), minSP)
				})
			}
		})
	}
}

// Boundaries exercise the unchanged original library bodies, not new wrappers.
// Returned bytes/word and every memory byte are checked against input snapshots.
func TestSQLiteOriginalBoundedPortAPIs(t *testing.T) {
	src, err := os.ReadFile("../../../examples/nanz/sql_test6.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.ParseWithOpts(string(src), "sql_test6.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			o := Options{ContractOpt: true, AssertMode: "none"}
			if mode != "ordinary" {
				o.UseGrace = true
				o.UseHIRGrace = true
				o.OptSize = mode == "grace-size"
			}
			s, err := CompileHIRSteps(h, o)
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + s.Assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatal(err, a.Errors)
			}
			run := func(name string, arg uint16, col byte, read func(uint16) byte, expectedOut [][2]byte, input []byte, writeAddr uint16, expectedWrite []byte, wantHL *uint16, wantA *byte) {
				t.Helper()
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				z.LoadMemory(0x100, a.Binary)
				target := a.Symbols["sql__sqlite__"+name]
				if target == 0 {
					t.Fatal(name)
				}
				z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
				if input != nil {
					z.LoadMemory(arg, input)
				}
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, HL: arg, A: col, IX: 0x1357, IY: 0x2468})
				before := append([]byte(nil), z.Memory()...)
				var out [][2]byte
				z.SetIOHandlers(func(p uint16) byte {
					if byte(p>>8) != z.GetRegisters().A {
						t.Fatal("IN high A", p, z.GetRegisters().A)
					}
					return read(p)
				}, func(p uint16, v byte) {
					if byte(p>>8) != v {
						t.Fatal("high", p, v)
					}
					out = append(out, [2]byte{byte(p), v})
				})
				done := false
				minSP := uint16(0xf000)
				for i := 0; i < 20000; i++ {
					pc := z.GetPC()
					if pc == 0x7003 && z.GetSP() == 0xf000 {
						done = true
						break
					}
					if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
						t.Fatal("fetch", pc)
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				if !done || fmt.Sprint(out) != fmt.Sprint(expectedOut) {
					t.Fatal(name, "boundary", done, out, expectedOut)
				}
				r := z.GetRegisters()
				if wantHL != nil && r.HL != *wantHL {
					t.Fatal(name, "word", r.HL, *wantHL)
				}
				if wantA != nil && r.A != *wantA {
					t.Fatal(name, "byte", r.A, *wantA)
				}
				if r.IX != 0x1357 || r.IY != 0x2468 {
					t.Fatal("indices", r)
				}
				for p, v := range before {
					if p >= int(minSP) && p < 0xf000 {
						continue
					}
					want := v
					if p >= int(writeAddr) && p < int(writeAddr)+len(expectedWrite) {
						want = expectedWrite[p-int(writeAddr)]
					}
					if z.Memory()[p] != want {
						t.Fatal(name, "RAM", p, z.Memory()[p], want)
					}
				}
			}
			for _, n := range []int{0, 1, 17, 255} {
				text := append(bytes.Repeat([]byte{'q'}, n), 0)
				pos := 0
				addr := uint16(0xc000)
				run("_sql_recv_str", addr, 0, func(p uint16) byte {
					if byte(p) != 0x45 || pos >= len(text) {
						t.Fatal("receive bounds", p, pos)
					}
					v := text[pos]
					pos++
					return v
				}, nil, nil, addr, text, &addr, nil)
				if pos != len(text) {
					t.Fatal("receive count")
				}
				var out [][2]byte
				for _, b := range text {
					out = append(out, [2]byte{0x45, b})
				}
				run("_sql_send_str", addr, 0, func(p uint16) byte { t.Fatal("unexpected read", p); return 0 }, out, text, 0, nil, nil, nil)
			}
			for _, q := range []uint16{1, 255} {
				for _, col := range []byte{0, 1, 255} {
					text := []byte{0}
					if col != 255 {
						text = append(bytes.Repeat([]byte{'Z'}, 255), 0)
					}
					pos := 0
					buf := uint16(a.Symbols["sql__sqlite___sql_buf"])
					run("sqlite_column_text", q, col, func(p uint16) byte {
						if byte(p) != 0x45 || pos >= len(text) {
							t.Fatal("column receive", p, pos)
						}
						v := text[pos]
						pos++
						return v
					}, [][2]byte{{0x45, byte(q)}, {0x47, col}, {0x41, 5}}, nil, buf, text, &buf, nil)
					if pos != len(text) {
						t.Fatal("column length")
					}
				}
			}
			for _, q := range []uint16{1, 255} {
				for _, col := range []byte{0, 1, 255} {
					for _, n := range []int64{0, 1, 255, 256, 32767, 32768, 65535, -1, -32768} {
						want := uint16(n)
						run("sqlite_column_int", q, col, func(p uint16) byte {
							switch byte(p) {
							case 0x45:
								return byte(n)
							case 0x47:
								return byte(n >> 8)
							}
							t.Fatal("input", p)
							return 0
						}, [][2]byte{{0x45, byte(q)}, {0x47, col}, {0x41, 6}}, nil, 0, nil, &want, nil)
					}
				}
			}
			for _, q := range []uint16{1, 255} {
				for _, name := range []string{"sqlite_step", "sqlite_finalize", "sqlite_close"} {
					cmd := byte(4)
					if name == "sqlite_finalize" {
						cmd = 7
					}
					if name == "sqlite_close" {
						cmd = 2
					}
					for _, status := range []byte{0, 1, 255} {
						run(name, q, 0, func(p uint16) byte {
							if byte(p) != 0x43 {
								t.Fatal(p)
							}
							return status
						}, [][2]byte{{0x45, byte(q)}, {0x41, cmd}}, nil, 0, nil, nil, &status)
					}
				}
			}
			t.Log("original send/receive 0,1,17,255 bytes; column int signed/modulo boundaries; handles/columns/status; fullRAM/SP PASS")
		})
	}
}
