package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestHIRExplicitOverlappingLayoutRoundTrip(t *testing.T) {
	source := "struct @Envelope { head:u8, inner.lo:volatile u8, inner.word:u16, tail:u8 } size=4 offsets=0:1:1:3\nstruct @Value { byte:u8, word:u16 } union\n"
	m, err := hir.ParseHIR(source, "layout")
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		st := m.Structs[0]
		if !st.Fields[1].Volatile || st.Fields[0].Volatile || mir2.ByteWidth(st) != 4 || st.ByteOffset(1) != 1 || st.ByteOffset(2) != 1 || st.ByteOffset(3) != 3 {
			t.Fatalf("wrong overlapping layout: %#v", st)
		}
		union := m.Structs[1]
		if mir2.ByteWidth(union) != 2 || union.ByteOffset(1) != 0 {
			t.Fatal("lost union layout")
		}
		m, err = hir.ParseHIR(m.Dump(), "roundtrip")
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, source := range []string{"struct @S { a:u16 } size=1 offsets=0", "struct @S { a:u8 } size=1 offsets=-1", "struct @S { a:u8 } size=1 offsets=0:0", "struct @S { a:u8 } offsets=0"} {
		if _, err := hir.ParseHIR(source, "invalid"); err == nil {
			t.Fatalf("accepted invalid explicit layout: %s", source)
		}
	}
}
