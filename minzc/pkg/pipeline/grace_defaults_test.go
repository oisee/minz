package pipeline

import "testing"

func TestDefaultOptionsEnableGrace(t *testing.T) {
	opts := DefaultOptions()
	if !opts.UseGrace || !opts.UseHIRGrace {
		t.Fatalf("production defaults must enable MIR and HIR Grace: %+v", opts)
	}
	// The production entry point must compile the same program as explicit Grace.
	h := graceMeasureParse(t, graceSumSource)
	got, err := CompileHIR(h)
	if err != nil {
		t.Fatal(err)
	}
	want, err := CompileHIRWithOptions(graceMeasureParse(t, graceSumSource), Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("default compiler differs from explicit Grace")
	}
	steps, err := CompileHIRSteps(graceMeasureParse(t, graceSumSource))
	if err != nil {
		t.Fatal(err)
	}
	if steps.Assembly != want {
		t.Fatal("no-options steps differ from explicit Grace")
	}
}
