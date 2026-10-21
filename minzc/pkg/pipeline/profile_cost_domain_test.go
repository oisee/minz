package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

// Validate measured zero-trip and positive-trip models over every u8 length, both admitted addresses
// and the last valid address range that intentionally takes the guard fallback.
func TestSumProfileCostFullU8Domain(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		h, err := nanz.Parse(string(source), "examples/nanz/01_sum_array.nanz")
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	type row struct {
		N        int    `json:"n"`
		Base     int64  `json:"base"`
		Path     string `json:"path"`
		Original int    `json:"original_t_states"`
		Pointer  int    `json:"pointer_t_states"`
		Result   int64  `json:"result"`
	}
	rows := []row{}
	for _, path := range []string{"fast", "last-fast", "fallback", "wrap"} {
		for n := 0; n < 256; n++ {
			if (path == "fallback" || path == "wrap") && n == 0 {
				continue
			} // guard always admits n=0
			base := int64(0x4000)
			if path == "last-fast" {
				base = 65535 - int64(n)
			}
			if path == "fallback" {
				base = 65536 - int64(n)
			}
			if path == "wrap" {
				base = 65535
			}
			mem := map[uint16]byte{}
			want := int64(0)
			for i := 0; i < n; i++ {
				v := byte(i*37 + 11)
				mem[uint16(base+int64(i))] = v
				want = (want + int64(v)) & 255
			}
			measured := []int{}
			for _, pointer := range []bool{false, true} {
				h := parse()
				if pointer {
					if _, err := hir.RunGrace(h, 0); err != nil {
						t.Fatal(err)
					}
				}
				raw := hir.LowerModule(h)
				vm := mir2.NewVM(raw)
				vm.EnsureHeap(65536)
				for addr, v := range mem {
					vm.WriteHeap(int64(addr), v)
				}
				result, err := vm.Call("sum_array", []mir2.Value{{I: base}, {I: int64(n)}})
				if err != nil || len(result) != 1 || result[0].I != want {
					t.Fatalf("%s n=%d pointer=%v VM=%v err=%v", path, n, pointer, result, err)
				}
				for addr, v := range mem {
					if vm.ReadHeap(int64(addr), 1)[0] != v {
						t.Fatal("VM changed input memory")
					}
				}
				cycles, _ := measureProfileZ80(t, h, "sum_array", Options{AssertMode: "none"}, []int64{base, int64(n)}, mem, want, "")
				measured = append(measured, cycles)
			}
			if measured[0] != 41+54*n {
				t.Fatalf("original model n=%d got=%d", n, measured[0])
			}
			expectedFast := 241 + 37*n
			if n == 0 {
				expectedFast = 232
			}
			if path == "last-fast" {
				expectedFast += 7
			}
			if (path == "fast" || path == "last-fast") && measured[1] != expectedFast {
				t.Fatalf("fast model n=%d got=%d", n, measured[1])
			}
			if (path == "fallback" || path == "wrap") && measured[1] != 258+178*n {
				t.Fatalf("fallback model n=%d got=%d", n, measured[1])
			}
			rows = append(rows, row{N: n, Base: base, Path: path, Original: measured[0], Pointer: measured[1], Result: want})
			if path == "fallback" && (n == 1 || n == 2 || n == 255) {
				t.Logf("fallback n=%d original=%d pointer=%d overhead=%d", n, measured[0], measured[1], measured[1]-measured[0])
			}
		}
	}
	if directory := os.Getenv("MINZ_PGO_ARTIFACT_DIR"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "full-u8-cost-domain.json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(fmt.Sprintf("Verified %d address/length scenarios on original and pointer VM + assembled Z80", len(rows)))
}
