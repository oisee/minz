package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"math/rand"
	"testing"
)

// This oracle measures an explicit ABI adapter, not classMoveCost or nodeCosts.
// C is live across CALL. When the callee owns C it deliberately destroys C,
// requiring a real PUSH/POP save. Return is fixed A; parameter is A/C/E.
func measuredContractAdapter(t *testing.T, src, dst string) int {
	t.Helper()
	body := ""
	if dst == "C" {
		body += "PUSH BC\n"
	}
	if src != dst {
		body += fmt.Sprintf("LD %s,%s\n", dst, src)
	}
	body += "CALL adapter_callee\n"
	if dst == "C" {
		body += "POP BC\n"
	}
	body += "DI\nHALT\nadapter_callee:\n"
	if dst != "A" {
		body += "LD A," + dst + "\n"
	}
	if dst == "C" {
		body += "LD C,0\n"
	}
	body += "RET\n"
	setup := fmt.Sprintf("LD BC,4660\nLD %s,7\n", src)
	result, err := z80asm.NewAssembler().AssembleString("ORG 32768\n" + setup + body)
	if err != nil || len(result.Errors) > 0 {
		t.Fatal(err, result.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	z.LoadMemory(0x8000, result.Binary)
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	r := z.GetRegisters()
	wantBC := uint16(4660)
	if src == "C" {
		wantBC = 0x1207
	}
	if r.A != 7 || r.BC != wantBC || r.SP != 0xf000 {
		t.Fatalf("adapter %s→%s output/liveBC/SP failure %+v", src, dst, r)
	}
	// Setup: LD BC,nn 10T; LD r,n 7T. All adapter moves/saves/callee included.
	return int(z.GetCycles()) - 17
}

func independentContractCost(nodes []pfccoNode, edges []pfccoEdge, a []int) int {
	if len(a) != len(nodes) {
		return InfCost
	}
	total := 0
	for i, k := range a {
		if k < 0 || k >= len(nodes[i].costs) || nodes[i].costs[k] >= InfCost {
			return InfCost
		}
		total += nodes[i].costs[k]
	}
	for _, e := range edges {
		c := e.costMatrix[a[e.callerIdx]][a[e.calleeIdx]]
		if c >= InfCost {
			return InfCost
		}
		total += c
	}
	return total
}

func exhaustiveContractOptimum(nodes []pfccoNode, edges []pfccoEdge) int {
	best := InfCost
	a := make([]int, len(nodes))
	var visit func(int)
	visit = func(i int) {
		if i == len(a) {
			if c := independentContractCost(nodes, edges, a); c < best {
				best = c
			}
			return
		}
		for k := range nodes[i].costs {
			a[i] = k
			visit(i + 1)
		}
	}
	visit(0)
	return best
}

func TestPFCCOForestsMatchIndependentExhaustiveMeasuredAdapters(t *testing.T) {
	names := []string{"A", "C", "E"}
	matrix := make([][]int, 3)
	for i, src := range names {
		matrix[i] = make([]int, 3)
		for j, dst := range names {
			matrix[i][j] = measuredContractAdapter(t, src, dst)
		}
	}
	t.Logf("native adapter matrix A/C/E including real CALL/RET and liveBC saves: %v", matrix)
	// Negative control: dropping PUSH BC / POP BC (21T) changes the winner.
	controlNodes := []pfccoNode{{costs: []int{0}}, {costs: []int{20, 0}}}
	controlEdges := []pfccoEdge{{callerIdx: 0, calleeIdx: 1, costMatrix: [][]int{{matrix[0][0], matrix[0][1]}}}}
	withoutSaves := []pfccoEdge{{callerIdx: 0, calleeIdx: 1, costMatrix: [][]int{{matrix[0][0], matrix[0][1] - 21}}}}
	good, bad := pfccoSolve(controlNodes, controlEdges), pfccoSolve(controlNodes, withoutSaves)
	if good[1] != 0 || bad[1] != 1 || independentContractCost(controlNodes, controlEdges, bad) <= independentContractCost(controlNodes, controlEdges, good) {
		t.Fatal("omitted-save cost negative control failed", good, bad)
	}
	t.Logf("omitted21T-save negative control picks C at real%dT vs exact A%dT", independentContractCost(controlNodes, controlEdges, bad), independentContractCost(controlNodes, controlEdges, good))
	rng := rand.New(rand.NewSource(20261011))
	cases := 0
	for n := 1; n <= 6; n++ {
		for sample := 0; sample < 100; sample++ {
			nodes := make([]pfccoNode, n)
			for i := range nodes {
				nodes[i].costs = make([]int, 3)
				for k := 0; k < 3; k++ {
					nodes[i].costs[k] = rng.Intn(7) * 4
					if rng.Intn(11) == 0 {
						nodes[i].costs[k] = InfCost
					}
				}
			}
			var edges []pfccoEdge
			for i := 1; i < n; i++ {
				if rng.Intn(5) == 0 {
					continue
				}
				j := rng.Intn(i)
				from, to := i, j
				if rng.Intn(2) == 0 {
					from, to = to, from
				}
				m := make([][]int, 3)
				weight := rng.Intn(5) + 1
				for a := 0; a < 3; a++ {
					m[a] = make([]int, 3)
					for b := 0; b < 3; b++ {
						m[a][b] = weight * matrix[a][b]
						if rng.Intn(13) == 0 {
							m[a][b] = InfCost
						}
					}
				}
				edges = append(edges, pfccoEdge{callerIdx: from, calleeIdx: to, costMatrix: m})
			}
			best := exhaustiveContractOptimum(nodes, edges)
			got := pfccoSolve(nodes, edges)
			if best >= InfCost {
				if got != nil {
					t.Fatalf("n%d case%d infeasible accepted%v", n, sample, got)
				}
			} else if cost := independentContractCost(nodes, edges, got); cost != best {
				t.Fatalf("n%d case%d assignment%v cost%d exact%d nodes%v edges%v", n, sample, got, cost, best, nodes, edges)
			}
			cases++
		}
	}
	t.Logf("%d deterministic forest tasks, domains3, up to729 assignments each: feasible exact optima and infeasible refusal PASS", cases)
}
