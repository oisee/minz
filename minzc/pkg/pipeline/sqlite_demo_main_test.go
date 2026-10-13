package pipeline

import (
	"bytes"
	"database/sql"
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
	_ "modernc.org/sqlite"
)

type sqliteDemoStatement struct {
	rows   *sql.Rows
	values []any
}
type sqliteDemoHosts struct {
	t          *testing.T
	db         *sql.DB
	statements map[uint16]*sqliteDemoStatement
	next       uint16
	events     []map[string]any
	readString func(uint16) string
}

func (h *sqliteDemoHosts) call(name string, args []uint16) uint16 {
	e := map[string]any{"function": name, "args": append([]uint16(nil), args...)}
	var result uint16
	switch name {
	case "sqlite_open":
		if len(args) != 1 || args[0] != 0 {
			h.t.Fatal("open null pointer contract", args)
		}
		var err error
		h.db, err = sql.Open("sqlite", ":memory:")
		if err != nil {
			h.t.Fatal(err)
		}
		h.db.SetMaxOpenConns(1)
		if err = h.db.Ping(); err != nil {
			h.t.Fatal(err)
		}
		result = 1
	case "sqlite_exec", "sqlite_query":
		if len(args) != 2 || args[0] != 1 {
			h.t.Fatal("db handle", args)
		}
		s := h.readString(args[1])
		e["sql"] = s
		e["args"] = args[:1]
		if name == "sqlite_exec" {
			if _, err := h.db.Exec(s); err != nil {
				h.t.Fatal(err)
			}
		} else {
			rows, err := h.db.Query(s)
			if err != nil {
				h.t.Fatal(err)
			}
			h.next++
			h.statements[h.next] = &sqliteDemoStatement{rows: rows}
			result = h.next
		}
	case "sqlite_step":
		st := h.statements[args[0]]
		if st == nil {
			h.t.Fatal("step invalid", args)
		}
		if st.rows.Next() {
			cols, err := st.rows.Columns()
			if err != nil {
				h.t.Fatal(err)
			}
			st.values = make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range ptrs {
				ptrs[i] = &st.values[i]
			}
			if err = st.rows.Scan(ptrs...); err != nil {
				h.t.Fatal(err)
			}
			result = 1
		} else {
			if err := st.rows.Err(); err != nil {
				h.t.Fatal(err)
			}
			st.rows.Close()
			delete(h.statements, args[0])
		}
	case "sqlite_column_int":
		st := h.statements[args[0]]
		if st == nil || int(args[1]) >= len(st.values) {
			h.t.Fatal("column bounds", args)
		}
		v, ok := st.values[args[1]].(int64)
		if !ok {
			h.t.Fatal("integer value", st.values)
		}
		result = uint16(v)
	case "sqlite_finalize":
		if st := h.statements[args[0]]; st != nil {
			st.rows.Close()
			delete(h.statements, args[0])
		} else {
			h.t.Fatal("finalize invalid", args)
		}
	case "sqlite_close":
		if args[0] != 1 || len(h.statements) != 0 {
			h.t.Fatal("close active", args)
		}
		h.db.Close()
		h.db = nil
	default:
		h.t.Fatal("unknown host", name)
	}
	e["result"] = result
	h.events = append(h.events, e)
	return result
}
func (h *sqliteDemoHosts) cleanup() {
	for _, s := range h.statements {
		s.rows.Close()
	}
	if h.db != nil {
		h.db.Close()
	}
}

