package participle

import (
	"testing"

	"github.com/minz/minzc/pkg/ast"
)

// findIteratorChain walks the AST to find the first IteratorChainExpr
func findIteratorChain(node ast.Node) *ast.IteratorChainExpr {
	switch n := node.(type) {
	case *ast.File:
		for _, d := range n.Declarations {
			if chain := findIteratorChain(d); chain != nil {
				return chain
			}
		}
	case *ast.FunctionDecl:
		if n.Body != nil {
			return findIteratorChainInBlock(n.Body)
		}
	}
	return nil
}

func findIteratorChainInBlock(block *ast.BlockStmt) *ast.IteratorChainExpr {
	for _, s := range block.Statements {
		if chain := findIteratorChainInStmt(s); chain != nil {
			return chain
		}
	}
	return nil
}

func findIteratorChainInStmt(stmt ast.Statement) *ast.IteratorChainExpr {
	switch s := stmt.(type) {
	case *ast.ExpressionStmt:
		if chain, ok := s.Expression.(*ast.IteratorChainExpr); ok {
			return chain
		}
	case *ast.VarDecl:
		if chain, ok := s.Value.(*ast.IteratorChainExpr); ok {
			return chain
		}
	case *ast.ForStmt:
		if chain, ok := s.Range.(*ast.IteratorChainExpr); ok {
			return chain
		}
	}
	return nil
}

func parseAndConvert(t *testing.T, source string) *ast.File {
	t.Helper()
	parser := New()
	pFile, err := parser.ParseString("test.minz", source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	converter := NewConverter("test.minz")
	astFile, err := converter.Convert(pFile)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	return astFile
}

func TestIteratorChainBasicForEach(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 5] = [1, 2, 3, 4, 5];
	arr.forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpForEach {
		t.Errorf("expected ForEach op, got %v", chain.Operations[0].Type)
	}
}

func TestIteratorChainMapWithLambda(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 3] = [1, 2, 3];
	arr.iter().map(|x| => u8 { x * 2 }).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	// .iter() should be stripped, leaving map + forEach
	if len(chain.Operations) != 2 {
		t.Fatalf("expected 2 operations (map, forEach), got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpMap {
		t.Errorf("expected Map op first, got %v", chain.Operations[0].Type)
	}
	if chain.Operations[1].Type != ast.IterOpForEach {
		t.Errorf("expected ForEach op second, got %v", chain.Operations[1].Type)
	}
	// Map should have a lambda function
	if chain.Operations[0].Function == nil {
		t.Error("expected map to have a lambda function")
	}
}

func TestIteratorChainFilterForEach(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 5] = [1, 2, 3, 4, 5];
	arr.iter().filter(|x| => bool { x > 5 }).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 2 {
		t.Fatalf("expected 2 operations (filter, forEach), got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpFilter {
		t.Errorf("expected Filter op, got %v", chain.Operations[0].Type)
	}
	if chain.Operations[1].Type != ast.IterOpForEach {
		t.Errorf("expected ForEach op, got %v", chain.Operations[1].Type)
	}
}

