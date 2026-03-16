package lir

import (
	"testing"
)

func TestZ80Facts_ForbiddenDD(t *testing.T) {
	db := Z80Facts()

	// Query: all forbidden combinations
	forbidden := db.Query("forbidden", "_", "_")
	if len(forbidden) < 6 {
		t.Errorf("expected >=6 forbidden facts, got %d", len(forbidden))
	}

	// Specific: IXH with ix_indirect is forbidden
	ixhForbid := db.Query("forbidden", "IXH", "ix_indirect")
	if len(ixhForbid) != 1 {
		t.Errorf("IXH+ix_indirect: expected 1 fact, got %d", len(ixhForbid))
	}

	// H with IXH is forbidden (DD prefix NOP)
	hIxh := db.Query("forbidden", "H", "IXH")
	if len(hIxh) != 1 {
		t.Errorf("H+IXH: expected 1 fact, got %d", len(hIxh))
	}

	t.Logf("Z80 has %d forbidden DD-prefix combos", len(forbidden))
}

func TestZ80Facts_Aliases(t *testing.T) {
	db := Z80Facts()

	// HL aliases H and L
	hlAliases := db.Query("alias", "HL", "_")
	if len(hlAliases) != 2 {
		t.Errorf("HL aliases: expected 2 (H,L), got %d", len(hlAliases))
	}

	// IX aliases IXH and IXL
	ixAliases := db.Query("alias", "IX", "_")
	if len(ixAliases) != 2 {
		t.Errorf("IX aliases: expected 2 (IXH,IXL), got %d", len(ixAliases))
	}
}

func TestZ80Facts_AccOnly(t *testing.T) {
	db := Z80Facts()

	accOps := db.Query("acc_only", "_", "_")
	if len(accOps) < 7 {
		t.Errorf("expected >=7 acc-only ops, got %d", len(accOps))
	}

	// Verify ADD8 requires A
	add8 := db.Query("acc_only", "ADD8", "_")
	if len(add8) != 1 || add8[0].Args[1] != "A" {
		t.Errorf("ADD8 acc requirement: %v", add8)
	}
}

func TestKnownPatterns(t *testing.T) {
	patterns := KnownPatterns()

	if len(patterns) < 10 {
		t.Fatalf("expected >=10 known patterns, got %d", len(patterns))
	}

	// Check specific patterns exist
	names := make(map[string]bool)
	for _, p := range patterns {
		names[p.Name] = true
		t.Logf("pattern: %s (%d nodes, %d edges, %d where)",
			p.Name, len(p.Nodes), len(p.Edges), len(p.Where))
	}

	expected := []string{
		"sub_cmp_fusion", "split_join_ret", "cond_ret_sink",
		"sub_swap_neg", "abs_diff_fusion", "inc_dec",
		"cp_zero_to_or", "dead_ret_ret", "store_load_forward",
		"djnz_fusion",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing pattern: %s", name)
		}
	}
}

func TestKnownRewrites(t *testing.T) {
	rewrites := KnownRewrites()
	if len(rewrites) < 4 {
		t.Fatalf("expected >=4 rewrites, got %d", len(rewrites))
	}
	for _, r := range rewrites {
		t.Logf("rewrite: %s → %d actions", r.Pattern.Name, len(r.Actions))
	}
}