func TestOriginalSQLiteDemoWholeMain(t *testing.T) {
	const root = "../../../reports/sqlite-demo-whole-main/"
	src, err := os.ReadFile("../../../examples/nanz/sqlite_demo.nanz")
	if err != nil {
		t.Fatal(err)
	}
	frozen, _ := os.ReadFile(root + "frozen/source.nanz")
	if !bytes.Equal(src, frozen) {
		t.Fatal("source changed")
	}
	var oracle struct {
		Stdout string           `json:"stdout"`
		Events []map[string]any `json:"events"`
	}
	fb, _ := os.ReadFile(root + "frozen/freeze.json")
	json.Unmarshal(fb, &oracle)
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "sqlite_demo.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	names := []string{"sqlite_open", "sqlite_close", "sqlite_exec", "sqlite_query", "sqlite_step", "sqlite_finalize", "sqlite_column_int"}
	compare := func(t *testing.T, host *sqliteDemoHosts, stdout string) {
		t.Helper()
		a, _ := json.Marshal(host.events)
		b, _ := json.Marshal(oracle.Events)
		if stdout != oracle.Stdout || !bytes.Equal(a, b) {
			t.Fatalf("SQL independent effects stdout%q want%q events%s want%s", stdout, oracle.Stdout, a, b)
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			o := Options{ContractOpt: true, AssertMode: "none"}
			if mode != "ordinary" {
				o.UseGrace = true
				o.UseHIRGrace = true
				o.OptSize = mode == "grace-size"
			}
			s, e := CompileHIRSteps(parse(), o)
			if e != nil {
				t.Fatal(e)
			}
			save := func(name string, d []byte) {
				if dir := os.Getenv("MINZ_SQLITE_DEMO_EXPORT"); dir != "" {
					p := filepath.Join(dir, mode, name)
					if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(p, d, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, kind := range []string{"raw", "final"} {
				t.Run(kind, func(t *testing.T) {
					m := s.MIR2Module
					if kind == "raw" {
						m = hir.LowerModule(parse())
					}
					vm := mir2.NewVM(m)
					host := &sqliteDemoHosts{t: t, statements: map[uint16]*sqliteDemoStatement{}}
					defer host.cleanup()
					host.readString = func(ptr uint16) string {
						var b []byte
						for i := 0; i < 4096; i++ {
							d := vm.ReadHeap(int64(ptr)+int64(i), 1)
							if len(d) != 1 {
								t.Fatal("VM SQL read", ptr, i)
							}
							if d[0] == 0 {
								return string(b)
							}
							b = append(b, d[0])
						}
						t.Fatal("unboundedCString")
						return ""
					}
					for _, name := range names {
						name := name
						vm.Hosts[name] = func(a []mir2.Value) ([]mir2.Value, error) {
							args := make([]uint16, len(a))
							for i := range a {
								args[i] = uint16(a[i].I)
							}
							return []mir2.Value{{I: int64(host.call(name, args))}}, nil
						}
					}
					var out bytes.Buffer
					vm.Hosts["@mir.io.print.u8"] = func(a []mir2.Value) ([]mir2.Value, error) { fmt.Fprintf(&out, "%d", byte(a[0].I)); return nil, nil }
					var events []mir2.VMDataAccess
					vm.ObserveDataAccess = func(v mir2.VMDataAccess) { events = append(events, v) }
					_, e := vm.Call("main", nil)
					b, _ := json.MarshalIndent(host.events, "", "  ")
					save(kind+"-events.json", b)
					save(kind+"-stdout.txt", out.Bytes())
					save(kind+".mir", []byte(m.Dump()))
					if e != nil {
						t.Fatal(e)
					}
					for _, v := range events {
						if v.Op == mir2.OpStore {
							t.Fatal("unexpectedVMstore", v)
						}
					}
					compare(t, host, out.String())
				})
			}
			linked := s.Assembly
			for _, name := range names {
				label := name + ":\n"
				if strings.Count(linked, label) != 1 {
					t.Fatal("extern symbol", name)
				}
				linked = strings.Replace(linked, label, label+" RET\n", 1)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n" + linked)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x400, a.Binary)
			entry := a.Symbols["main"]
			z.LoadMemory(0x7000, []byte{0xcd, byte(entry), byte(entry >> 8), 0x76})
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, IX: 0x1357, IY: 0x2468})
			before := append([]byte(nil), z.Memory()...)
			host := &sqliteDemoHosts{t: t, statements: map[uint16]*sqliteDemoStatement{}}
			defer host.cleanup()
			host.readString = func(ptr uint16) string {
				var b []byte
				for i := 0; i < 4096; i++ {
					p := int(ptr) + i
					if p < 0x400 || p >= 0x400+len(a.Binary) {
						t.Fatal("nativeSQLpointer", ptr, i)
					}
					if z.Memory()[p] == 0 {
						return string(b)
					}
					b = append(b, z.Memory()[p])
				}
				t.Fatal("unboundedCString")
				return ""
			}
			read := func(name string, index int) uint16 {
				p := s.MIR2Module.FuncByName(name).Contract.Params[index]
				loc := assertParamLocation(p, s.Allocation)
				r := z.GetRegisters()
				switch loc.Name {
				case "HL":
					return r.HL
				case "DE":
					return r.DE
				case "BC":
					return r.BC
				case "A":
					return uint16(r.A)
				case "C":
					return r.BC & 255
				case "B":
					return r.BC >> 8
				}
				t.Fatal("unknownformalhome", name, index, loc)
				return 0
			}
			var out bytes.Buffer
			z.SetIOHandlers(func(p uint16) byte { t.Fatal("unexpectedIN", p); return 0 }, func(p uint16, v byte) {
				if byte(p) != 0x23 || byte(p>>8) != v {
					t.Fatal("stdoutport", p, v)
				}
				out.WriteByte(v)
			})
			minSP := uint16(0xf000)
			done := false
			for i := 0; i < 100000; i++ {
				pc := z.GetPC()
				if pc == 0x7003 && z.GetSP() == 0xf000 {
					done = true
					break
				}
				if z.IsHalted() {
					if z.GetSP() != 0xeffe {
						t.Fatal("HALT SP", z.GetSP())
					}
					done = true
					break
				}
				handled := false
				for _, name := range names {
					if pc != uint16(a.Symbols[name]) {
						continue
					}
					args := make([]uint16, len(s.MIR2Module.FuncByName(name).Contract.Params))
					for j := range args {
						args[j] = read(name, j)
					}
					r0 := z.GetRegisters()
					ret := uint16(z.Memory()[r0.SP]) | uint16(z.Memory()[r0.SP+1])<<8
					tailClose := name == "sqlite_close" && ret == 0x7003
					if !tailClose && (int(ret) < 0x400 || int(ret) >= 0x400+len(a.Binary) || z.Memory()[ret-3] != 0xcd || uint16(z.Memory()[ret-2])|uint16(z.Memory()[ret-1])<<8 != pc) {
						t.Fatal("host caller authority", name, ret)
					}
					v := host.call(name, args)
					r := z.GetRegisters()
					if mir2.ByteWidth(s.MIR2Module.FuncByName(name).Contract.Returns[0].Ty) == 1 {
						r.A = byte(v)
					} else {
						r.HL = v
					}
					r.PC = uint16(z.Memory()[r.SP]) | uint16(z.Memory()[r.SP+1])<<8
					r.SP += 2
					z.SetRegisters(r)
					handled = true
					break
				}
				if handled {
					continue
				}
				if pc != 0x7000 && (pc < 0x400 || int(pc) >= 0x400+len(a.Binary)) {
					t.Fatal("foreignfetch", pc)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			b, _ := json.MarshalIndent(host.events, "", "  ")
			save("native-events.json", b)
			meta, _ := json.MarshalIndent(map[string]any{"options": o, "allocation": s.Allocation, "functions": s.MIR2Module.Funcs, "min_sp": minSP, "registers": z.GetRegisters(), "cycles_excluding_host": z.GetCycles(), "bytes": len(a.Binary)}, "", "  ")
			save("context.json", meta)
			save("native-stdout.txt", out.Bytes())
			save("native.a80", []byte("ORG 0x400\n"+linked))
			save("native.bin", a.Binary)
			save("native-before.ram", before)
			save("native-after.ram", z.Memory())
			if !done {
				t.Fatal("notterminated", z.GetRegisters())
			}
			compare(t, host, out.String())
			spills := map[int]bool{}
			for _, l := range a.Listing {
				if strings.HasPrefix(strings.ToLower(l.Label), "_spill_") {
					for i := range l.Bytes {
						spills[l.Address+i] = true
					}
				}
			}
			for p, v := range before {
				if p >= int(minSP) && p < 0xf000 || spills[p] {
					continue
				}
				if z.Memory()[p] != v {
					t.Fatal("nativeRAM", p, z.Memory()[p], v)
				}
			}
			t.Logf("original SQLite demo %d B 12-byte stdout, SQL effects/fullRAM/SP PASS", len(a.Binary))
		})
	}
}
