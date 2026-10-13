package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestProfileWorkloadDeniedHooksAndPrivateEvents(t *testing.T) {
	source := &Module{Funcs: []*Func{{Name: "disk_read", IsExtern: true, RetTy: mir2.TyU8}}}
	vm := mir2.NewVM(&mir2.Module{})
	vm.Hosts = map[string]mir2.HostFunc{}
	installProfileDeniedExternHooks(vm, source)
	for _, name := range []string{"disk_read", "@disk_read"} {
		v, e := vm.Hosts[name](nil)
		if e == nil || v != nil {
			t.Fatal("fabricated unbound result", name, v, e)
		}
	}
}
