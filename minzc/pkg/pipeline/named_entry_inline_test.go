package pipeline

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Decode linkage metadata as persisted HIR does, then check its propagation
// through lowering, both inliners, pruning and native code generation.
func TestSingleCallNamedEntriesVMZ80(t *testing.T) {
	for _, entry := range []string{"lookup", "lib__runtime__lookup"} {
		t.Run(entry, func(t *testing.T) {
			h := graceMeasureParse(t, strings.ReplaceAll(`
fun lookup(x:u8)->u16 { if x<128 { return ((x as u16)<<8)+(x as u16) } return 65535-(x as u16) }
fun idle(x:u8)->u16 { return ((x as u16)<<8)+(x as u16) }
fun tiny(x:u8)->u8 { return x+1 }
fun wraptiny(x:u8)->u8 { return tiny(x) }
fun wrapper(x:u8)->u16 { return lookup(x)+1 }
`, "lookup", entry))
			for _, name := range []string{entry, "idle", "tiny"} {
				f := h.FuncByName(name)
				f.Internal = true
				// An external name reference must override even explicit private linkage.
				if err := json.Unmarshal([]byte(`{"Private":true,"ReferencedByName":true}`), f); err != nil {
					t.Fatal(err)
				}
			}
			raw := hir.LowerModule(h)
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{entry, "idle", "tiny"} {
				if s.MIR2Module.FuncByName(name) == nil {
					t.Fatalf("named entry %s removed:\n%s", name, s.MIR2Module.Dump())
				}
			}
			for x := int64(0); x < 256; x++ {
				lookup := int64(65535) - x
				if x < 128 {
					lookup = (x << 8) + x
				}
				for name, want := range map[string]int64{entry: lookup, "idle": (x << 8) + x, "wrapper": (lookup + 1) & 65535, "tiny": (x + 1) & 255, "wraptiny": (x + 1) & 255} {
					for _, m := range []*mir2.Module{raw, s.MIR2Module} {
						got, e := mir2.NewVM(m).Call(name, []mir2.Value{{I: x}})
						if e != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("%s(%d): %v %v, want %d", name, x, got, e, want)
						}
					}
					measureGraceZ80Compiled(t, h, s, name, []int64{x}, nil, want, 1000)
				}
			}
		})
	}
}

func TestSingleCallInternalHelperVMZ80(t *testing.T) {
	h := graceMeasureParse(t, `fun provider(x:u8)->u16 { if x<128 { return ((x as u16)<<8)+(x as u16) } return 65535-(x as u16) } fun wrapper(x:u8)->u16 { return provider(x)+1 }`)
	h.FuncByName("provider").Internal = true // compiler runtime does not imply private
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if s.MIR2Module.FuncByName("provider") == nil || !strings.Contains(s.MIR2Module.Dump(), "call @provider") {
		t.Fatalf("unproven private helper was consumed:\n%s", s.MIR2Module.Dump())
	}
	for x := int64(0); x < 256; x++ {
		want := int64(65535) - x
		if x < 128 {
			want = (x << 8) + x
		}
		got, e := mir2.NewVM(s.MIR2Module).Call("provider", []mir2.Value{{I: x}})
		if e != nil || len(got) != 1 || got[0].I != want {
			t.Fatal(x, got, e, want)
		}
		measureGraceZ80Compiled(t, h, s, "provider", []int64{x}, nil, want, 1000)
	}
}
