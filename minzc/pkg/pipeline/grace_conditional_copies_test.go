package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestGraceConditionalCopiesStayInlineNativeVM(t *testing.T) {
	const source = `typedef unsigned char u8;
 u8 choose_low(u8 a,u8 b) { return a<b?a:b; }
 u8 choose_high(u8 a,u8 b) { return a>b?a:b; }
 u8 bounded(u8 x,u8 lo,u8 hi) { return x<lo?lo:(x>hi?hi:x); }
 `
	for _, backend := range []string{"mir2", "z80"} {
		input := source
		for _, a := range []int{0, 1, 127, 255} {
			for _, b := range []int{0, 1, 128, 255} {
				input += fmt.Sprintf("// assert choose_low(%d,%d)==%d via %s\n", a, b, min(a, b), backend)
				input += fmt.Sprintf("// assert choose_high(%d,%d)==%d via %s\n", a, b, max(a, b), backend)
			}
			input += fmt.Sprintf("// assert bounded(%d,1,128)==%d via %s\n", a, max(1, min(128, a)), backend)
		}
		sizes := map[bool]int{}
		for _, grace := range []bool{false, true} {
			h, err := c89.Compile(input, "inline-copies.c")
			if err != nil {
				t.Fatal(err)
			}
			h.AssertStats = &hir.AssertStats{}
			steps, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: backend})
			if err != nil {
				t.Fatal(err)
			}
			if h.AssertStats.Executed != 36 || h.AssertStats.Passed != 36 {
				t.Fatal(h.AssertStats)
			}
			if strings.Contains(steps.Assembly, "_trmp") {
				t.Fatalf("Grace=%v moved conditional copies into trampolines", grace)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(a.Errors) != 0 {
				t.Fatalf("assembly: %v %v", err, a.Errors)
			}
			sizes[grace] = len(a.Binary)
		}
		if sizes[true] > sizes[false] {
			t.Fatalf("conditional copies grew: %d -> %d", sizes[false], sizes[true])
		}
	}
}
