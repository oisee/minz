package mir2

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// dominatorsByRemoval is the definition: d dominates b when b is reachable
// from the entry, but not once d is removed from the graph.
func dominatorsByRemoval(f *Func) map[string]map[string]bool {
	blocks := map[string]*Block{}
	for _, b := range f.Blocks {
		blocks[b.Label] = b
	}
	reach := func(without string) map[string]bool {
		seen := map[string]bool{}
		var walk func(label string)
		walk = func(label string) {
			if label == without || seen[label] {
				return
			}
			seen[label] = true
			for _, s := range blocks[label].Successors() {
				walk(s)
			}
		}
		walk(f.Blocks[0].Label)
		return seen
	}
	all := reach("")
	dom := map[string]map[string]bool{}
	for b := range all {
		dom[b] = map[string]bool{b: true}
	}
	for d := range all {
		kept := reach(d)
		for b := range all {
			if !kept[b] {
				dom[b][d] = true
			}
		}
	}
	return dom
}

func TestReachableDominatorsMatchDefinition(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for round := 0; round < 3000; round++ {
		n := 1 + rng.Intn(24)
		f := &Func{Name: "g"}
		label := func(i int) string { return fmt.Sprintf("b%d", i) }
		// Low fan-out towards later blocks gives long dominator chains;
		// arbitrary targets add back edges, irreducible regions, edges into
		// the entry and unreachable blocks.
		target := func(i int) string {
			if rng.Intn(3) == 0 {
				return label(rng.Intn(n))
			}
			return label(min(n-1, i+1+rng.Intn(3)))
		}
		for i := 0; i < n; i++ {
			b := &Block{Label: label(i)}
			switch rng.Intn(6) {
			case 0:
				b.Term = &TermRet{}
			case 1, 2:
				b.Term = &TermJmp{Target: target(i)}
			default:
				b.Term = &TermBrIf{Then: target(i), Else: target(i)}
			}
			f.Blocks = append(f.Blocks, b)
		}
		got, want := reachableDominators(f), dominatorsByRemoval(f)
		if !reflect.DeepEqual(got, want) {
			var edges []string
			for _, b := range f.Blocks {
				edges = append(edges, fmt.Sprintf("%s -> %v", b.Label, b.Successors()))
			}
			t.Fatalf("round %d: %v\ngot  %v\nwant %v", round, edges, got, want)
		}
	}
}

// A chain of n two-block diamonds has dominator sets of total size
// proportional to n squared; the test fails by timeout if computing them
// costs more than that.
func TestReachableDominatorsLongChain(t *testing.T) {
	const n = 4000
	f := &Func{Name: "chain"}
	for i := 0; i < n; i++ {
		f.Blocks = append(f.Blocks,
			&Block{Label: fmt.Sprintf("h%d", i), Term: &TermBrIf{Then: fmt.Sprintf("s%d", i), Else: fmt.Sprintf("h%d", i+1)}},
			&Block{Label: fmt.Sprintf("s%d", i), Term: &TermJmp{Target: fmt.Sprintf("h%d", i+1)}})
	}
	f.Blocks = append(f.Blocks, &Block{Label: fmt.Sprintf("h%d", n), Term: &TermRet{}})
	dom := reachableDominators(f)
	if len(dom) != 2*n+1 {
		t.Fatalf("%d reachable blocks, want %d", len(dom), 2*n+1)
	}
	last := dom[fmt.Sprintf("h%d", n)]
	if len(last) != n+1 || last["s0"] || !last["h0"] {
		t.Fatalf("exit block has %d dominators, want the %d chain heads", len(last), n+1)
	}
}
