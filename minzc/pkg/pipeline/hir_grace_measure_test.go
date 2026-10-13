package pipeline

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func graceMeasureSource(tb testing.TB) string {
	tb.Helper()
	src, err := os.ReadFile("../../../examples/nanz/grace_hir_constant_if.nanz")
	if err != nil {
		tb.Fatal(err)
	}
	return string(src)
}

func graceMeasureParse(tb testing.TB, src string) *hir.Module {
	tb.Helper()
	h, err := nanz.Parse(src, "grace_measure.nanz")
	if err != nil {
		tb.Fatal(err)
	}
	return h
}

func graceMeasureModule(tb testing.TB, src string, useHIR, final bool, useMIR ...bool) *mir2.Module {
	tb.Helper()
	h := graceMeasureParse(tb, src)
	if final {
		s, err := CompileHIRSteps(h, Options{UseHIRGrace: useHIR, UseGrace: len(useMIR) > 0 && useMIR[0], AssertMode: "mir2", SkipZ80Emission: true})
		if err != nil {
			tb.Fatal(err)
		}
		return s.MIR2Module
	}
	if useHIR {
		if _, err := hir.RunGrace(h, 0); err != nil {
			tb.Fatal(err)
		}
	}
	m := hir.LowerModule(h)
	if err := mir2.Verify(m); err != nil {
		tb.Fatal(err)
	}
	return m
}

func graceMeasureShape(m *mir2.Module) (blocks, insts int) {
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			blocks++
			insts += len(b.Insts) // terminators reported through block count
		}
	}
	return
}

// Measure actual VM work over all byte inputs for both functions. Expected
// values come from Go byte arithmetic, not the other VM execution.
func graceMeasureVM(tb testing.TB, m *mir2.Module) int64 {
	tb.Helper()
	vm := mir2.NewVM(m)
	for x := int64(0); x < 256; x++ {
		for _, fn := range []string{"wrap", "choose"} {
			want := x
			if fn == "wrap" {
				want = int64(uint8(x + 1))
			}
			got, err := vm.Call(fn, []mir2.Value{{I: x}})
			if err != nil || len(got) != 1 || got[0].I != want {
				tb.Fatalf("%s(%d): got %v err=%v want %d", fn, x, got, err, want)
			}
		}
	}
	return vm.StepsExecuted()
}

func TestHIRGraceMeasurements(t *testing.T) {
	src := graceMeasureSource(t)
	for _, stage := range []struct {
		name  string
		final bool
	}{{"raw", false}, {"optimized", true}, {"optimized_mir_grace", true}} {
		before := graceMeasureModule(t, src, false, stage.final, stage.name == "optimized_mir_grace")
		after := graceMeasureModule(t, src, true, stage.final, stage.name == "optimized_mir_grace")
		bb, bi := graceMeasureShape(before)
		ab, ai := graceMeasureShape(after)
		bs, as := graceMeasureVM(t, before), graceMeasureVM(t, after)
		t.Logf("%s: blocks %d -> %d; instructions %d -> %d; VM steps over 512 calls %d -> %d", stage.name, bb, ab, bi, ai, bs, as)
	}
}

// Parsing is outside these timings. grace_lower includes rewriting, per-round
// validation and lowering; pipeline includes the usual optimized MIR2 path and
// four VM assertions. No target emission or execution is measured.
func BenchmarkHIRGraceCompile(b *testing.B) {
	src := graceMeasureSource(b)
	for _, name := range []string{"lower", "grace_lower", "pipeline", "grace_pipeline"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				h := graceMeasureParse(b, src)
				b.StartTimer()
				switch name {
				case "lower", "grace_lower":
					if name == "grace_lower" {
						if _, err := hir.RunGrace(h, 0); err != nil {
							b.Fatal(err)
						}
					}
					_ = hir.LowerModule(h)
				default:
					if _, err := CompileHIRSteps(h, Options{UseHIRGrace: name == "grace_pipeline", AssertMode: "mir2", SkipZ80Emission: true}); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

func BenchmarkHIRGraceVM(b *testing.B) {
	src := graceMeasureSource(b)
	for _, tc := range []struct {
		name          string
		useHIR, final bool
	}{
		{"raw", false, false}, {"raw_grace", true, false},
		{"optimized", false, true}, {"optimized_grace", true, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			m := graceMeasureModule(b, src, tc.useHIR, tc.final)
			steps := graceMeasureVM(b, m)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				graceMeasureVM(b, m)
			}
			b.ReportMetric(float64(steps)/512, "steps/call")
		})
	}
}
