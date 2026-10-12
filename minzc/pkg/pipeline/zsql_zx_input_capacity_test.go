package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// Boundary inputs execute the complete original readline ASM, including echo.
func TestZSQLZXRealReadlineCapacity(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/zsql_zx_real.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(string(src), "zsql_zx_real.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			sym := func(n string) uint16 {
				v, ok := a.Symbols[n]
				if !ok {
					t.Fatal(n)
				}
				return uint16(v)
			}
			for _, length := range []int{0, 39, 40, 59, 60, 61} {
				t.Run(fmt.Sprintf("length-%d", length), func(t *testing.T) {
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					z.Hooks = nil
					mem := z.Memory()
					copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
					copy(mem[0x8000:], a.Binary)
					for ch := 32; ch < 128; ch++ {
						for line := 0; line < 8; line++ {
							mem[0x3d00+(ch-32)*8+line] = byte((ch*17 + line*29) ^ 0x5a)
						}
					}
					p := sym("readline")
					copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
					want := append([]byte(nil), mem...)
					n := length
					if n > 60 {
						n = 60
					}
					buf := sym("_inbuf")
					for j := 0; j < n; j++ {
						want[buf+uint16(j)] = 'A'
					}
					want[buf+uint16(n)] = 0
					row, col := 0, 0
					for j := 0; j < n; j++ {
						for line := 0; line < 8; line++ {
							addr := 0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col
							want[addr] = byte(('A'*17 + line*29) ^ 0x5a)
						}
						col++
						if col == 32 {
							col = 0
							row++
						}
					}
					row++
					want[sym("_zx_row")] = byte(row)
					want[sym("_zx_col")] = 0
					input := append(bytes.Repeat([]byte{'A'}, length), 13)
					reads := 0
					z.SetIOHandlers(func(p uint16) byte {
						if byte(p) != 0x23 || reads >= len(input) {
							t.Fatal("input", p, reads)
						}
						v := input[reads]
						reads++
						return v
					}, func(p uint16, v byte) { t.Fatal("unexpected output", p, v) })
					z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
					minSP := uint16(0xf000)
					for step := 0; step < 100000 && z.GetPC() != 0x7003; step++ {
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
						if minSP < 0xe000 || z.GetSP() > 0xf000 {
							t.Fatal("stack")
						}
					}
					expectedReads := length + 1
					if length >= 60 {
						expectedReads = 60
					}
					if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 || int(z.GetRegisters().A) != n || reads != expectedReads {
						t.Fatal("return/input", length, z.GetRegisters(), reads)
					}
					copy(want[minSP:0xf000], mem[minSP:0xf000])
					for _, l := range a.Listing {
						if strings.HasPrefix(l.Label, "_spill_") {
							copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
						}
					}
					// A write beyond the source-declared buffer must fail even if the model
					// happens to expect an echoed character there.
					capacity := 40
					for _, g := range h.Globals {
						if g.Name == "_inbuf" {
							if a, ok := g.Ty.(*mir2.ArrayTy); ok {
								capacity = a.Len
							}
						}
					}
					if n >= capacity {
						t.Fatalf("readline length %d writes terminator outside %d-byte source buffer", n, capacity)
					}
					if !bytes.Equal(want, mem) {
						for p := range mem {
							if want[p] != mem[p] {
								t.Fatalf("length %d RAM %04x got %02x want %02x", length, p, mem[p], want[p])
							}
						}
					}
					t.Log(length, "input", n, "accepted", reads, "reads/full RAM PASS")
				})
			}
		})
	}
}
