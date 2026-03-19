package sexpr

import "testing"

func TestParse_Basic(t *testing.T) {
	nodes, err := Parse(`(hello world) (foo 42)`)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if nodes[0].String() != "(hello world)" {
		t.Errorf("node 0: got %s", nodes[0])
	}
}

func TestParse_Nested(t *testing.T) {
	nodes, err := Parse(`(rule (add ?x (const 1)) (inc ?x))`)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	if !nodes[0].IsList() || len(nodes[0].List) != 3 {
		t.Fatalf("expected list of 3, got %s", nodes[0])
	}
}

func TestParse_Comments(t *testing.T) {
	nodes, err := Parse(`;; comment
(foo bar)
;; another comment
(baz)`)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
}

func TestParse_String(t *testing.T) {
	nodes, err := Parse(`(term-kind "br_if")`)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatal("expected 1 node")
	}
	if nodes[0].List[1].Atom != `"br_if"` {
		t.Errorf("string atom: got %q", nodes[0].List[1].Atom)
	}
}

func TestParse_Empty(t *testing.T) {
	nodes, err := Parse(``)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("got %d nodes, want 0", len(nodes))
	}
}

func TestParse_Unterminated(t *testing.T) {
	_, err := Parse(`(hello`)
	if err == nil {
		t.Fatal("expected error for unterminated list")
	}
}
