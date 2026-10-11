package mir2

import "testing"

func TestPFCCOR1RestoresAccumulatedCosts(t *testing.T) {
	// A projected leaf cost must survive reconstruction of its neighbor.
	nodes := []pfccoNode{{costs: []int{0, 100}}, {costs: []int{0, 1}}, {costs: []int{0}}}
	edges := []pfccoEdge{
		{callerIdx: 0, calleeIdx: 1, costMatrix: [][]int{{10, 0}, {0, 0}}},
		{callerIdx: 1, calleeIdx: 2, costMatrix: [][]int{{0}, {0}}},
	}
	got := pfccoSolve(nodes, edges)
	cost := func(a []int) int {
		total := 0
		for i, k := range a {
			total += nodes[i].costs[k]
		}
		for _, e := range edges {
			total += e.costMatrix[a[e.callerIdx]][a[e.calleeIdx]]
		}
		return total
	}
	best := InfCost
	for a := 0; a < 2; a++ {
		for b := 0; b < 2; b++ {
			if c := cost([]int{a, b, 0}); c < best {
				best = c
			}
		}
	}
	if c := cost(got); c != best {
		t.Fatalf("assignment %v cost %d; exhaustive optimum %d", got, c, best)
	}
}
