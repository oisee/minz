package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestMemoryHostDeadExternDefensiveHooks(t *testing.T) {
	m := &mir2.Module{Funcs: []*mir2.Func{{Name: "unbound", Attrs: mir2.FuncAttrs{IsExtern: true}}, {Name: "private", Attrs: mir2.FuncAttrs{IsExtern: true}}}}
	vm := mir2.NewVM(m)
	called := 0
	private := func([]mir2.Value) ([]mir2.Value, error) { called++; return nil, nil }
	vm.Hosts["private"] = private
	installMemoryHostDeniedExternHooks(vm, m, MemoryHostWorkload{}, map[string]mir2.HostFunc{"private": private})
	for _, name := range []string{"unbound", "@unbound"} {
		hook := vm.Hosts[name]
		if hook == nil {
			t.Fatal("missing refusal", name)
		}
		out, e := hook([]mir2.Value{{I: 99}})
		if e == nil || !strings.Contains(e.Error(), "executed unbound extern") || len(out) != 0 {
			t.Fatal("fabricated external effect/result", out, e)
		}
	}
	vm.Hosts["private"](nil)
	if called != 1 {
		t.Fatal("owned hook replaced")
	}
}
