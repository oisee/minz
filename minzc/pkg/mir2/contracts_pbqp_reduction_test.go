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

func TestPFCCOR1RejectsIncompatibleNeighborChoice(t *testing.T) {
	nodes := []pfccoNode{{costs: []int{0}}, {costs: []int{0, 1}}}
	edges := []pfccoEdge{{callerIdx: 0, calleeIdx: 1, costMatrix: [][]int{{InfCost, 0}}}}
	got := pfccoSolve(nodes, edges)
	if got[0] != 0 || got[1] != 1 {
		t.Fatalf("incompatible assignment %v; want [0 1]", got)
	}
}

func TestPFCCORejectsInfeasibleAssignments(t *testing.T) {
	cases := []struct {
		name  string
		nodes []pfccoNode
		edges []pfccoEdge
	}{
		{"isolated", []pfccoNode{{costs: []int{InfCost}}}, nil},
		{"leaf", []pfccoNode{{costs: []int{0}}, {costs: []int{0}}}, []pfccoEdge{{callerIdx: 0, calleeIdx: 1, costMatrix: [][]int{{InfCost}}}}},
		// An odd cycle of unequal binary choices has no feasible assignment.
		// RN must reject its greedy result rather than advertise it as valid.
		{"residual", []pfccoNode{{costs: []int{0, 1}}, {costs: []int{0, 1}}, {costs: []int{0, 1}}}, []pfccoEdge{
			{callerIdx: 0, calleeIdx: 1, costMatrix: [][]int{{InfCost, 0}, {0, InfCost}}},
			{callerIdx: 1, calleeIdx: 2, costMatrix: [][]int{{InfCost, 0}, {0, InfCost}}},
			{callerIdx: 2, calleeIdx: 0, costMatrix: [][]int{{InfCost, 0}, {0, InfCost}}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pfccoSolve(tc.nodes, tc.edges); got != nil {
				t.Fatalf("infeasible assignment admitted: %v", got)
			}
		})
	}
}
