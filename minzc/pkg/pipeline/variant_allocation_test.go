package pipeline

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
)

func TestVariantAllocationPathRegressionBound(t *testing.T) {
	image := func(body string) *variantImage {
		t.Helper()
		im, err := assembleVariant(Steps{Assembly: "; fun probe(a: u8 = A) -> u8 = A ; clobbers: A, F\nprobe:\n    AND A\n    JRS Z, .zero\n" + body})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := im.score(&mir2.Func{Name: "probe"}); !ok {
			t.Fatal("synthetic score unsupported")
		}
		return im
	}
	// The maximum improves while the cheap arm regresses. Comparing only
	// maxima (or granting an allowance independently to each block) is unsound.
	baseline := image(strings.Repeat("    NOP\n", 12) + "    RET\n.zero:\n    RET\n")
	for _, tc := range []struct {
		name  string
		extra int
		want  bool
	}{
		{"plus12", 3, true},
		{"at16", 4, true},
		{"plus20", 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := image("    RET\n.zero:\n" + strings.Repeat("    NOP\n", tc.extra) + "    RET\n")
			oldT, _ := baseline.score(&mir2.Func{Name: "probe"})
			newT, _ := candidate.score(&mir2.Func{Name: "probe"})
			if newT >= oldT || len(candidate.binary) > len(baseline.binary) {
				t.Fatal("fixture must improve the maximum without growing the image")
			}
			if baseline.nonIncreasingPaths(candidate, "probe") {
				t.Fatal("existing non-increasing rule must still reject a slower arm")
			}
			if got := baseline.boundedAllocationPaths(candidate, "probe"); got != tc.want {
				t.Fatalf("+%dT path: accepted=%t want %t", 4*tc.extra, got, tc.want)
			}
		})
	}
	t.Run("cumulative", func(t *testing.T) {
		a := image("    RET\n.zero:\n    NOP\n.middle:\n    NOP\n    RET\n")
		b := image("    RET\n.zero:\n" + strings.Repeat("    NOP\n", 4) + ".middle:\n" + strings.Repeat("    NOP\n", 4) + "    RET\n")
		if a.boundedAllocationPaths(b, "probe") {
			t.Fatal("two +12T segments exceeded the per-path bound")
		}
	})
	t.Run("positive-cycle", func(t *testing.T) {
		// Assemble directly: this synthetic loop intentionally lacks MIR facts;
		// its difference graph must still refuse cumulative iteration growth.
		makeLoop := func(nops int) *variantImage {
			s := Steps{Assembly: "; fun probe() -> u8 = A ; clobbers: A, F\nprobe:\n.loop:\n" + strings.Repeat("    NOP\n", nops) + "    AND A\n    JRS NZ, .loop\n    RET\n"}
			im, err := assembleVariant(s)
			if err != nil {
				t.Fatal(err)
			}
			return im
		}
		a, b := makeLoop(0), makeLoop(1)
		if a.boundedAllocationPaths(b, "probe") {
			t.Fatal("+4T per iteration can exceed the bound")
		}
		if !b.boundedAllocationPaths(a, "probe") {
			t.Fatal("negative cycle should be bounded")
		}
	})
	t.Run("changed-predicate", func(t *testing.T) {
		b := image("    RET\n.zero:\n    RET\n")
		changed := false
		for pc, n := range b.nodes {
			if len(n.edges) == 2 {
				n.text = strings.Replace(n.text, "Z,", "NZ,", 1)
				b.nodes[pc] = n
				changed = true
			}
		}
		if !changed {
			t.Fatal("missing synthetic branch")
		}
		if baseline.boundedAllocationPaths(b, "probe") {
			t.Fatal("changed branch correspondence accepted")
		}
	})
}

