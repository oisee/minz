package c89

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"testing"
)

func TestConditionalExpressionEffectsRaw(t *testing.T) {
	src, e := os.ReadFile("../../../reports/c89-expression-effects/source.c")
	if e != nil {
		t.Fatal(e)
	}
	h, e := CompileWithOpts(string(src), "expression-effects.c", CompileOpts{NoLibc: true})
	if e != nil {
		t.Fatal(e)
	}
	m := hir.LowerModule(h)
	for _, tc := range []struct {
		f       string
		a, b, w int64
	}{{"and_post", 0, 7, 1792}, {"and_post", 1, 7, 2049}, {"or_post", 1, 7, 1793}, {"or_post", 0, 7, 2049}, {"pick_post", 0, 7, 2056}, {"pick_post", 1, 7, 2055}, {"effect_cond", 0, 7, 264}, {"nested", 0, 7, 2055}} {
		t.Run(tc.f, func(t *testing.T) {
			vm := mir2.NewVM(m)
			v, e := vm.Call(tc.f, []mir2.Value{{I: tc.a}, {I: tc.b}})
			if e != nil || len(v) != 1 || v[0].I != tc.w {
				t.Fatal(tc, v, e)
			}
		})
	}
}
