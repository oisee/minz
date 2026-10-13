package pipeline

import (
	"bytes"
	"errors"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func conditionalWorkloadFixture() (func() (*hir.Module, error), hir.ProfileWorkload) {
	source := `global observed:u8=0
fun classify(c:u8)->u8 {
 if c==32 { return 1 }
 if c==9 { return 1 }
 if c==10 { return 1 }
 if c==13 { return 1 }
 return 0
}
fun caller(c:u8)->u8 {
 let result:u8=classify(c)
 observed=result
 return result+c
}
fun untouched(c:u8)->u8 { return c+7 }`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "conditional.nanz") }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{"observed": {255}}}
	for _, c := range []int64{32, 65, 9, 48, 10, 66, 13, 0, 255} {
		value := c
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "caller", Arguments: []hir.ProfileArgument{{Value: &value}}})
	}
	return factory, w
}
func TestConditionalLayoutWorkloadNativeAndArtifact(t *testing.T) {
	factory, w := conditionalWorkloadFixture()
	for _, goal := range []string{"speed", "size"} {
		r, err := MeasureConditionalLayoutWorkload(factory, "classify", w, Options{OptSize: true, ContractOpt: true, AssertMode: "none"}, goal, 4)
		if err != nil {
			t.Fatal(err)
		}
		chosen := r.Evidence.Candidates[r.Evidence.Selected]
		if r.Steps.Assembly != chosen.Assembly || !bytes.Equal(r.Binary, chosen.Binary) || len(r.Evidence.Candidates) < 2 {
			t.Fatal("not selected executable")
		}
		for _, candidate := range r.Evidence.Candidates {
			for i, call := range w.Calls {
				c := byte(*call.Arguments[0].Value)
				flag := byte(0)
				if c == 32 || c == 9 || c == 10 || c == 13 {
					flag = 1
				}
				got := candidate.Run.Invocations[i]
				if len(got.Results) != 1 || byte(got.Results[0].I) != c+flag || !bytes.Equal(got.Snapshot.Globals["observed"], []byte{flag}) {
					t.Fatal("independent result/state", i, got)
				}
			}
		}
		source, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		oracle, err := hir.RunLoopProfile(source, w)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, source, w, *oracle)
		if err != nil || replay.TotalTStates != chosen.Run.TotalTStates {
			t.Fatal("exact replay", err)
		}
		t.Log(goal, "selected", r.Evidence.Selected, "bytes", len(r.Binary), "T", chosen.Run.TotalTStates)
	}
}
func TestConditionalLayoutWorkloadRejectsDriftAndPartialResults(t *testing.T) {
	for _, kind := range []string{"shared", "source", "ABI", "late-mutation", "late-failure", "candidate-mutation", "policy", "late-policy"} {
		t.Run(kind, func(t *testing.T) {
			fresh, w := conditionalWorkloadFixture()
			shared, err := fresh()
			if err != nil {
				t.Fatal(err)
			}
			var prior *hir.Module
			calls := 0
			factory := func() (*hir.Module, error) {
				calls++
				m, e := fresh()
				if e != nil {
					return nil, e
				}
				if kind == "policy" {
					m.GraceRulePolicy = map[string]bool{}
				}
				if calls == 3 && kind == "late-policy" {
					prior.GraceRulePolicy = map[string]bool{}
				}
				if kind == "shared" {
					return shared, nil
				}
				if calls == 2 && kind == "source" {
					m.Name = "changed"
				}
				if calls == 2 && kind == "ABI" {
					m.FuncByName("classify").Params[0].Name = "foreign"
				}
				if calls == 3 && kind == "late-failure" {
					return nil, errors.New("late failure")
				}
				if calls == 3 && (kind == "late-mutation" || kind == "candidate-mutation") {
					prior.Name = "mutated"
				}
				if calls == 1 && (kind == "late-mutation" || kind == "late-policy") || calls == 2 && kind == "candidate-mutation" {
					prior = m
				}
				return m, nil
			}
			result, err := MeasureConditionalLayoutWorkload(factory, "classify", w, Options{OptSize: true, AssertMode: "none"}, "speed", 4)
			if err == nil || result != nil {
				t.Fatal("partial/drifting winner accepted", kind)
			}
		})
	}
}
func TestConditionalFunctionSpanKeepsAllReturnsAndStorage(t *testing.T) {
	for _, suffix := range []string{"; globals\ng: DB 1\n", "; strings\ns: DB 0\n", "; runtime: helper\nh: RET\n", "; fun next()\nnext: RET\n", ""} {
		asm := "; generated\n; fun test(c:u8)\ntest:\n RET\nlabel:\n RET\n\n" + suffix
		a, b, err := conditionalFunctionSpan(asm, "test")
		if err != nil || asm[b:] != suffix || !bytes.Contains([]byte(asm[a:b]), []byte("label:\n RET")) {
			t.Fatal(a, b, err, suffix)
		}
	}
}
