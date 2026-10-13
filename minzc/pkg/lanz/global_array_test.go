package lanz_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
)

func TestGlobalArrayInitializerByteOrderAndZeroFill(t *testing.T) {
	for _, tc := range []struct {
		ty, init string
		want     []byte
	}{
		{"u8 4", "1 128 255", []byte{1, 128, 255, 0}},
		{"u16 3", "4660 65535", []byte{0x34, 0x12, 0xff, 0xff, 0, 0}},
		{"i16 2", "-2", []byte{0xfe, 0xff, 0, 0}},
	} {
		m, err := lanz.Compile("(global palette ("+tc.ty+") (array "+tc.init+"))", "palette")
		if err != nil {
			t.Fatal(err)
		}
		g := m.Globals[0]
		if _, ok := g.Ty.(*mir2.ArrayTy); !ok {
			t.Fatalf("array became %v", g.Ty)
		}
		if !bytes.Equal(g.Init, tc.want) {
			t.Fatalf("%s init=%x want=%x", tc.ty, g.Init, tc.want)
		}
	}
}

func TestGlobalArrayRejectsInvalidShape(t *testing.T) {
	for _, src := range []string{
		"(global p (u8 0))", "(global p (u8 65537))", "(global p (void 8))", "(global p (banana 8))", "(global p (u8))",
		"(global p (u8 1) (array 1 2))", "(global p (u8 1) (array unknown))", "(global p (u8 1) 7)",
	} {
		_, err := lanz.Compile(src, "badpalette")
		if err == nil || !strings.Contains(err.Error(), "global array") {
			t.Fatalf("%s: expected array diagnostic, got %v", src, err)
		}
	}
}
