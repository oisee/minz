package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestNanzNestedFieldsWithImportNativeVM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.nanz"), []byte("fun marker()->u8 {return 1}"), 0600); err != nil {
		t.Fatal(err)
	}
	source := `import marker
struct Inner {a:u8,b:u16}
struct Outer {head:u8,inner:Inner,tail:u8}
global item:Outer
fun run()->u8 {
 item.head=0x11
 item.inner.a=0x22
 item.inner.b=0xbeef
 item.tail=0x33
 let data:^u8=ptr(u16(&item))
 if data[0]!=0x11 {return 0}
 if data[1]!=0x22 {return 0}
 if data[2]!=0xef {return 0}
 if data[3]!=0xbe {return 0}
 if data[4]!=0x33 {return 0}
 if item.inner.b!=0xbeef {return 0}
 return 1
}`
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, "nested-import.nanz", nanz.ParseOpts{StdlibDir: dir})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, nil, 1)
		steps, e := CompileHIRSteps(parse(), opt)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(m)
			result, e := vm.Call("run", nil)
			if e != nil || len(result) != 1 || result[0].I != 1 {
				t.Fatal(result, e)
			}
		}
	}
}
