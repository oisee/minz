package vir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompositionRenamedHashProposal(t *testing.T) {
	dir := os.Getenv("COMPOSITION_EXPORT")
	if dir == "" {
		t.Skip("set COMPOSITION_EXPORT")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "inventory.json"))
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Function string
		Problem  *struct {
			Ops     []VIROp
			Options SolverOptions
		}
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	var p *LocalTableProblem
	for _, r := range rows {
		if r.Function == "hash3" {
			p = &LocalTableProblem{Ops: r.Problem.Ops, Options: r.Problem.Options}
			break
		}
	}
	if p == nil {
		t.Fatal("problem")
	}
	// Rename only the proposal ABI D to C; actual ABI remains A/C/D. Since actual
	// C and D coexist, this is a bijection swapping D↔C over the entire assignment.
	renamed := *p
	renamed.Options = p.Options
	renamed.Options.ParamLocs = map[int]int{}
	for v, loc := range p.Options.ParamLocs {
		if loc == Z80.LocByName("D") {
			loc = Z80.LocByName("C")
		} else if loc == Z80.LocByName("C") {
			loc = Z80.LocByName("D")
		}
		renamed.Options.ParamLocs[v] = loc
	}
	_, _, e = indexedQuotientShape(&renamed)
	if e == nil {
		t.Fatal("two simultaneous non-A ABI singletons cannot fit one historicalC singleton")
	}
	t.Log("bounded D/C permutation still fails: ", e)
	// A weaker, explicitly untrusted proposal shape may leave B as any-GPR8.
	// This is NOT a historical exact-ABI shape match. Lift then certify original.
	proposal := *p
	proposal.Options = p.Options
	proposal.Options.ParamLocs = map[int]int{}
	for v, loc := range p.Options.ParamLocs {
		if loc != Z80.LocByName("D") {
			proposal.Options.ParamLocs[v] = loc
		}
	}
	tmp := &RegAllocTable{}
	tmp.Init()
	shape, groups, e := indexedQuotientShape(&proposal)
	if e != nil {
		t.Fatal(e)
	}
	ev, e := importIndexedENRTMapped("/mnt/safe/z80-compiler/tables/enriched_4v.enr", shape, &proposal, tmp, groups)
	if e != nil || ev.Import == nil || ev.Import.Admitted != 1 {
		t.Fatal(e, ev)
	}
	assignment, _, hit := tmp.LookupChecked(p.Ops, Z80, proposal.Options)
	if !hit {
		t.Fatal("proposal")
	}
	target := &RegAllocTable{}
	target.Init()
	if e = target.AddChecked(p.Ops, assignment, Z80, p.Options); e != nil {
		t.Log("relaxed direct proposal rejected by original ABI", e)
		// Try one explicit physical namespace bijection B↔D, leaving A/C fixed.
		for v, loc := range assignment {
			if loc == Z80.LocByName("B") {
				assignment[v] = Z80.LocByName("D")
			} else if loc == Z80.LocByName("D") {
				assignment[v] = Z80.LocByName("B")
			}
		}
		if e = target.AddChecked(p.Ops, assignment, Z80, p.Options); e != nil {
			t.Log("B/D renamed proposal also rejected", e)
			return
		}
	}
	if e = target.Save(filepath.Join(dir, "hash3-proposal-table.json")); e != nil {
		t.Fatal(e)
	}
	data, _ := json.MarshalIndent(ev, "", "  ")
	os.WriteFile(filepath.Join(dir, "hash3-proposal.json"), data, 0644)
	t.Log("untrusted relaxed proposal independently admitted under exact original ABI")
}
