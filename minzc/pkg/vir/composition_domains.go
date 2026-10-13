package vir

// compositionAssignmentsCompatible binds shared SSA values to one location.
// Nonadjacent fragments are checked too: a value may survive across an island
// that does not use it locally. Full admission still checks its physical storage.
func compositionAssignmentsCompatible(a, b map[int]int) bool {
	for v, location := range a {
		if other, shared := b[v]; shared && other != location {
			return false
		}
	}
	return true
}

// propagateCompositionDomains enforces pairwise arc consistency. It deletes a
// candidate only if some other domain has no compatible support. Every complete
// compatible assignment therefore survives. It does not collapse domains, prove
// global satisfiability, assign registers, or replace whole-function admission.
// Input slices/maps are read-only; the returned slices own their storage.
func propagateCompositionDomains(input [][]map[int]int) [][]map[int]int {
	domains := make([][]map[int]int, len(input))
	for i, candidates := range input {
		domains[i] = append([]map[int]int(nil), candidates...)
	}
	changed := true
	for changed {
		changed = false
		for i, choices := range domains {
			retained := make([]map[int]int, 0, len(choices))
			for _, choice := range choices {
				supported := true
				for j, alternatives := range domains {
					if i == j {
						continue
					}
					found := false
					for _, alternative := range alternatives {
						if compositionAssignmentsCompatible(choice, alternative) {
							found = true
							break
						}
					}
					if !found {
						supported = false
						break
					}
				}
				if supported {
					retained = append(retained, choice)
				} else {
					changed = true
				}
			}
			domains[i] = retained
		}
	}
	return domains
}
