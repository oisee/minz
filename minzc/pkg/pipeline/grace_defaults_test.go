package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestDefaultOptionsEnableGrace(t *testing.T) {
	opts := DefaultOptions()
	if !opts.UseGrace || opts.UseHIRGrace {
		t.Fatalf("production defaults must enable MIR Grace and leave HIR Grace opt-in: %+v", opts)
	}
	// The production entry point must compile the same program as explicit Grace.
	h := graceMeasureParse(t, graceSumSource)
	got, err := CompileHIR(h)
	if err != nil {
		t.Fatal(err)
	}
	want, err := CompileHIRWithOptions(graceMeasureParse(t, graceSumSource), Options{ContractOpt: true, UseGrace: true, UseHIRGrace: false})
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

// Check the public default on an executable loop, including zero trip and u8
// overflow, rather than only checking the option bits.
func TestGraceDefaultPointerLoopNativeVM(t *testing.T) {
	source := graceSumSource + `
 global data:[u8;3]=[250,10,3]
 fun fixture(n:u8)->u8 { return sum_array(&data,n) }
 assert fixture(0)==0 via z80
 assert fixture(1)==250 via z80
  `
	for _, backend := range []string{"mir2", "z80"} {
		input := strings.ReplaceAll(source, "via z80", "via "+backend)
		want := 2
		// The MIR VM's arithmetic domain is wider; the native assertion checks
		// byte wraparound on the actual Z80 code.
		if backend == "z80" {
			input += "assert fixture(3)==7 via z80\n"
			want++
		}
		h := graceMeasureParse(t, input)
		h.AssertStats = &hir.AssertStats{}
		options := DefaultOptions()
		options.AssertMode = backend
		steps, err := CompileHIRSteps(h, options)
		if err != nil {
			t.Fatal(err)
		}
		if h.AssertStats.Executed != want || h.AssertStats.Passed != want {
			t.Fatal(h.AssertStats)
		}
		if strings.Contains(steps.HIR, "__grace_") {
			t.Fatal("default unexpectedly versions the pointer loop")
		}
	}
}

func TestGraceDefaultCompareLoopNativeVM(t *testing.T) {
	const source = `fun reduce(a:u8,b:u8)->u8 {
 while a!=b { if a>b { a=a-b } else { b=b-a } }
 return a
 }
 assert reduce(12,8)==4 via z80
 assert reduce(21,14)==7 via z80
 assert reduce(7,7)==7 via z80
 assert reduce(0,0)==0 via z80
 `
	for _, backend := range []string{"mir2", "z80"} {
		h := graceMeasureParse(t, strings.ReplaceAll(source, "via z80", "via "+backend))
		h.AssertStats = &hir.AssertStats{}
		options := DefaultOptions()
		options.AssertMode = backend
		steps, err := CompileHIRSteps(h, options)
		if err != nil {
			t.Fatal(err)
		}
		if h.AssertStats.Executed != 4 || h.AssertStats.Passed != 4 {
			t.Fatal(h.AssertStats)
		}
		fused := false
		for _, f := range steps.MIR2Module.Funcs {
			for _, b := range f.Blocks {
				if _, ok := b.Term.(*mir2.TermBrIf2); ok {
					fused = true
				}
			}
		}
		if !fused {
			t.Fatal("default missed fused equality/ordering branch")
		}
	}
}
