package pipeline

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestConditionalLayoutModesOwnsInputsAndTies(t *testing.T) {
	fresh, w := conditionalWorkloadFixture()
	options := Options{OptSize: true, AssertMode: "none"}
	modes := []Options{options, options}
	for _, goal := range []string{"speed", "size"} {
		calls := 0
		factory := func() (*hir.Module, error) {
			calls++
			if calls == 1 {
				modes[1].Runtime = "foreign"
				*w.Calls[0].Arguments[0].Value = 99
			}
			return fresh()
		}
		result, e := MeasureConditionalLayoutModes(factory, "classify", w, modes, goal, 4)
		if e != nil {
			t.Fatal(e)
		}
		if result.Evidence.SelectedMode != 0 || len(result.Evidence.Runs) != 2 {
			t.Fatal("tie/order")
		}
		chosen := result.Evidence.Runs[0].Candidates[result.Evidence.Runs[0].Selected]
		if result.Steps.Assembly != chosen.Assembly || !bytes.Equal(result.Binary, chosen.Binary) || chosen.Run.Invocations[0].Results[0].I != 33 {
			t.Fatal("frozen input/exact winner")
		}
		modes[1] = options
		*w.Calls[0].Arguments[0].Value = 32
	}
}
func TestConditionalLayoutModesRejectsCrossModeDrift(t *testing.T) {
	for _, kind := range []string{"source", "failure", "previous-mutation", "shared"} {
		t.Run(kind, func(t *testing.T) {
			fresh, w := conditionalWorkloadFixture()
			calls := 0
			var earlier *hir.Module
			factory := func() (*hir.Module, error) {
				calls++
				m, e := fresh()
				if e != nil {
					return nil, e
				}
				if calls == 2 {
					earlier = m
				}
				if calls == 4 {
					switch kind {
					case "source":
						m.Name = "foreign"
					case "failure":
						return nil, errors.New("second-mode failure")
					case "previous-mutation":
						earlier.Name = "mutated earlier candidate"
					case "shared":
						return earlier, nil
					}
				}
				return m, nil
			}
			options := Options{OptSize: true, AssertMode: "none"}
			result, e := MeasureConditionalLayoutModes(factory, "classify", w, []Options{options, options}, "speed", 4)
			if calls < 4 {
				t.Fatal("did not reach cross-mode check", calls, e)
			}
			if e == nil || result != nil {
				t.Fatal("partial/drifting winner admitted")
			}
		})
	}
}

func TestConditionalLayoutModesRechecksEarlierTablePin(t *testing.T) {
	_, path := localTableFixture(t)
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	fresh, w := conditionalWorkloadFixture()
	pinned := Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path, CheckedLocalTableSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	ordinary := Options{OptSize: true, AssertMode: "none"}
	calls := 0
	factory := func() (*hir.Module, error) {
		calls++
		if calls == 4 {
			if e := os.WriteFile(path, []byte("[]"), 0644); e != nil {
				return nil, e
			}
		}
		return fresh()
	}
	result, e := MeasureConditionalLayoutModes(factory, "classify", w, []Options{pinned, ordinary}, "speed", 4)
	if calls != 6 {
		t.Fatal("did not finish later unpinned mode", calls, e)
	}
	if e == nil || result != nil {
		t.Fatal("earlier pinned configuration changed without rejection")
	}
}
