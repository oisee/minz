package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// A read-modify-write of one bit becomes SET/RES n,(HL) at the store, which
// reads the byte again. Memory written between the load and the store must
// not reach the result.
func TestBitStoreKeepsValueLoadedBeforeInterveningWrite(t *testing.T) {
	for _, c := range []struct {
		name, source string
	}{
		{"store through the same pointer", `global g: u8 = 17
fun f(p: ^u8) -> u8 {
    let c: u8 = p^
    p^ = 64
    p^ = c | 4
    return p^
}
fun t() -> u8 { g = 17
    return f(&g) }
assert t() == 21 via mir2
assert t() == 21 via z80
`},
		{"call that writes the global", `global g: u8 = 17
fun reset() -> void { g = 64 }
fun f(p: ^u8) -> u8 {
    let c: u8 = p^
    reset()
    p^ = c | 4
    return p^
}
fun t() -> u8 { g = 17
    return f(&g) }
assert t() == 21 via mir2
assert t() == 21 via z80
`},
		{"store through an aliasing pointer", `global g: u8 = 17
fun f(p: ^u8, q: ^u8) -> u8 {
    let c: u8 = p^
    q^ = 64
    p^ = c | 4
    return p^
}
fun t() -> u8 { g = 17
    return f(&g, &g) }
assert t() == 21 via mir2
assert t() == 21 via z80
`},
		{"cleared bit", `global g: u8 = 17
fun f(p: ^u8) -> u8 {
    let c: u8 = p^
    p^ = 255
    p^ = c & 254
    return p^
}
fun t() -> u8 { g = 17
    return f(&g) }
assert t() == 16 via mir2
assert t() == 16 via z80
`},
		{"word", `global g: u16 = 17
fun f(p: ^u16) -> u16 {
    let c: u16 = p^
    p^ = 16384
    p^ = c | 256
    return p^
}
fun t() -> u16 { g = 17
    return f(&g) }
assert t() == 273 via mir2
assert t() == 273 via z80
`},
	} {
		t.Run(c.name, func(t *testing.T) {
			hm, err := nanz.Parse(c.source, "bit_store.nanz")
			if err != nil {
				t.Fatal(err)
			}
			hm.AssertStats = &hir.AssertStats{}
			if _, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions()); err != nil {
				t.Fatal(err)
			}
			if *hm.AssertStats != (hir.AssertStats{Executed: 2, Passed: 2}) {
				t.Fatalf("assert stats = %+v", hm.AssertStats)
			}
		})
	}
}

// With nothing but the mask between the load and the store the fusion stays.
func TestBitStoreStillFusesAdjacentReadModifyWrite(t *testing.T) {
	const source = `global g: u8 = 17
fun f(p: ^u8) -> u8 {
    p^ = p^ | 4
    return p^
}
fun t() -> u8 { g = 17
    return f(&g) }
assert t() == 21 via z80
`
	hm, err := nanz.Parse(source, "bit_store.nanz")
	if err != nil {
		t.Fatal(err)
	}
	hm.AssertStats = &hir.AssertStats{}
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if *hm.AssertStats != (hir.AssertStats{Executed: 1, Passed: 1}) {
		t.Fatalf("assert stats = %+v", hm.AssertStats)
	}
	if !strings.Contains(steps.Assembly, "SET 2, (HL)") {
		t.Fatalf("expected SET 2, (HL):\n%s", steps.Assembly)
	}
}
