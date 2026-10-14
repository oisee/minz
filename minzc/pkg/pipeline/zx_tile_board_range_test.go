package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

// Independent oracle: raster coordinates and ascending row events. Every
// legal half-open interval and every inverted/out-of-bounds interval in 0..24
// is exercised at both horizontal boundaries of a ten-column viewport.
func TestZXTileBoardRangeNative(t *testing.T) {
	const source = `import zx.tile_cell
 global first:u8=0
 global limit:u8=0
 global accepted:bool=false
 global seen_count:u8=0
 global seen_rows:[u8;24]
 fun record_row(row:u8) {seen_rows[seen_count]=row seen_count=seen_count+1}
 @zx_tile_board("left") {
 viewport 0 0 10 24
 tiles "tiles"
 attributes "attrs"
 row_done "record_row"
 row_range
 }
 @zx_tile_board("right") {
 viewport 22 0 10 24
 tiles "tiles"
 attributes "attrs"
 row_done "record_row"
 row_range
 }
 fun probe_left() {accepted=left_range(ptr(0xd000),ptr(0xd100),first,limit)}
 fun probe_right() {accepted=right_range(ptr(0xd000),ptr(0xd100),first,limit)}
 `
	for mode, opt := range []Options{{AssertMode: "none", ContractOpt: true}, {AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			h, e := nanz.ParseWithOpts(source, "board-range.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			steps, e := CompileHIRSteps(h, opt)
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(a.Errors) != 0 {
				t.Fatal(e, a.Errors)
			}
			if 0x8000+len(a.Binary) >= 0xd000 {
				t.Fatal("code overlaps oracle data")
			}
			sym := func(name string) uint16 {
				v, ok := a.Symbols[name]
				if !ok {
					t.Fatal(name)
				}
				return uint16(v)
			}
			m, e := spectrum.New48K(make([]byte, 16384))
			if e != nil {
				t.Fatal(e)
			}
			scratch := map[int]bool{}
			reachable := map[string]bool{}
			var visit func(string)
			visit = func(name string) {
				if reachable[name] {
					return
				}
				reachable[name] = true
				if f := steps.MIR2Module.FuncByName(name); f != nil {
					for _, b := range f.Blocks {
						for _, in := range b.Insts {
							if in.Op == mir2.OpCall || in.Op == mir2.OpCallCond {
								visit(in.Sym)
							}
						}
					}
				}
			}
			visit("probe_left")
			visit("probe_right")
			for _, line := range strings.Split(steps.Assembly, "\n") {
				fields := strings.Fields(line)
				if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
					continue
				}
				name := strings.TrimSuffix(fields[0], ":")
				owned := false
				for owner := range reachable {
					if strings.HasPrefix(name, "_spill_"+owner+"_r") {
						owned = true
						break
					}
				}
				if !owned {
					continue
				}
				size := 1
				if fields[1] == "DW" {
					size = 2
				} else if fields[1] != "DB" {
					t.Fatal(line)
				}
				size *= len(strings.Split(strings.Join(fields[2:], ""), ","))
				for i := 0; i < size; i++ {
					scratch[int(sym(name))+i] = true
				}
			}
			intervals := [][2]int{}
			for first := 0; first <= 24; first++ {
				for limit := 0; limit <= 24; limit++ {
					intervals = append(intervals, [2]int{first, limit})
				}
			}
			intervals = append(intervals, [2]int{0, 255}, [2]int{255, 255}, [2]int{255, 0}, [2]int{24, 25})
			for _, x := range []int{0, 22} {
				for _, interval := range intervals {
					first, limit := interval[0], interval[1]
					valid := first <= limit && limit <= 24
					for at := 0x4000; at < 65536; at++ {
						m.Memory.Write(uint16(at), byte(at*37+at/256+11), false)
					}
					for i, b := range a.Binary {
						m.Memory.Write(0x8000+uint16(i), b, false)
					}
					for i := 0; i < 240; i++ {
						tile := byte((i*7 + i/10) % 9)
						m.Memory.Write(0xd000+uint16(i), tile, false)
						m.Memory.Write(0xd100+uint16(i), byte(i*3+5), false)
					}
					m.Memory.Write(sym("first"), byte(first), false)
					m.Memory.Write(sym("limit"), byte(limit), false)
					m.Memory.Write(sym("seen_count"), 0, false)
					for i := 0; i < 24; i++ {
						m.Memory.Write(sym("seen_rows")+uint16(i), 0xee, false)
					}
					m.Memory.Write(0xeff0, 0, false)
					m.Memory.Write(0xeff1, 0x70, false)
					want := make([]byte, 65536)
					for at := range want {
						want[at] = m.Memory.ReadByteInternal(uint16(at))
					}
					want[sym("accepted")] = 0
					if valid {
						want[sym("accepted")] = 1
						want[sym("seen_count")] = byte(limit - first)
						for row := first; row < limit; row++ {
							want[int(sym("seen_rows"))+row-first] = byte(row)
							for col := 0; col < 10; col++ {
								want[0x5800+row*32+x+col] = want[0xd100+row*10+col]
								value := byte(0)
								if want[0xd000+row*10+col] != 0 {
									value = 255
								}
								for scan := row * 8; scan < row*8+8; scan++ {
									address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + x + col
									want[address] = value
								}
							}
						}
					}
					name := "probe_left"
					if x == 22 {
						name = "probe_right"
					}
					m.CPU.SetPC(sym(name))
					m.CPU.SetSP(0xeff0)
					m.CPU.SetIFF1(false)
					m.CPU.SetIFF2(false)
					for count := 0; m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xeff2; count++ {
						if count > 200000 {
							t.Fatalf("x%d [%d,%d) did not return", x, first, limit)
						}
						m.CPU.DoOpcode()
						if m.CPU.SP() < 0xefb0 {
							t.Fatal("range publisher exceeded 64-byte stack")
						}
					}
					if m.CPU.IFF1() || m.CPU.IFF2() {
						t.Fatal("range publisher changed INT")
					}
					for at, value := range want {
						if (at >= 0xefb0 && at < 0xeff0) || scratch[at] {
							continue
						}
						if got := m.Memory.ReadByteInternal(uint16(at)); got != value {
							t.Fatalf("x%d [%d,%d) RAM%04x=%02x want%02x", x, first, limit, at, got, value)
						}
					}
				}
			}
			t.Log("1258 viewport/interval cases: full native RAM, ordered callbacks, bounds admission, PC/SP and disabled INT PASS")
		})
	}
}

func TestZXTileBoardRangeAdmission(t *testing.T) {
	for _, tc := range []struct{ field, tiles, attrs, want string }{
		{"row_range 1", "tiles", "attrs", "takes no arguments"},
		{"row_range\nrow_range", "tiles", "attrs", "duplicate row_range"},
		{"row_range", "_zx_first", "attrs", "reserved range symbol"},
		{"row_range", "tiles", "_zx_limit", "reserved range symbol"},
		{"row_range", "same", "same", "duplicate input symbols"},
	} {
		source := fmt.Sprintf(`import zx.tile_cell
  @zx_tile_board("field") {
  viewport 0 0 10 24
  tiles "%s"
  attributes "%s"
  row_done "row_done"
  %s
  }`, tc.tiles, tc.attrs, tc.field)
		_, err := nanz.ParseWithOpts(source, "invalid-range.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s %s/%s: %v want %q", tc.field, tc.tiles, tc.attrs, err, tc.want)
		}
	}
}
