package mir2

import (
	"strings"
	"testing"
)

func TestCPMApplicationImageBudget(t *testing.T) {
	// Ordinary application entry can exceed the guarded runtime's 0x8000 limit.
	if err := validateCPMApplicationImage(cpmApplicationEntry + "main:\n RET\n DS 32768\n" + cpmApplicationTail()); err != nil {
		t.Fatal(err)
	}
	if err := validateCPMApplicationImage(cpmApplicationEntry + "main:\n RET\n DS 64000\n" + cpmApplicationTail()); err == nil || !strings.Contains(err.Error(), "below 0xFE00") {
		t.Fatalf("oversized resident stack image admitted: %v", err)
	}
}

func TestCPMApplicationEntryMainContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Module, *Func)
	}{
		{"missing main", func(m *Module, _ *Func) { m.Funcs = nil }},
		{"target", func(m *Module, _ *Func) { m.Target = 255 }},
		{"argument", func(_ *Module, f *Func) { f.Contract.Params = []Param{{Ty: TyU8}} }},
		{"result", func(_ *Module, f *Func) { f.Contract.Returns = []Return{{Ty: TyU8}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("main")
			tc.change(m, f)
			if err := validateCPMApplicationEntry(m); err == nil {
				t.Fatal("unsupported main admitted")
			}
		})
	}
	m := &Module{}
	m.AddFunc("main")
	if _, err := Z80Codegen(m, &AllocResult{}, Z80CodegenOptions{CPMApplicationEntry: true, Runtime: Z80RuntimeCPM}); err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("ambiguous entry/runtime admitted: %v", err)
	}
	m.AddFunc("__cpm_app_entry")
	if err := ValidateZ80Symbols(m); err == nil {
		t.Fatal("application namespace not reserved")
	}
}
