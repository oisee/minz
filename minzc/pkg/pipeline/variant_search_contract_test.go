package pipeline

import (
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Reflection keeps these regressions executable on the pre-search main branch.
func variantContractSwitch(o *Options, name string, value bool) {
	if f := reflect.ValueOf(o).Elem().FieldByName(name); f.IsValid() {
		f.SetBool(value)
	}
}

func TestVariantSearchExactConfigurationVMZ80(t *testing.T) {
	const source = `fun add5(x:u8)->u8 { return (x+5) & 255 }`
	ordinary, err := CompileHIRSteps(graceMeasureParse(t, source), Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ordinary.Assembly, "variant=") {
		t.Error("ordinary native compilation did not search")
	}
	// Empty maps explicitly disable every site. They are requests, unlike nil.
	for _, policy := range []string{"OrderedByteRWPolicy", "OrderedBytePolicy", "RegionComparePolicy", "InvariantBasePolicy", "GraceRulePolicy", "ForwardCopyPolicy", "PointerWalkPolicy", "LoopLineage", "CollectHIRSites"} {
		for _, force := range []bool{false, true} {
			name := policy
			if force {
				name += "/forced"
			}
			t.Run(name, func(t *testing.T) {
				prepare := func() *hir.Module {
					h := graceMeasureParse(t, source)
					if policy != "CollectHIRSites" {
						f := reflect.ValueOf(h).Elem().FieldByName(policy)
						f.Set(reflect.MakeMap(f.Type()))
					}
					return h
				}
				o := Options{AssertMode: "none", CollectHIRSites: policy == "CollectHIRSites"}
				variantContractSwitch(&o, "ForceVariantSearch", force)
				got, err := CompileHIRSteps(prepare(), o)
				if err != nil {
					t.Fatal(err)
				}
				variantContractSwitch(&o, "DisableVariantSearch", true)
				want, err := CompileHIRSteps(prepare(), o)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(got.Assembly, "variant=") || got.Assembly != want.Assembly {
					t.Error("search changed an exact configuration")
				}
				for _, x := range []int64{0, 250, 255} {
					v, err := mir2.NewVM(got.MIR2Module).Call("add5", []mir2.Value{{I: x}})
					if err != nil || len(v) != 1 || v[0].I != int64(byte(x+5)) {
						t.Fatalf("MIR x=%d: %v %v", x, v, err)
					}
					executeLocalTableFunction(t, got.Assembly, byte(x))
				}
			})
		}
	}
}

func TestVariantSearchImplicitDefaultsVMZ80(t *testing.T) {
	const source = `fun add5(x:u8)->u8 { return (x+5) & 255 }`
	got, err := CompileHIRSteps(graceMeasureParse(t, source))
	if err != nil {
		t.Fatal(err)
	}
	want, err := CompileHIRSteps(graceMeasureParse(t, source), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got.Assembly != want.Assembly || got.MIR2Opt != want.MIR2Opt {
		t.Error("implicit search options differ from production defaults")
	}
	if !strings.Contains(got.Assembly, "mir-grace=true") {
		t.Error("implicit search lost default MIR Grace")
	}
	for _, x := range []int64{0, 250, 255} {
		v, err := mir2.NewVM(got.MIR2Module).Call("add5", []mir2.Value{{I: x}})
		if err != nil || len(v) != 1 || v[0].I != int64(byte(x+5)) {
			t.Fatalf("MIR x=%d: %v %v", x, v, err)
		}
		executeLocalTableFunction(t, got.Assembly, byte(x))
	}
}

func TestVariantSearchMeasuredConfigurations(t *testing.T) {
	check := func(assembly string) {
		t.Helper()
		if strings.Contains(assembly, "variant=") {
			t.Error("static variant search entered a measured configuration")
		}
	}
	factory, w := modeFixture()
	modes, err := MeasureCompilerModeVariants(factory, w, "speed", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range modes.Evidence.Candidates {
		check(c.Assembly)
	}
	contracts, err := MeasureContractStrategies(factory, w, "speed", Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range contracts.Evidence.Candidates {
		check(c.Assembly)
	}
	factory, w = conditionalWorkloadFixture()
	layouts, err := MeasureConditionalLayoutModes(factory, "classify", w, []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true}}, "speed", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range layouts.Evidence.Runs {
		for _, c := range run.Candidates {
			check(c.Assembly)
		}
	}
	// The public evidence options must reproduce the exact conditional baseline.
	replay, err := CompileHIRSteps(func() *hir.Module {
		h, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		return h
	}(), layouts.Evidence.Options[0])
	if err != nil {
		t.Fatal(err)
	}
	if replay.Assembly != layouts.Evidence.Runs[0].Candidates[0].Assembly {
		t.Error("evidence options did not replay the measured baseline")
	}
}
