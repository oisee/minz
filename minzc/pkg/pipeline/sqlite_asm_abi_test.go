package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
)

// Use the complete original consumer and real allocated entry ABI. No VM ASM
// oracle: compare the assembled bridge's ordered output to the supplied SQL.
func TestSQLiteASMInputABI(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/sql_test6.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(string(src), "sql_test6.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			o := Options{ContractOpt: true, AssertMode: "none"}
			if mode != "ordinary" {
				o.UseGrace = true
				o.UseHIRGrace = true
				o.OptSize = mode == "grace-size"
			}
			s, e := CompileHIRSteps(h, o)
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + s.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			for _, name := range []string{"sql__sqlite__sqlite_exec", "sql__sqlite__sqlite_query"} {
				f := s.MIR2Module.FuncByName(name)
				if f == nil {
					t.Fatal(name)
				}
				if s.Allocation.Locs[f.Contract.Params[0].Reg].Name != "HL" || s.Allocation.Locs[f.Contract.Params[1].Reg].Name != "DE" {
					t.Fatal("actual formal ABI", name)
				}
				for _, n := range []int{0, 1, 17, 255} {
					for _, db := range []uint16{1, 255, 0x1234} {
						z := emulator.NewRemogattoZ80WithScreen()
						z.SetROMEnd(0)
						z.LoadMemory(0x4000, a.Binary)
						text := append(bytes.Repeat([]byte{'x'}, n), 0)
						z.LoadMemory(0xc000, text)
						target := a.Symbols[name]
						z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
						z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, HL: db, DE: 0xc000, IX: 0x1357, IY: 0x2468})
						before := append([]byte(nil), z.Memory()...)
						var sent, commands, handles []byte
						z.SetIOHandlers(func(p uint16) byte {
							if byte(p) != 0x43 {
								t.Fatal("unexpected input", p)
							}
							return 0x2a
						}, func(p uint16, v byte) {
							if byte(p>>8) != v {
								t.Fatal("port high", p, v)
							}
							switch byte(p) {
							case 0x45:
								sent = append(sent, v)
							case 0x41:
								commands = append(commands, v)
							case 0x47:
								handles = append(handles, v)
							default:
								t.Fatal("port", p)
							}
						})
						done := false
						minSP := uint16(0xf000)
						for i := 0; i < 10000; i++ {
							if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							pc := z.GetPC()
							if pc != 0x7000 && (pc < 0x4000 || int(pc) >= 0x4000+len(a.Binary)) {
								t.Fatal("foreign fetch", pc)
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						command := byte(8)
						if name == "sql__sqlite__sqlite_query" {
							command = 3
						}
						if !done || !bytes.Equal(sent, text) || !bytes.Equal(commands, []byte{command}) || !bytes.Equal(handles, []byte{byte(db)}) {
							t.Fatal("bridge effects", name, n, db, done, sent, commands, handles)
						}
						rr := z.GetRegisters()
						if rr.IX != 0x1357 || rr.IY != 0x2468 {
							t.Fatal("indices")
						}
						if command == 3 {
							if rr.HL != 0x2a {
								t.Fatal("query return", rr)
							}
						} else if rr.A != 0x2a {
							t.Fatal("exec return", rr)
						}
						for p, v := range before {
							if p >= int(minSP) && p < 0xf000 {
								continue
							}
							if z.Memory()[p] != v {
								t.Fatal("RAM", p)
							}
						}
					}
				}
			}
		})
	}
}
