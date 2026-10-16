package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestCUnionLayoutImportNativeVM(t *testing.T) {
	dir := t.TempDir()
	c := `union U{unsigned char bytes[3];unsigned int word;};struct Envelope{unsigned char head;union U inner;unsigned char tail;};struct Envelope envelope={1,{{2,3,4}},5};struct Envelope records[2]={{1,{{2,3,4}},5},{10,{{20,30,200}},100}};unsigned int read_tail(void){return envelope.tail;}unsigned int touch(struct Envelope value){value.tail=9;return value.tail;}`
	if err := os.WriteFile(filepath.Join(dir, "layout.c"), []byte(c), 0600); err != nil {
		t.Fatal(err)
	}
	source := `import layout {envelope, records, read_tail, touch, Envelope}
global initialized:Envelope=Envelope{tail:11}
fun initialized_tail()->u16{return u16(initialized.tail)}
fun tail()->u16{return u16(envelope.tail)}
fun second()->u16{return u16(records[1].tail)}
fun store()->u16{envelope.tail=7 return read_tail()}
fun copied()->u16{envelope.tail=5 let value:u16=touch(envelope) return u16(envelope.tail)}
fun local()->u16{var e:Envelope=Envelope{tail:9} return u16(e.tail)}
`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "caller.nanz", nanz.ParseOpts{BaseDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	cases := []struct {
		name string
		want int64
	}{{"tail", 5}, {"second", 100}, {"store", 7}, {"local", 9}, {"copied", 5}, {"initialized_tail", 11}}
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			h := parse()
			for _, c := range cases {
				h.Asserts = append(h.Asserts, hir.Assert{FuncName: c.name, Expected: c.want, Via: "z80"})
			}
			steps, err := pipeline.CompileHIRSteps(h, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
				vm := mir2.NewVM(m)
				for _, c := range cases {
					got, err := vm.Call(c.name, nil)
					if err != nil || len(got) != 1 || got[0].I != c.want {
						t.Fatalf("%s got=%v err=%v want=%d", c.name, got, err, c.want)
					}
				}
			}
		})
	}
}
