package nanz_test

import (
	"bytes"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestWideGlobalInitializersPreserveBytes(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want []byte
	}{
		{"global value:u32=4294967294", []byte{254, 255, 255, 255}},
		{"global value:u24=1193046", []byte{0x56, 0x34, 0x12}},
		{"global value:[u32;2]=[305419896,4294967295]", []byte{0x78, 0x56, 0x34, 0x12, 255, 255, 255, 255}},
		{"global value:[u24;2]=[1193046,16777215]", []byte{0x56, 0x34, 0x12, 255, 255, 255}},
	} {
		m, e := nanz.Parse(tc.src, "wide_init")
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(m.Globals[0].Init, tc.want) {
			t.Fatalf("%s got%x want%x", tc.src, m.Globals[0].Init, tc.want)
		}
	}
}
