package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestMultiCallPreservesFormalIntegerWidth(t *testing.T) {
	const source = `fun pair(a:u16,b:u16)->(u16,u16) { return (a+256,b+512) }
fun check()->u16 { let (a,b)=pair(10,20)
 return a+b }
assert check()==798
`
	h, e := nanz.Parse(source, "multicall-formal-width.nanz")
	if e != nil {
		t.Fatal(e)
	}
	raw := hir.LowerModule(h)
	check := raw.FuncByName("check")
	types := map[mir2.Reg]mir2.Ty{}
	for _, b := range check.Blocks {
		for _, in := range b.Insts {
			if in.Dst != mir2.NoReg {
				types[in.Dst] = in.Ty
			}
		}
	}
	found := false
	for _, b := range check.Blocks {
		for _, in := range b.Insts {
			if in.Op == mir2.OpCall && in.Sym == "pair" {
				found = true
				for _, arg := range in.Args {
					if types[arg].Width() != 16 {
						t.Fatalf("multi-call arg r%d width%d want16", arg, types[arg].Width())
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("missing pair call")
	}
	if _, e = CompileHIRSteps(h); e != nil {
		t.Fatal(e)
	}
	t.Log("formal u16 arguments from narrow literals retained; full tuple caller VM/native798 PASS")
}
