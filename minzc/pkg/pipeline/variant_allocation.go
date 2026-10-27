package pipeline

import (
	"sort"
	"strings"
)

// speedAllocationPathRegressionLimit is the owner's speed-mode tradeoff: a
// strictly lower loop-weighted maximum may cost at most 16 extra static T on
// any corresponding path. This allowance is only for allocation-only trials.
const speedAllocationPathRegressionLimit int64 = 16

const allocationReturn = "\x00return" // cannot collide with an assembler symbol

type allocationPath struct {
	to   string
	cost int64
}

// allocationPaths splits the encoded CFG at named labels. Branch decisions
// retain their condition and symbolic destination, independently of instruction
// addresses, register assignments and JR/JP relaxation. Refuse changed topology
// rather than comparing unrelated maximum-cost arms.
func (im *variantImage) allocationPaths(name string) (map[string]map[string]allocationPath, string, bool) {
	labels := map[int][]string{}
	for symbol, pc := range im.symbols {
		if n, ok := im.nodes[pc]; ok && n.owner == name {
			labels[pc] = append(labels[pc], symbol)
		}
	}
	anchors := map[int]string{-1: allocationReturn}
	for pc, names := range labels {
		sort.Strings(names)
		anchors[pc] = strings.Join(names, "|")
	}
	entry, exists := im.symbols[name]
	if !exists || anchors[entry] == "" || len(labels) > 4096 {
		return nil, "", false
	}
	paths := map[string]map[string]allocationPath{}
	work := 0
	for start, label := range anchors {
		if start == -1 {
			continue
		}
		out := map[string]allocationPath{}
		active := map[int]bool{}
		var walk func(int, string, int64, bool) bool
		walk = func(pc int, key string, cost int64, first bool) bool {
			work++
			if work > 16384 {
				return false
			}
			if dest := anchors[pc]; !first && dest != "" {
				key += ">" + dest
				if _, duplicate := out[key]; duplicate {
					return false
				}
				out[key] = allocationPath{dest, cost}
				return true
			}
			n, ok := im.nodes[pc]
			if !ok || n.owner != name || len(n.edges) == 0 || len(n.edges) > 2 || active[pc] {
				return false
			}
			// The scorer summarizes recognized helpers by their worst case.
			// Such a summary cannot bound every caller path's timing delta.
			if op, _ := splitVariantInstruction(n.text); op == "CALL" {
				return false
			}
			active[pc] = true
			defer delete(active, pc)
			predicate := ""
			if len(n.edges) == 2 {
				op, args := splitVariantInstruction(n.text)
				switch op {
				case "DJNZ":
					predicate = "DJNZ"
				case "JR", "JRS", "JP", "RET":
					if len(args) < 1 {
						return false
					}
					predicate = args[0]
				default:
					return false
				}
				if anchors[n.edges[0].to] == "" {
					return false
				}
				predicate += ":" + anchors[n.edges[0].to]
			}
			for i, edge := range n.edges {
				nextKey := key
				if predicate != "" {
					if i == 0 {
						nextKey += "/" + predicate + "=taken"
					} else {
						nextKey += "/" + predicate + "=fallthrough"
					}
				}
				sum, ok := addVariantCost(cost, edge.cost)
				if !ok || !walk(edge.to, nextKey, sum, false) {
					return false
				}
			}
			return true
		}
		if !walk(start, "", 0, true) {
			return nil, "", false
		}
		paths[label] = out
	}
	return paths, anchors[entry], true
}

// boundedAllocationPaths requires identical optimized MIR at its call site:
// that preserves source branch meaning while allocation changes registers.
// Matching labeled CFG paths alone cannot establish that correspondence for
// arbitrary MIR rewrites. Costs here are actual encoded instruction/edge T,
// not differences of independently maximized model scores.
//
// Max-plus relaxation bounds the sum of candidate-minus-incumbent costs over
// every entry-to-return path. A reachable positive cycle refuses the trial:
// even a small per-iteration regression could accumulate beyond the allowance.
// Fixed loops are deliberately treated conservatively too; the separate score
// still uses their proved trip counts to require a strict model improvement.
func (a *variantImage) boundedAllocationPaths(b *variantImage, name string) bool {
	x, entry, ok := a.allocationPaths(name)
	if !ok {
		return false
	}
	y, otherEntry, ok := b.allocationPaths(name)
	if !ok || entry != otherEntry || len(x) != len(y) {
		return false
	}
	type edge struct {
		from, to string
		delta    int64
	}
	var edges []edge
	var labels []string
	for label := range x {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		paths := x[label]
		other, exists := y[label]
		if !exists || len(paths) != len(other) {
			return false
		}
		var keys []string
		for key := range paths {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			p := paths[key]
			q, exists := other[key]
			if !exists || p.to != q.to {
				return false
			}
			edges = append(edges, edge{label, p.to, q.cost - p.cost})
		}
	}
	distance := map[string]int64{entry: 0}
	// There are len(x)+1 vertices including return. After V-1 passes any
	// further increase implies a reachable positive cycle. Stable edge order
	// keeps this refusing bound deterministic across compiler runs.
	for pass := 0; pass <= len(x); pass++ {
		changed := false
		for _, e := range edges {
			d, reached := distance[e.from]
			if !reached {
				continue
			}
			next := d + e.delta
			if next < -variantCostLimit || next > variantCostLimit {
				return false
			}
			if old, seen := distance[e.to]; !seen || next > old {
				distance[e.to] = next
				changed = true
			}
		}
		if !changed {
			d, returned := distance[allocationReturn]
			return returned && d <= speedAllocationPathRegressionLimit
		}
	}
	return false
}
