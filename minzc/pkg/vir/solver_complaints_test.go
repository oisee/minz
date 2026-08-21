package vir

import "testing"

// TestSolverComplaints pins the check that stops us accepting a model z3
// produced after discarding part of the query. PFCCO emits a bare top-level
// (ite ...); z3 answers "unsupported", drops the bool-return cost term, and
// still reports sat — so the old code returned a solution to a different
// problem and nobody noticed.
func TestSolverComplaints(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   int
	}{
		{"clean sat", "sat\n(model (define-fun x () Int 3))", 0},
		{"clean unsat", "unsat", 0},
		{"unsupported alone", "unsupported\nsat\n(model )", 1},
		{"unsupported with detail", "unsupported\n; ite line: 3 position: 27\nsat", 1},
		{"error line", "(error \"line 5 column 2: unknown constant\")\nsat", 1},
		{"both", "unsupported\n(error \"bad\")\nsat", 2},
		{"empty", "", 0},
		// "unsupported" must not be matched inside ordinary model text.
		{"word in model", "sat\n(model (define-fun unsupported_flag () Bool true))", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(solverComplaints(tc.output)); got != tc.want {
				t.Errorf("solverComplaints(%q) found %d, want %d", tc.output, got, tc.want)
			}
		})
	}
}