func TestIteratorChainMapFilterReduce(t *testing.T) {
	source := `
fun double(x: u8) -> u8 { return x * 2; }
fun is_big(x: u8) -> bool { return x > 5; }
fun sum(acc: u8, x: u8) -> u8 { return acc + x; }

fun f() {
	let arr: [u8; 5] = [1, 2, 3, 4, 5];
	arr.iter().map(double).filter(is_big).reduce(sum);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpMap {
		t.Errorf("op[0]: expected Map, got %v", chain.Operations[0].Type)
	}
	if chain.Operations[1].Type != ast.IterOpFilter {
		t.Errorf("op[1]: expected Filter, got %v", chain.Operations[1].Type)
	}
	if chain.Operations[2].Type != ast.IterOpReduce {
		t.Errorf("op[2]: expected Reduce, got %v", chain.Operations[2].Type)
	}
}

func TestIteratorChainTakeSkip(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 10] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
	arr.iter().take(5).skip(2).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 3 {
		t.Fatalf("expected 3 operations (take, skip, forEach), got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpTake {
		t.Errorf("op[0]: expected Take, got %v", chain.Operations[0].Type)
	}
	if chain.Operations[1].Type != ast.IterOpSkip {
		t.Errorf("op[1]: expected Skip, got %v", chain.Operations[1].Type)
	}
	if chain.Operations[2].Type != ast.IterOpForEach {
		t.Errorf("op[2]: expected ForEach, got %v", chain.Operations[2].Type)
	}
}

func TestIteratorChainZip(t *testing.T) {
	source := `
fun f() {
	let a: [u8; 3] = [1, 2, 3];
	let b: [u8; 3] = [4, 5, 6];
	a.iter().zip(b).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 2 {
		t.Fatalf("expected 2 operations (zip, forEach), got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpZip {
		t.Errorf("op[0]: expected Zip, got %v", chain.Operations[0].Type)
	}
	// Zip should have the second array as its "function" argument
	if chain.Operations[0].Function == nil {
		t.Error("expected zip to have an argument (second source)")
	}
}

func TestIteratorChainEnumerate(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 3] = [1, 2, 3];
	arr.iter().enumerate().forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 2 {
		t.Fatalf("expected 2 operations (enumerate, forEach), got %d", len(chain.Operations))
	}
	if chain.Operations[0].Type != ast.IterOpEnumerate {
		t.Errorf("op[0]: expected Enumerate, got %v", chain.Operations[0].Type)
	}
	// Enumerate takes no arguments
	if chain.Operations[0].Function != nil {
		t.Error("expected enumerate to have no function argument")
	}
}

func TestNonIteratorMethodNotConverted(t *testing.T) {
	source := `
fun f() {
	let x = obj.method();
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain != nil {
		t.Error("expected no IteratorChainExpr for non-iterator method, got one")
	}
}

func TestIterAloneNoChain(t *testing.T) {
	// .iter() alone (no subsequent operations) is stripped and produces no chain.
	// This is correct — a bare .iter() with no map/filter/forEach is a no-op.
	source := `
fun f() {
	let arr: [u8; 3] = [1, 2, 3];
	arr.iter();
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain != nil {
		t.Errorf("expected no IteratorChainExpr for bare .iter(), got one with %d ops", len(chain.Operations))
	}
}

func TestIteratorChainDirectMethods(t *testing.T) {
	// Without .iter() — direct method chains should also work
	source := `
fun double(x: u8) -> u8 { return x * 2; }

fun f() {
	let arr: [u8; 3] = [1, 2, 3];
	arr.map(double).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	if len(chain.Operations) != 2 {
		t.Fatalf("expected 2 operations (map, forEach), got %d", len(chain.Operations))
	}
}

func TestIteratorOpTypeMapping(t *testing.T) {
	tests := []struct {
		method   string
		expected ast.IteratorOpType
	}{
		{"map", ast.IterOpMap},
		{"filter", ast.IterOpFilter},
		{"forEach", ast.IterOpForEach},
		{"reduce", ast.IterOpReduce},
		{"collect", ast.IterOpCollect},
		{"take", ast.IterOpTake},
		{"skip", ast.IterOpSkip},
		{"zip", ast.IterOpZip},
		{"enumerate", ast.IterOpEnumerate},
		{"flatMap", ast.IterOpFlatMap},
		{"takeWhile", ast.IterOpTakeWhile},
		{"skipWhile", ast.IterOpSkipWhile},
		{"peek", ast.IterOpPeek},
		{"inspect", ast.IterOpInspect},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := getIteratorOpType(tt.method)
			if got != tt.expected {
				t.Errorf("getIteratorOpType(%q) = %v, want %v", tt.method, got, tt.expected)
			}
		})
	}
}

func TestIsIteratorMethod(t *testing.T) {
	iteratorMethods := []string{
		"iter", "map", "filter", "forEach", "reduce", "collect",
		"take", "skip", "zip", "enumerate", "flatMap", "takeWhile",
		"skipWhile", "peek", "inspect", "sum", "count", "any", "all",
		"find", "first", "last",
	}
	for _, m := range iteratorMethods {
		if !isIteratorMethod(m) {
			t.Errorf("isIteratorMethod(%q) = false, want true", m)
		}
	}

	nonIteratorMethods := []string{"toString", "length", "push", "pop", "get", "set"}
	for _, m := range nonIteratorMethods {
		if isIteratorMethod(m) {
			t.Errorf("isIteratorMethod(%q) = true, want false", m)
		}
	}
}
