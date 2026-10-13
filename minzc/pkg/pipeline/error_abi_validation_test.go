package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestErrorABIRejectsUnsupportedBeforeLowering(t *testing.T) {
	for _, src := range []string{
		`fun f()->u16 {@error(1) return 0}`,
		`fun f()->u8 {@propagate return 0}`,
		`fun f()->u8 {@error(1)} fun g()->u8 {return f()}`,
		`fun f()->u8 {@error(1)} fun g()->u8 {var x:u8=f() var y:u8=0 @propagate return x}`,
	} {
		for _, steps := range []bool{false, true} {
			h, err := nanz.Parse(src, "error-validation.nanz")
			if err != nil {
				t.Fatal(err)
			}
			if steps {
				_, err = CompileHIRSteps(h)
			} else {
				_, err = CompileHIRWithOptions(h, Options{})
			}
			if err == nil {
				t.Fatalf("accepted unsupported CY semantics: %s", src)
			}
		}
	}
}
func TestFallibleSuffixHasExplicitSuccessStatusAndAPIParity(t *testing.T) {
	src := `fun f?(x:u8)->u8 {return x+1} fun caller(x:u8)->u8 {var y:u8=f?(x) @propagate return y}`
	parse := func() *hir.Module {
		h, err := nanz.Parse(src, "fallible-suffix.nanz")
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	steps, err := CompileHIRSteps(parse(), Options{ContractOpt: true})
	if err != nil {
		t.Fatal(err)
	}
	asm, err := CompileHIRWithOptions(parse(), Options{ContractOpt: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asm, "CALL f") || !strings.Contains(steps.Assembly, "CALL f") || !strings.Contains(steps.MIR2Raw, "outcome") {
		t.Fatal("fallible source disappeared or API mismatch")
	}
	for _, x := range []int64{0, 127, 254, 255} {
		out, err := mir2.NewVM(steps.MIR2Module).CallWithOutcome("caller", []mir2.Value{{I: x}})
		if err != nil || out.Failed || len(out.Values) != 1 || out.Values[0].I != (x+1)&255 {
			t.Fatal(out, err)
		}
	}
}

func TestValueOnlyProfilesRejectLanguageOutcomeBeforeExecution(t *testing.T) {
	h, err := nanz.Parse(`fun f()->u8 {@error(1)}`, "profile-error.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = hir.BuildWorkloadProfileContext(h, hir.ProfileWorkload{}); err == nil || !strings.Contains(err.Error(), "CY language outcomes") {
		t.Fatal("value-only profile accepted", err)
	}
	h, err = nanz.Parse(`fun f()->u8 {@error(1)}`, "memory-profile-error.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = hir.PrepareMemoryHostWorkload(h, hir.MemoryHostWorkload{}); err == nil || !strings.Contains(err.Error(), "CY language outcomes") {
		t.Fatal("memory profile accepted", err)
	}
}

func TestErrorABIRejectsUnimplementedBackend(t *testing.T) {
	h, err := nanz.Parse(`fun f()->u8 {@error(1)}`, "error-ez80.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CompileHIRSteps(h, Options{Backend: "ez80"}); err == nil || !strings.Contains(err.Error(), "requires Z80") {
		t.Fatal(err)
	}
}
