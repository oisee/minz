package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestForEachWideCountSourceVMZ80(t *testing.T) {
	const base = 0x4000
	data := make([]byte, 300)
	memory := make(map[uint16]byte, len(data))
	for i := range data {
		data[i] = byte(i%3 + 1)
		memory[uint16(base+i)] = data[i]
	}
	for _, control := range []string{"count", "continue", "break"} {
		body := "s = s + 1"
		switch control {
		case "continue":
			body = "if x == 2 { continue }\n s = s + 1"
		case "break":
			// Cross the byte boundary before breaking.
			body = "if s == 270 { break }\n s = s + 1"
		}
		source := fmt.Sprintf(`fun count(ptr: ^u8, n: u16) -> u16 {
			var s: u16 = 0
			for x: u8 in ptr[0..n] { %s }
			return s
		}`, body)
		for _, n := range []int{0, 1, 255, 256, 300} {
			t.Run(fmt.Sprintf("%s/n%d", control, n), func(t *testing.T) {
				// Compute the oracle independently of either compiler execution.
				want := int64(0)
				for _, x := range data[:n] {
					if control == "continue" && x == 2 {
						continue
					}
					if control == "break" && want == 270 {
						break
					}
					want++
				}
				parse := func(t *testing.T) *hir.Module {
					t.Helper()
					h, err := nanz.Parse(source, "foreach_wide_count.nanz")
					if err != nil {
						t.Fatal(err)
					}
					return h
				}
				t.Run("source_vm", func(t *testing.T) {
					m := hir.LowerModule(parse(t))
					if err := mir2.Verify(m); err != nil {
						t.Fatal(err)
					}
					vm := mir2.NewVM(m)
					vm.EnsureHeap(int64(base + len(data)))
					vm.WriteHeapSlice(base, data)
					got, err := vm.Call("count", []mir2.Value{{I: base}, {I: int64(n)}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("got %v err=%v, want %d", got, err, want)
					}
				})
				t.Run("assembled_z80", func(t *testing.T) {
					measureProfileZ80(t, parse(t), "count", Options{AssertMode: "none"}, []int64{base, int64(n)}, memory, want, "")
				})
			})
		}
	}
}