func TestVariantSearchCRC8AllocationVMZ80(t *testing.T) {
	source, err := os.ReadFile("../../../research/c89-benchmark/cases/crc8.c")
	if err != nil {
		t.Fatal(err)
	}
	compile := func(baseline bool) Steps {
		h, err := c89.CompileWithOpts(string(source), "crc8.c", c89.CompileOpts{})
		if err != nil {
			t.Fatal(err)
		}
		o := DefaultOptions()
		o.AssertMode = "none"
		if baseline {
			// Pin the incumbent recipe, independently of the search under test.
			o.FunctionVariants = map[string]FunctionVariant{"execute": {MIRGrace: true}}
		}
		s, err := CompileHIRSteps(h, o)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	before, after := compile(true), compile(false)
	images := [2]*variantImage{}
	for i, s := range []Steps{before, after} {
		images[i], err = assembleVariant(s)
		if err != nil {
			t.Fatal(err)
		}
	}
	var totals [2]int64
	var empty [2]int
	// Exhaust the two byte inputs: seed and n, including every n=0 case.
	// values[i] = seed+i exercises every byte at every index. This finite
	// domain is explicit; it is not an exhaustive enumeration of 256-byte arrays.
	for seed := 0; seed < 256; seed++ {
		values := make([]byte, 256)
		for i := range values {
			values[i] = byte(seed + i)
		}
		var want byte
		q := 0
		for n := 0; n < 256; n++ {
			if n > 0 {
				want ^= values[n-1]
				for bit := 0; bit < 8; bit++ {
					carry := want&128 != 0
					want <<= 1
					if carry {
						want ^= 7
						q++
					}
				}
			}
			var cycles [2]int
			for i, s := range []Steps{before, after} {
				vm := mir2.NewVM(s.MIR2Module)
				vm.WriteHeapSlice(vm.GlobalAddr("values"), values)
				vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(n)})
				vm.WriteHeapSlice(vm.GlobalAddr("result"), []byte{0xef, 0xbe})
				if _, err := vm.Call("execute", nil); err != nil {
					t.Fatalf("VM seed=%d n=%d: %v", seed, n, err)
				}
				vmResult := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))
				im := images[i]
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], im.binary)
				copy(mem[im.symbols["values"]:], values)
				mem[im.symbols["n"]] = byte(n)
				addr := im.symbols["result"]
				mem[addr], mem[addr+1] = 0xef, 0xbe
				mem[0xeffe], mem[0xefff] = 0, 0xf1
				z.SetRegisters(emulator.Registers{PC: uint16(im.symbols["execute"]), SP: 0xeffe})
				for steps := 0; steps < 100000 && z.GetPC() != 0xf100; steps++ {
					z.Step()
				}
				got := binary.LittleEndian.Uint16(mem[addr : addr+2])
				if z.GetPC() != 0xf100 || z.GetSP() != 0xf000 || got != uint16(want) || got != vmResult {
					t.Fatalf("recipe=%d seed=%d n=%d: Z80=%d VM=%d want=%d PC/SP=%04x/%04x", i, seed, n, got, vmResult, want, z.GetPC(), z.GetSP())
				}
				cycles[i] = z.GetCycles()
				totals[i] += int64(cycles[i])
			}
			if n == 0 {
				empty = cycles
			}
			// Defer the speed assertion until the exhaustive correctness sweep
			// has completed, so the pre-change failure records both results.
			if after.VariantDecisions[0].Recipe.SizeAllocation && cycles[0]-cycles[1] != 36*n+16*q-12 {
				t.Fatalf("seed=%d n=%d: saved %dT, want 36n+16q-12=%dT", seed, n, cycles[0]-cycles[1], 36*n+16*q-12)
			}
		}
	}
	t.Logf("65,536 calls per recipe, VM=Z80=oracle; bytes %d->%d; total T %d->%d; n=0 %d->%dT", len(images[0].binary), len(images[1].binary), totals[0], totals[1], empty[0], empty[1])
	if totals[1] >= totals[0] {
		t.Fatalf("speed allocation did not improve CRC8: %dT >= baseline %dT; decisions=%+v", totals[1], totals[0], after.VariantDecisions)
	}
	if !after.VariantDecisions[0].Recipe.SizeAllocation || len(images[1].binary) > len(images[0].binary) {
		t.Fatal("allocation winner missing or image grew")
	}
}
