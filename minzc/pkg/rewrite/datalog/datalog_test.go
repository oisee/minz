package datalog

import "testing"

func TestFactDB_Basic(t *testing.T) {
	db := NewFactDB()
	db.Add("reg", "A", "8", "acc")
	db.Add("reg", "B", "8", "gen")
	db.Add("reg", "HL", "16", "ptr")

	// Query all 8-bit regs
	results := db.Query("reg", "_", "8", "_")
	if len(results) != 2 {
		t.Fatalf("expected 2 8-bit regs, got %d", len(results))
	}

	// Query accumulator
	results = db.Query("reg", "A", "_", "acc")
	if len(results) != 1 {
		t.Fatalf("expected 1 acc, got %d", len(results))
	}

	// No match
	results = db.Query("reg", "X", "_", "_")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestParseFacts(t *testing.T) {
	src := `
;; Z80 registers
(fact reg "A" "8" "acc")
(fact reg "B" "8" "gen")
(fact reg "HL" "16" "ptr")
(fact alias "HL" "H")
(fact alias "HL" "L")
`
	db, err := ParseFacts(src)
	if err != nil {
		t.Fatal(err)
	}

	if db.Count("reg") != 3 {
		t.Errorf("expected 3 reg facts, got %d", db.Count("reg"))
	}
	if db.Count("alias") != 2 {
		t.Errorf("expected 2 alias facts, got %d", db.Count("alias"))
	}

	results := db.Query("reg", "A", "8", "acc")
	if len(results) != 1 {
		t.Errorf("expected 1 A/8/acc fact, got %d", len(results))
	}
}
