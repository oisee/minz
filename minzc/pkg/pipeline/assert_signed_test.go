package pipeline

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/minz/minzc/pkg/mir2llvm"
	"github.com/minz/minzc/pkg/mir2wasm"
	"github.com/minz/minzc/pkg/nanz"
)

func TestAssertSignedReturnDomains(t *testing.T) {
	llvmErr := exec.Command("python3", "-c", "import llvmlite.binding").Run()
	for _, backend := range []string{"mir2", "z80", "wasm", "llvm"} {
		for _, c := range []struct{ name, decl, call, want string }{
			{"negative_byte", "fun entry(a:i8)->i8{return a-10}", "entry(-5)", "-15"},
			{"minimum_byte", "fun entry(a:i8)->i8{return a}", "entry(-128)", "-128"},
			{"positive_byte", "fun entry(a:i8)->i8{return a-10}", "entry(20)", "10"},
			{"negative_word", "fun entry(a:i16)->i16{return a-100}", "entry(-5)", "-105"},
			{"minimum_word", "fun entry(a:i16)->i16{return a}", "entry(-32768)", "-32768"},
			{"literal_word", "fun entry()->i16{return 0-495}", "entry()", "-495"},
			{"positive_word", "fun entry(a:i16)->i16{return a-100}", "entry(595)", "495"},
			{"unsigned_byte", "fun entry(a:u8)->u8{return a}", "entry(255)", "255"},
			{"unsigned_word", "fun entry(a:u16)->u16{return a}", "entry(65535)", "65535"},
			{"tuple", "fun entry(a:i8,b:i16)->(i8,i16,u8,u16){return (a,b,255,65535)}", "entry(-5,-495)", "(-5,-495,255,65535)"},
		} {
			if (backend == "wasm" || backend == "llvm") && c.name == "tuple" {
				continue
			} // These code generators only support one return.
			t.Run(backend+"/"+c.name, func(t *testing.T) {
				if backend == "llvm" && llvmErr != nil {
					t.Skipf("optional llvmlite runtime unavailable: %v", llvmErr)
				}
				source := fmt.Sprintf("%s\nassert %s == %s via %s\n", c.decl, c.call, c.want, "mir2")
				check := func(mutate int, mask int64) error {
					hm, err := nanz.Parse(source, "signed-assert.nanz")
					if err != nil {
						t.Fatal(err)
					}
					if mutate >= 0 {
						a := &hm.Asserts[0]
						if len(a.ExpectedMulti) > 0 {
							a.ExpectedMulti[mutate] ^= mask
						} else {
							a.Expected ^= mask
						}
					}
					for i := range hm.Asserts {
						hm.Asserts[i].Via = backend
					}
					mode := backend
					if backend == "wasm" || backend == "llvm" {
						mode = "none"
					}
					steps, err := CompileHIRSteps(hm, Options{ContractOpt: true, AssertMode: mode})
					if err == nil && backend == "wasm" {
						err = mir2wasm.RunAsserts(hm, steps.MIR2Module, true)
					}
					if err == nil && backend == "llvm" {
						err = mir2llvm.RunAsserts(hm, steps.MIR2Module, true)
					}
					return err
				}
				if err := check(-1, 0); err != nil {
					t.Fatal(err)
				}
				count := 1
				if c.name == "tuple" {
					count = 4
				}
				for i := 0; i < count; i++ {
					for _, mask := range []int64{1, 1 << 32} {
						if err := check(i, mask); err == nil {
							t.Fatalf("wrong expected value passed at element %d with mask %d", i, mask)
						}
					}
				}
			})
		}
	}
}
