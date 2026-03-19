// Package datalog provides a simple Datalog-style fact database with
// query support, extracted from pkg/lir/rules.go for cross-IR reuse.
package datalog

// Fact is a Datalog-style fact about the target or IR.
type Fact struct {
	Predicate string   // "reg", "forbidden", "alias", "clobber", "width"
	Args      []string // predicate arguments
}

// FactDB is a collection of facts, indexed by predicate for fast lookup.
type FactDB struct {
	facts map[string][]Fact // predicate → facts
}

// NewFactDB creates an empty fact database.
func NewFactDB() *FactDB {
	return &FactDB{facts: make(map[string][]Fact)}
}

// Add adds a fact to the database.
func (db *FactDB) Add(pred string, args ...string) {
	db.facts[pred] = append(db.facts[pred], Fact{Predicate: pred, Args: args})
}

// Query returns all facts matching the predicate. Args with "_" are wildcards.
func (db *FactDB) Query(pred string, args ...string) []Fact {
	var results []Fact
	for _, f := range db.facts[pred] {
		if matchArgs(f.Args, args) {
			results = append(results, f)
		}
	}
	return results
}

// All returns all facts for a predicate.
func (db *FactDB) All(pred string) []Fact {
	return db.facts[pred]
}

// Count returns the number of facts for a predicate.
func (db *FactDB) Count(pred string) int {
	return len(db.facts[pred])
}

func matchArgs(fact, query []string) bool {
	if len(fact) != len(query) {
		return false
	}
	for i, q := range query {
		if q != "_" && q != fact[i] {
			return false
		}
	}
	return true
}
