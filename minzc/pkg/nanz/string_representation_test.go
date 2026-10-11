package nanz_test

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestLiteralPrintRepresentations(t *testing.T) {
	for _, c := range []struct {
		literal string
		want    []byte
	}{{`"AB"`, []byte("AB")}, {`c"AB"`, []byte("AB")}, {`l"AB"`, []byte("AB")}, {`""`, nil}, {`c""`, nil}, {`l""`, nil}, {`"A\0B"`, []byte{65, 0, 66}}, {`c"A\0B"`, []byte{65}}, {`l"A\0B"`, []byte{65, 0, 66}}} {
		t.Run(c.literal, func(t *testing.T) {
			src := fmt.Sprintf("fun show(x:u16)->u16 {\n @print(%s)\n @print(c\"!\")\n return x+1\n}\n", c.literal)
			h, e := nanz.Parse(src, "print.nanz")
			if e != nil {
				t.Fatal(e)
			}
			m := hir.LowerModule(h)
			if e = mir2.Verify(m); e != nil {
				t.Fatal(e)
			}
			vm := mir2.NewVM(m)
			var out []byte
			vm.Hosts["@mir.io.print.char"] = func(a []mir2.Value) ([]mir2.Value, error) { out = append(out, byte(a[0].I)); return nil, nil }
			vm.Hosts["@mir.io.print.str"] = func(a []mir2.Value) ([]mir2.Value, error) {
				for p := a[0].I; ; p++ {
					b := vm.ReadHeap(p, 1)
					if len(b) == 0 {
						return nil, fmt.Errorf("overread")
					}
					if b[0] == 0 {
						break
					}
					out = append(out, b[0])
				}
				return nil, nil
			}
			r, e := vm.Call("show", []mir2.Value{{I: 4660}})
			if e != nil {
				t.Fatal(e)
			}
			want := append(append([]byte{}, c.want...), byte('!'))
			if !bytes.Equal(out, want) || len(r) != 1 || r[0].I != 4661 {
				t.Fatalf("out%v want%v result%v", out, want, r)
			}
		})
	}
}
func TestLiteralByteLengthAdmission(t *testing.T) {
	for _, c := range []struct {
		prefix string
		n      int
		ok     bool
	}{{"", 255, true}, {"", 256, false}, {"l", 65535, true}, {"l", 65536, false}} {
		_, e := nanz.Parse("fun f()->^u8 { return "+c.prefix+"\""+strings.Repeat("x", c.n)+"\" }", "bounds.nanz")
		if (e == nil) != c.ok {
			t.Fatalf("%s%d err%v", c.prefix, c.n, e)
		}
	}
	_, e := nanz.Parse("fun f()->^u8 { return \""+strings.Repeat("é", 128)+"\" }", "utf8.nanz")
	if e == nil {
		t.Fatal("256 UTF8 bytes accepted")
	}
}
