package vir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Address oracle follows the declared row-major expression y*5+x. Check stores
// independently of getters, so two matching wrong address computations fail.
func TestVIRIndexedHelpersNativeDomain(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	hm, err := nanz.ParseWithOpts(`global board: [u8;25]
fun board_get(x:u8,y:u8)->u8 { return board[y*5+x] }
fun board_set(x:u8,y:u8,val:u8)->void { board[y*5+x]=val }
`, "indexed_helpers.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(hm)
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	hints := make(map[string]map[int]int)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
		ar := mir2.Allocate(f, mir2.ComputeLiveness(f), mir2.Z80CostTable{})
		hints[f.Name] = make(map[int]int)
		for r, l := range ar.Locs {
			combined.Locs[r] = l
		}
		combined.Spilled = append(combined.Spilled, ar.Spilled...)
		for _, p := range f.Contract.Params {
			hints[f.Name][int(p.Reg)] = vir.Z80.LocByName(ar.Locs[p.Reg].Name)
		}
	}
	asm, rs := vir.CodegenModule(m, vir.SolverOptions{Timeout: 30 * time.Second, FuncParamLocs: hints, PBQPAlloc: combined})
	for _, r := range rs {
		if !r.OK {
			t.Fatalf("codegen: %+v", r)
		}
	}
	ar, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly: %v %v", err, ar.Errors)
	}
	if path := os.Getenv("VIR_HELPER_ASM"); path != "" {
		if err := os.WriteFile(path, []byte(asm), 0644); err != nil {
			t.Fatal(err)
		}
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(uint16(ar.Origin), ar.Binary); err != nil {
		t.Fatal(err)
	}
	board := uint16(ar.Symbols["board"])
	call := func(name string, r emulator.Registers) emulator.Registers {
		t.Helper()
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		r.PC = uint16(ar.Symbols[name])
		r.SP = 0xeffe
		z.SetRegisters(r)
		for i := 0; i < 2000 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		out := z.GetRegisters()
		if out.PC != 0xf100 || out.SP != 0xf000 {
			t.Fatalf("%s return: %+v", name, out)
		}
		return out
	}
	params := func(name string, values ...byte) emulator.Registers {
		var r emulator.Registers
		f := m.FuncByName(name)
		for i, v := range values {
			loc := vir.Z80.Locs[hints[name][int(f.Contract.Params[i].Reg)]].Name
			switch loc {
			case "A":
				r.A = v
			case "B":
				r.BC = r.BC&255 | uint16(v)<<8
			case "C":
				r.BC = r.BC&0xff00 | uint16(v)
			case "D":
				r.DE = r.DE&255 | uint16(v)<<8
			case "E":
				r.DE = r.DE&0xff00 | uint16(v)
			case "H":
				r.HL = r.HL&255 | uint16(v)<<8
			case "L":
				r.HL = r.HL&0xff00 | uint16(v)
			default:
				t.Fatalf("unsupported parameter location %s", loc)
			}
		}
		return r
	}
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			for v := 0; v < 256; v++ {
				seed := make([]byte, 25)
				for i := range seed {
					seed[i] = byte(i + 40)
				}
				before, after := z.GetMemory(board-1), z.GetMemory(board+25)
				if err := z.LoadMemory(board, seed); err != nil {
					t.Fatal(err)
				}
				call("board_set", params("board_set", byte(x), byte(y), byte(v)))
				for i := 0; i < 25; i++ {
					want := seed[i]
					if i == y*5+x {
						want = byte(v)
					}
					if got := z.GetMemory(board + uint16(i)); got != want {
						t.Fatalf("set(%d,%d,%d) offset %d: got %d want %d", x, y, v, i, got, want)
					}
				}
				if z.GetMemory(board-1) != before || z.GetMemory(board+25) != after {
					t.Fatal("store changed adjacent byte")
				}
				got := call("board_get", params("board_get", byte(x), byte(y)))
				if got.A != byte(v) {
					t.Fatalf("get(%d,%d): got %d want %d", x, y, got.A, v)
				}
			}
		}
	}
	t.Log("6400 stores and 6400 reads; all 25 cells, all byte values, adjacent memory and exact PC/SP PASS")
}

// Independent modular arithmetic oracle covers every input through two calls.
func TestVIRChainedCallsFullByteDomain(t *testing.T) {
	var src strings.Builder
	src.WriteString(`fun add(a:u8,b:u8)->u8 { return a+b }
fun mul2(x:u8)->u8 { return x+x }
fun compute(x:u8)->u8 { return mul2(add(x,1)) }
`)
	for x := 0; x < 256; x++ {
		fmt.Fprintf(&src, "assert compute(%d) == %d\n", x, (2*(x+1))&255)
	}
	runVIRAsserts(t, "chained_calls_full_byte", src.String())
}

// The first read remains live through another call whose callee uses IX/IY.
func TestVIRIndexedMixedCallsByteDomain(t *testing.T) {
	var src strings.Builder
	src.WriteString(`global board:[u8;25]
fun board_get(x:u8,y:u8)->u8 { return board[y*5+x] }
fun board_set(x:u8,y:u8,v:u8)->void { board[y*5+x]=v }
fun combine(v:u8)->u8 {
 board_set(0,0,v)
 board_set(3,2,9)
 return board_get(0,0)+board_get(3,2)
}
`)
	for v := 0; v < 256; v++ {
		fmt.Fprintf(&src, "assert combine(%d) == %d\n", v, (v+9)&255)
	}
	runVIRAsserts(t, "indexed_mixed_calls_byte_domain", src.String())
}
