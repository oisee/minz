package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestFlagLocatedBoolStores(t *testing.T) {
	for _, op := range []string{"==", "!=", "<", ">=", "call"} {
		for _, storage := range []string{"pointer", "global", "field", "array"} {
			t.Run(op+"/"+storage, func(t *testing.T) {
				expr := "i " + op + " 3"
				if op == "call" {
					expr = "predicate(i)"
				}
				target := map[string]string{"pointer": "p^", "global": "f", "field": "flags.value", "array": "items[1]"}[storage]
				source := fmt.Sprintf(`struct Flags { pad:u8 value:bool }
global flags:Flags
global items:[bool;3]
global f:bool=false
global limit:u8=3
fun predicate(i:u8)->bool { return i == limit }
fun store(p:^bool,i:u8) { %s = %s }
fun entry(i:u8)->u8 {
 store(&f,i)
 if %s { return 1 }
 return 0
}
`, target, expr, target)
				if storage == "pointer" {
					source = fmt.Sprintf(`global f:bool=false
global limit:u8=3
fun predicate(i:u8)->bool { return i == limit }
fun store(p:^bool,i:u8) { p^ = %s }
fun entry(i:u8)->u8 { store(&f,i) if f { return 1 } return 0 }
`, expr)
				}
				for _, i := range []int{0, 2, 3, 4, 255} {
					truth := map[string]bool{"==": i == 3, "!=": i != 3, "<": i < 3, ">=": i >= 3, "call": i == 3}[op]
					want := 0
					if truth {
						want = 1
					}
					source += fmt.Sprintf("assert entry(%d) == %d via mir2\nassert entry(%d) == %d via z80\n", i, want, i, want)
				}
				parse := func() *hir.Module {
					h, e := nanz.Parse(source, "flag-store.nanz")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				raw := parse()
				if e := RunAssertsMIR2(raw, hir.LowerModule(raw)); e != nil {
					t.Fatal(e)
				}
				for _, grace := range []bool{false, true} {
					steps, e := CompileHIRSteps(parse(), Options{ContractOpt: true, UseHIRGrace: grace, UseGrace: grace, AssertMode: "all"})
					if e != nil {
						t.Errorf("grace=%v: %v", grace, e)
						continue
					}
					if op == "call" && !strings.Contains(steps.Assembly, "CALL predicate") {
						t.Errorf("grace=%v returned-bool fixture must retain CALL predicate:\n%s", grace, steps.Assembly)
					}
					if e := RunAssertsMIR2(parse(), steps.MIR2Module); e != nil {
						t.Error(e)
					}
				}
			})
		}
	}
}
