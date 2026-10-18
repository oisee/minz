package pipeline

import (
	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestCPMApplicationEntryAdmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*hir.Module, *Options)
	}{
		{"runtime", func(_ *hir.Module, o *Options) { o.Runtime = mir2.Z80RuntimeCPM }},
		{"target", func(h *hir.Module, _ *Options) { h.Target = hir.TargetGeneric }},
		{"backend", func(_ *hir.Module, o *Options) { o.Backend = "ez80" }},
		{"intermediate", func(_ *hir.Module, o *Options) { o.SkipZ80Emission = true }},
		{"profile", func(_ *hir.Module, o *Options) { o.ProfileSelection = &ProfileSelection{} }},
		{"assertion", func(h *hir.Module, o *Options) { o.AssertMode = "mir2"; h.Asserts = []hir.Assert{{}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := abap.Compile("REPORT zentry.", "entry")
			if err != nil {
				t.Fatal(err)
			}
			o := DefaultOptions()
			o.CPMApplicationEntry = true
			o.AssertMode = "none"
			tc.change(h, &o)
			_, err = CompileHIRSteps(h, o)
			if err == nil || !strings.Contains(err.Error(), "CP/M application entry") {
				t.Fatalf("unsupported entry combination admitted: %v", err)
			}
		})
	}
}
