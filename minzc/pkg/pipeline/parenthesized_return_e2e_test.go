package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestParenthesizedReturnOptInlineZ80(t *testing.T) {
	for _, expr := range []string{"(b+b)-(a+a)", "2-(a+a)", "2-(a<<1)"} {
		for _, tail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/tail=%v", expr, tail), func(t *testing.T) {
				body := "return twice_diff(a,1)"
				if !tail {
					body = "let result:u16=twice_diff(a,1) return result"
				}
				src := fmt.Sprintf("fun twice_diff(a:u16,b:u16)->u16{return %s}\nfun entry(a:u16)->u16{%s}\n", expr, body)
				for _, v := range []uint16{0, 1, 2, 32767, 32768, 43418, 65535} {
					want := uint16(2 - 2*uint32(v))
					src += fmt.Sprintf("assert entry(%d) == %d via mir2\nassert entry(%d) == %d via z80\n", v, want, v, want)
				}
				parse := func() *hir.Module {
					h, e := nanz.Parse(src, "return.nanz")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				raw := parse()
				if err := RunAssertsMIR2(raw, hir.LowerModule(raw)); err != nil {
					t.Errorf("raw VM: %v", err)
				}
				for _, grace := range []bool{false, true} {
					steps, err := CompileHIRSteps(parse(), Options{ContractOpt: true, UseHIRGrace: grace, UseGrace: grace, AssertMode: "all"})
					if err != nil {
						t.Errorf("grace=%v: %v", grace, err)
						continue
					}
					if err := RunAssertsMIR2(parse(), steps.MIR2Module); err != nil {
						t.Errorf("final VM: %v", err)
					}
				}
			})
		}
	}
}
