package vir

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// Complete independent Cartesian enumeration is the oracle, not another
// invocation of the propagator. 343 domain configurations over a three-cycle.
func TestCompositionDomainPropagationPreservesAllSolutions(t *testing.T) {
	choices := []map[int]int{{1: 0, 2: 0}, {1: 0, 2: 1}, {1: 1, 2: 0}}
	solutions := func(domains [][]map[int]int) []string {
		found := []string{}
		for _, a := range domains[0] {
			for _, b := range domains[1] {
				for _, c := range domains[2] {
					if a[2] == b[2] && b[3] == c[3] && a[1] == c[1] {
						found = append(found, fmt.Sprintf("%d/%d/%d", a[1], a[2], b[3]))
					}
				}
			}
		}
		sort.Strings(found)
		return found
	}
	cases := 0
	for ma := 1; ma < 8; ma++ {
		for mb := 1; mb < 8; mb++ {
			for mc := 1; mc < 8; mc++ {
				input := make([][]map[int]int, 3)
				for i, choice := range choices {
					if ma&(1<<i) != 0 {
						input[0] = append(input[0], choice)
					}
					if mb&(1<<i) != 0 {
						input[1] = append(input[1], map[int]int{2: choice[1], 3: choice[2]})
					}
					if mc&(1<<i) != 0 {
						input[2] = append(input[2], map[int]int{1: choice[1], 3: choice[2]})
					}
				}
				before := solutions(input)
				output := propagateCompositionDomains(input)
				after := solutions(output)
				if !reflect.DeepEqual(before, after) {
					t.Fatal(ma, mb, mc, before, after)
				}
				if !reflect.DeepEqual(output, propagateCompositionDomains(output)) {
					t.Fatal("not at fixpoint")
				}
				cases++
			}
		}
	}
	t.Logf("%d finite domain configurations: complete solution sets preserved", cases)
}

func TestCompositionDomainPropagationPrunesAndOwnsSlices(t *testing.T) {
	first := map[int]int{1: 0}
	unsupported := map[int]int{1: 1}
	input := [][]map[int]int{{first, unsupported}, {{1: 0}}, {{1: 0}}}
	output := propagateCompositionDomains(input)
	if len(output[0]) != 1 || len(input[0]) != 2 || input[0][1][1] != 1 {
		t.Fatal("bad pruning or input mutated")
	}
	output[0][0] = map[int]int{1: 9}
	if input[0][0][1] != 0 {
		t.Fatal("slice aliases input")
	}
}

func TestCompositionDomainRejectsEmptyAndNonadjacentConflict(t *testing.T) {
	domains := [][]map[int]int{{{1: 0}}, {{2: 1}}, {{1: 1}}}
	output := propagateCompositionDomains(domains)
	for _, domain := range output {
		if len(domain) != 0 {
			t.Fatal("nonadjacent conflict retains domain", output)
		}
	}
	output = propagateCompositionDomains([][]map[int]int{{{1: 0}}, nil})
	if len(output[0]) != 0 {
		t.Fatal("unsupported candidate survives empty domain")
	}
}
