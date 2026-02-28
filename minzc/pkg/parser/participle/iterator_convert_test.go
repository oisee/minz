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

// --- Argument field routing tests ---

func TestTakeArgumentRouting(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 10] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
	arr.iter().take(5).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	takeOp := chain.Operations[0]
	if takeOp.Type != ast.IterOpTake {
		t.Fatalf("op[0]: expected Take, got %v", takeOp.Type)
	}
	// Argument should hold the number, Function should be nil
	if takeOp.Function != nil {
		t.Errorf("take(5): Function should be nil, got %T", takeOp.Function)
	}
	if takeOp.Argument == nil {
		t.Fatal("take(5): Argument should not be nil")
	}
	lit, ok := takeOp.Argument.(*ast.NumberLiteral)
	if !ok {
		t.Fatalf("take(5): Argument should be *NumberLiteral, got %T", takeOp.Argument)
	}
	if lit.Value != 5 {
		t.Errorf("take(5): expected value 5, got %d", lit.Value)
	}
}

func TestSkipArgumentRouting(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 10] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
	arr.iter().skip(2).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	skipOp := chain.Operations[0]
	if skipOp.Type != ast.IterOpSkip {
		t.Fatalf("op[0]: expected Skip, got %v", skipOp.Type)
	}
	if skipOp.Function != nil {
		t.Errorf("skip(2): Function should be nil, got %T", skipOp.Function)
	}
	if skipOp.Argument == nil {
		t.Fatal("skip(2): Argument should not be nil")
	}
	lit, ok := skipOp.Argument.(*ast.NumberLiteral)
	if !ok {
		t.Fatalf("skip(2): Argument should be *NumberLiteral, got %T", skipOp.Argument)
	}
	if lit.Value != 2 {
		t.Errorf("skip(2): expected value 2, got %d", lit.Value)
	}
}

func TestEnumerateNoArguments(t *testing.T) {
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
	enumOp := chain.Operations[0]
	if enumOp.Type != ast.IterOpEnumerate {
		t.Fatalf("op[0]: expected Enumerate, got %v", enumOp.Type)
	}
	if enumOp.Function != nil {
		t.Errorf("enumerate(): Function should be nil, got %T", enumOp.Function)
	}
	if enumOp.Argument != nil {
		t.Errorf("enumerate(): Argument should be nil, got %T", enumOp.Argument)
	}
}

func TestReduceWithFunctionRouting(t *testing.T) {
	source := `
fun sum(acc: u8, x: u8) -> u8 { return acc + x; }

fun f() {
	let arr: [u8; 5] = [1, 2, 3, 4, 5];
	arr.iter().reduce(sum);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	reduceOp := chain.Operations[0]
	if reduceOp.Type != ast.IterOpReduce {
		t.Fatalf("op[0]: expected Reduce, got %v", reduceOp.Type)
	}
	// Single-arg reduce: fn goes to Function, Argument is nil
	if reduceOp.Function == nil {
		t.Error("reduce(sum): Function should not be nil")
	}
	if reduceOp.Argument != nil {
		t.Errorf("reduce(sum): Argument should be nil (no init value), got %T", reduceOp.Argument)
	}
}

func TestReduceWithInitAndFunction(t *testing.T) {
	source := `
fun f() {
	let arr: [u8; 5] = [1, 2, 3, 4, 5];
	arr.iter().reduce(0, |acc, x| => u8 { acc + x });
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	reduceOp := chain.Operations[0]
	if reduceOp.Type != ast.IterOpReduce {
		t.Fatalf("op[0]: expected Reduce, got %v", reduceOp.Type)
	}
	// Two-arg reduce: init → Argument, fn → Function
	if reduceOp.Argument == nil {
		t.Fatal("reduce(0, fn): Argument (init value) should not be nil")
	}
	if reduceOp.Function == nil {
		t.Fatal("reduce(0, fn): Function should not be nil")
	}
	// Init value should be 0
	lit, ok := reduceOp.Argument.(*ast.NumberLiteral)
	if !ok {
		t.Fatalf("reduce init: expected *NumberLiteral, got %T", reduceOp.Argument)
	}
	if lit.Value != 0 {
		t.Errorf("reduce init: expected 0, got %d", lit.Value)
	}
	// Function should be a lambda
	if _, ok := reduceOp.Function.(*ast.LambdaExpr); !ok {
		t.Errorf("reduce fn: expected *LambdaExpr, got %T", reduceOp.Function)
	}
}

func TestMapFunctionInFunctionField(t *testing.T) {
	// Verify map/filter/forEach still use Function field (not Argument)
	source := `
fun double(x: u8) -> u8 { return x * 2; }

fun f() {
	let arr: [u8; 3] = [1, 2, 3];
	arr.iter().map(double).forEach(print_u8);
}
`
	file := parseAndConvert(t, source)
	chain := findIteratorChain(file)
	if chain == nil {
		t.Fatal("expected IteratorChainExpr, got nil")
	}
	mapOp := chain.Operations[0]
	if mapOp.Type != ast.IterOpMap {
		t.Fatalf("op[0]: expected Map, got %v", mapOp.Type)
	}
	if mapOp.Function == nil {
		t.Error("map(double): Function should not be nil")
	}
	if mapOp.Argument != nil {
		t.Errorf("map(double): Argument should be nil, got %T", mapOp.Argument)
	}
}
