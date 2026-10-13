package vir

import "fmt"

// Layouts returns up to four deterministic proposals: source order, false-first
// DFS, true-first DFS and returns-last. These are bounded candidates, not an
// exhaustive permutation solver or a claim of optimal layout. Duplicate orders
// are removed. The validated source contract and all edges remain unchanged.
func (p *ConditionalTableProblem) Layouts(limit int) ([][]int, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 4 {
		return nil, fmt.Errorf("conditional layout limit must be 1..4")
	}
	original := make([]int, len(p.Blocks))
	indices := map[string]int{}
	for i, node := range p.Blocks {
		original[i] = i
		indices[node.Label] = i
	}
	candidates := [][]int{original}
	for _, trueFirst := range []bool{false, true} {
		visited := map[int]bool{}
		order := []int{}
		var visit func(int)
		visit = func(index int) {
			if visited[index] {
				return
			}
			visited[index] = true
			order = append(order, index)
			node := p.Blocks[index]
			if node.Jump != "" {
				visit(indices[node.Jump])
			}
			if node.Compare {
				if trueFirst {
					visit(indices[node.Then])
				}
				if !node.FalseReturn {
					visit(indices[node.Else])
				}
				if !trueFirst {
					visit(indices[node.Then])
				}
			}
		}
		visit(0)
		candidates = append(candidates, order)
	}
	last := []int{0}
	for i := 1; i < len(p.Blocks); i++ {
		if !p.Blocks[i].Return {
			last = append(last, i)
		}
	}
	for i := 1; i < len(p.Blocks); i++ {
		if p.Blocks[i].Return {
			last = append(last, i)
		}
	}
	candidates = append(candidates, last)
	result := [][]int{}
	seen := map[string]bool{}
	for _, order := range candidates {
		key := fmt.Sprint(order)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, order)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}
