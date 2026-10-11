package nanz_test

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"io"
	"os"
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

func TestLiteralPrintBuiltinBytesAndHygiene(t *testing.T) {
	source := "global __print_ptr_1:u16=123\nglobal __print_count_2:u16=456\nfun show()->u16 {\n let __print_ptr_3:u16=321\n @print(\"é\\0B\")\n @print(l\"é\")\n @print(c\"!\")\n return __print_ptr_3\n}\n"
	h, e := nanz.Parse(source, "hygiene.nanz")
	if e != nil {
		t.Fatal(e)
	}
	m := hir.LowerModule(h)
	if e = mir2.Verify(m); e != nil {
		t.Fatal(e)
	}
	vm := mir2.NewVM(m)
	old := os.Stdout
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	os.Stdout = w
	result, callErr := vm.Call("show", nil)
	w.Close()
	os.Stdout = old
	output, e := io.ReadAll(r)
	r.Close()
	if e != nil || callErr != nil {
		t.Fatal(e, callErr)
	}
	want := []byte{0xc3, 0xa9, 0, 66, 0xc3, 0xa9, 33}
	if !bytes.Equal(output, want) {
		t.Fatalf("actual builtin bytes %x want%x", output, want)
	}
	if len(result) != 1 || result[0].I != 321 {
		t.Fatal("source local changed", result)
	}
	for n, v := range map[string]uint16{"__print_ptr_1": 123, "__print_count_2": 456} {
		b := vm.ReadHeap(vm.GlobalAddr(n), 2)
		if uint16(b[0])|uint16(b[1])<<8 != v {
			t.Fatal("source global changed", n, b)
		}
	}
}

func TestStringStorageAndPrintRawOptimizedVM(t *testing.T) {
	for _, optimized := range []bool{false, true} {
		t.Run(fmt.Sprint(optimized), func(t *testing.T) {
			source := `global first_s:u8=0
 global first_c:u8=0
 global first_l:u8=0
 fun show()->void {
 let s:^u8="AB"
 let c:^u8=c"AB"
 let l:^u8=l"AB"
 first_s=s^
 first_c=c^
 first_l=l^
 @print("é\0B")
 @print(l"X\0Y")
 @print(c"!")
 }`
			h, e := nanz.Parse(source, "vm_formats.nanz")
			if e != nil {
				t.Fatal(e)
			}
			m := hir.LowerModule(h)
			if optimized {
				h, e = nanz.Parse(source, "vm_formats.nanz")
				if e != nil {
					t.Fatal(e)
				}
				steps, e := pipeline.CompileHIRSteps(h)
				if e != nil {
					t.Fatal(e)
				}
				m = steps.MIR2Module
			}
			vm := mir2.NewVM(m)
			old := os.Stdout
			r, w, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			os.Stdout = w
			_, ce := vm.Call("show", nil)
			w.Close()
			os.Stdout = old
			out, e := io.ReadAll(r)
			r.Close()
			if e != nil || ce != nil {
				t.Fatal(e, ce)
			}
			if !bytes.Equal(out, []byte{0xc3, 0xa9, 0, 66, 88, 0, 89, 33}) {
				t.Fatalf("builtin raw bytes %x", out)
			}
			for n, v := range map[string]byte{"first_s": 2, "first_c": 65, "first_l": 2} {
				b := vm.ReadHeap(vm.GlobalAddr(n), 1)
				if len(b) != 1 || b[0] != v {
					t.Fatalf("actual VMload %s %v", n, b)
				}
			}
		})
	}
}
