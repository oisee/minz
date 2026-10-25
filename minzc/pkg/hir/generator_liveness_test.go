package hir_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestGeneratorFrameDefinitionLiveness(t *testing.T) {
	m, err := nanz.Parse(`
gen phases(n:u8) -> u8 {
    var i:u8 = 0
    while true {
        let before:u8 = i
        let condition:bool = before < n
        if !condition { break }
        let value:u8 = i + 7
        yield value
        let next_value:u8 = i + 1
        i = next_value
    }
    i = 0
    while i < n {
        let second:u8 = i + 17
        yield second
        i = i + 1
    }
}
fun drain(n:u8) -> u16 {
    var sum:u16 = 0
    for value in phases(n) { sum = sum + u16(value) }
    return sum
}
gen overwritten(gate:u8) -> u8 {
    var stale:u8 = 99
    yield 1
    if gate == 0 { stale = 7 } else { stale = 9 }
    yield stale
}
fun branch(gate:u8) -> u16 {
    var sum:u16 = 0
    for value in overwritten(gate) { sum = sum + u16(value) }
    return sum
}
global gate:u8 = 1
gen continued() -> u8 {
    var keep:u8 = 42
    var i:u8 = 0
    while i < 4 {
        i = i + 1
        if gate != 0 { yield keep
            continue }
        keep = keep + 1
        yield keep
    }
}
fun changing_gate() -> u16 {
    gate = 1
    var sum:u16 = 0
    for value in continued() { sum = sum + u16(value)
        gate = 0 }
    return sum
}
`, "generator-liveness.nanz")
	if err != nil {
		t.Fatal(err)
	}
	expanded := hir.CloneModule(m)
	if err := hir.ExpandGenerators(expanded); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"phases$frame": 3, "overwritten$frame": 2, "continued$frame": 3} {
		found := false
		for _, st := range expanded.Structs {
			if st.Name == name {
				found = true
				if got := mir2.ByteWidth(st); got != want {
					t.Errorf("%s frame %d bytes, want %d: %v", name, got, want, st.Fields)
				}
			}
		}
		if !found {
			t.Errorf("missing %s", name)
		}
	}
	if t.Failed() {
		return
	}
	m.NonNullObjects = true
	for _, n := range []int64{0, 1, 3, 8, 255} {
		var want uint16
		for i := int64(0); i < n; i++ {
			want += uint16(uint8(i+7)) + uint16(uint8(i+17))
		}
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "drain", Args: []int64{n}, Expected: int64(want), Source: "two-phase ANF frame"})
	}
	m.Asserts = append(m.Asserts, hir.Assert{FuncName: "branch", Args: []int64{0}, Expected: 8}, hir.Assert{FuncName: "branch", Args: []int64{1}, Expected: 10}, hir.Assert{FuncName: "changing_gate", Expected: 174})
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
