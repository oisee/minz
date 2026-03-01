package semantic

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/parser"
)

// compileIteratorToMIR compiles a MinZ source with iterator patterns to MIR
func compileIteratorToMIR(t *testing.T, source string) *ir.Module {
	t.Helper()
	p := parser.New()
	decls, err := p.ParseString(source, "test.minz")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	astFile := &ast.File{
		Name:         "test.minz",
		Declarations: decls,
	}
	analyzer := NewAnalyzer()
	analyzer.SetTargetBackend("mir")
	module, err := analyzer.Analyze(astFile)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	return module
}

// findMainFunction finds the main function in a module
func findMainFunction(module *ir.Module) *ir.Function {
	for _, fn := range module.Functions {
		if fn.Name == "main" || strings.HasSuffix(fn.Name, ".main") {
			return fn
		}
	}
	return nil
}

// countInstructions counts instructions matching a predicate
func countInstructions(fn *ir.Function, pred func(ir.Instruction) bool) int {
	count := 0
	for _, inst := range fn.Instructions {
		if pred(inst) {
			count++
		}
	}
	return count
}

// hasOpcode checks if a function contains a specific opcode
func hasOpcode(fn *ir.Function, op ir.Opcode) bool {
	return countInstructions(fn, func(inst ir.Instruction) bool {
		return inst.Op == op
	}) > 0
}

// --- DJNZ Eligibility Tests ---

func TestDJNZForSmallArray(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let numbers: [u8; 5] = [1, 2, 3, 4, 5];
    numbers.forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Array of 5 elements should use DJNZ
	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected OpDJNZ for array[5], got standard loop")
		t.Log("Instructions:")
		for i, inst := range mainFn.Instructions {
			t.Logf("  %d: %s", i, inst.String())
		}
	}
}

func TestDJNZForMaxArray(t *testing.T) {
	// Verify the DJNZ eligibility threshold at the IR level.
	// We can't parse [u8; 255] with fill syntax, so we check the
	// semantic logic: generateArrayIteration uses DJNZ when Length <= 255.
	// Test with a smaller array to verify the codegen path works.
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let big: [u8; 10] = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
    big.forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected OpDJNZ for array[10] (within 255 threshold)")
	}
}

// --- Iterator Chain Analysis Tests ---

func TestForEachGeneratesCall(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 3] = [1, 2, 3];
    nums.forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should have a CALL to print_u8
	hasPrintCall := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpCall && strings.Contains(inst.Symbol, "print_u8") {
			hasPrintCall = true
			break
		}
	}
	if !hasPrintCall {
		t.Error("expected OpCall to print_u8 in forEach body")
	}
}

func TestMapWithFunctionRef(t *testing.T) {
	source := `
fun double(x: u8) -> u8 { return x * 2; }
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 3] = [1, 2, 3];
    nums.map(double).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should have calls to both double and print_u8
	hasDoubleCall := false
	hasPrintCall := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpCall {
			if strings.Contains(inst.Symbol, "double") {
				hasDoubleCall = true
			}
			if strings.Contains(inst.Symbol, "print_u8") {
				hasPrintCall = true
			}
		}
	}
	if !hasDoubleCall {
		t.Error("expected OpCall to double in map body")
	}
	if !hasPrintCall {
		t.Error("expected OpCall to print_u8 in forEach body")
	}
}

func TestFilterGeneratesConditionalJump(t *testing.T) {
	source := `
fun is_big(x: u8) -> bool { return x > 3; }
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.filter(is_big).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Filter should generate a JumpIfNot (skip when predicate is false)
	hasConditionalJump := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfNot {
			hasConditionalJump = true
			break
		}
	}
	if !hasConditionalJump {
		t.Error("expected OpJumpIfNot for filter predicate")
		t.Log("Instructions:")
		for i, inst := range mainFn.Instructions {
			t.Logf("  %d: %s", i, inst.String())
		}
	}
}

func TestMapFilterForEachChain(t *testing.T) {
	source := `
fun double(x: u8) -> u8 { return x * 2; }
fun is_big(x: u8) -> bool { return x > 5; }
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.map(double).filter(is_big).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should have all three types of operations in the fused loop
	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected DJNZ optimization for fused map+filter+forEach")
	}

	// Verify all calls are present
	calls := map[string]bool{}
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpCall {
			calls[inst.Symbol] = true
		}
	}

	for _, expected := range []string{"double", "is_big", "print_u8"} {
		found := false
		for sym := range calls {
			if strings.Contains(sym, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected call to %s in fused chain", expected)
		}
	}
}

// --- Lambda Tests ---

func TestMapWithLambda(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 3] = [1, 2, 3];
    nums.iter().map(|x| => u8 { x * 2 }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Lambda should be extracted as a separate function
	hasLambdaFunc := false
	for _, fn := range module.Functions {
		if strings.Contains(fn.Name, "iter_lambda") {
			hasLambdaFunc = true
			break
		}
	}
	if !hasLambdaFunc {
		t.Error("expected lambda to be extracted as separate function named iter_lambda_*")
		t.Log("Functions in module:")
		for _, fn := range module.Functions {
			t.Logf("  %s", fn.Name)
		}
	}
}

func TestFilterWithLambda(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().filter(|x| => bool { x > 3 }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Simple comparison lambda (x > 3) should be inlined as OpJumpIfFlag (ADR-0008)
	// No lambda function extraction, no OpCall — direct CP + JR
	if !hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("expected OpJumpIfFlag for inline filter lambda |x| x > 3")
	}

	// Should NOT have a lambda function for this simple comparison
	hasLambdaFunc := false
	for _, fn := range module.Functions {
		if strings.Contains(fn.Name, "iter_lambda") {
			hasLambdaFunc = true
			break
		}
	}
	if hasLambdaFunc {
		t.Error("simple comparison lambda should NOT be extracted as separate function")
	}
}

// --- Iter Stripping Tests ---

func TestIterStrippedFromChain(t *testing.T) {
	// .iter() should not appear as an operation — it's just a chain marker
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 3] = [1, 2, 3];
    nums.iter().forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// The key test: .iter() is stripped, so the chain should compile
	// successfully with just forEach. If .iter() were treated as an
	// operation, it would fail with "map requires a function".
	if !hasOpcode(mainFn, ir.OpCall) {
		t.Error("expected OpCall for forEach callback after .iter() stripping")
	}
}

// --- Loop Structure Tests ---

func TestDJNZLoopStructure(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Find the DJNZ instruction
	djnzIdx := -1
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpDJNZ {
			djnzIdx = i
			break
		}
	}
	if djnzIdx < 0 {
		t.Fatal("DJNZ instruction not found")
	}

	djnz := mainFn.Instructions[djnzIdx]

	// DJNZ should jump to a label that precedes it (backward branch)
	labelIdx := -1
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpLabel && inst.Label == djnz.Label {
			labelIdx = i
			break
		}
	}
	if labelIdx < 0 {
		t.Fatalf("DJNZ target label %q not found", djnz.Label)
	}
	if labelIdx >= djnzIdx {
		t.Errorf("DJNZ label at idx %d should precede DJNZ at idx %d (backward branch)", labelIdx, djnzIdx)
	}
}

func TestDJNZCounterInit(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Find DJNZ to get the counter register
	var djnzInst *ir.Instruction
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpDJNZ {
			djnzInst = &mainFn.Instructions[i]
			break
		}
	}
	if djnzInst == nil {
		t.Fatal("DJNZ instruction not found")
	}

	// Find the LoadConst that initializes the counter register
	counterReg := djnzInst.Src1
	foundInit := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpLoadConst && inst.Dest == counterReg && inst.Imm == 5 {
			foundInit = true
			break
		}
	}
	if !foundInit {
		t.Errorf("expected LoadConst initializing r%d to 5 (array length)", counterReg)
	}
}

// --- Element Loading Tests ---

func TestElementLoadedViaPointer(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 3] = [1, 2, 3];
    nums.forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should have an OpLoad instruction (load element through pointer)
	if !hasOpcode(mainFn, ir.OpLoad) {
		t.Error("expected OpLoad instruction for element loading via pointer")
	}

	// Should have an OpInc instruction (pointer increment)
	if !hasOpcode(mainFn, ir.OpInc) {
		t.Error("expected OpInc instruction for pointer advancement")
	}
}

// --- Filter Label Placement ---

func TestFilterContinueLabelAfterBody(t *testing.T) {
	source := `
fun is_big(x: u8) -> bool { return x > 3; }
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.filter(is_big).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Find the JumpIfNot (filter skip) and its target label
	var filterJump *ir.Instruction
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfNot && strings.Contains(inst.Label, "filter") {
			filterJump = &mainFn.Instructions[i]
			break
		}
	}
	if filterJump == nil {
		t.Fatal("filter JumpIfNot not found")
	}

	// Find the continue label
	continueLabelIdx := -1
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpLabel && inst.Label == filterJump.Label {
			continueLabelIdx = i
			break
		}
	}
	if continueLabelIdx < 0 {
		t.Fatalf("filter continue label %q not found", filterJump.Label)
	}

	// The continue label should be placed after the forEach call
	// (it's where we jump to skip the body when filter fails)
	printCallIdx := -1
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpCall && strings.Contains(inst.Symbol, "print_u8") {
			printCallIdx = i
			break
		}
	}
	if printCallIdx < 0 {
		t.Fatal("print_u8 call not found")
	}

	if continueLabelIdx < printCallIdx {
		t.Errorf("filter continue label (idx %d) should come after print_u8 call (idx %d)",
			continueLabelIdx, printCallIdx)
	}
}

// --- Enhanced Iterator Operations ---

func TestTakeCompiles(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 10] = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
    nums.iter().take(3).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should use DJNZ with counter = 3 (take count)
	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected DJNZ for take(3)")
	}

	// Find counter init — should be 3, not 10
	var djnzInst *ir.Instruction
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpDJNZ {
			djnzInst = &mainFn.Instructions[i]
			break
		}
	}
	if djnzInst == nil {
		t.Fatal("DJNZ instruction not found")
	}
	counterReg := djnzInst.Src1
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpLoadConst && inst.Dest == counterReg {
			if inst.Imm != 3 {
				t.Errorf("expected DJNZ counter = 3 (take count), got %d", inst.Imm)
			}
			break
		}
	}
}

func TestSkipCompiles(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 10] = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
    nums.iter().skip(2).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected DJNZ for skip(2)")
	}

	// Should have a comment mentioning skip=2
	hasSkipComment := false
	for _, inst := range mainFn.Instructions {
		if strings.Contains(inst.Comment, "skip=2") {
			hasSkipComment = true
			break
		}
	}
	if !hasSkipComment {
		t.Error("expected enhanced DJNZ comment mentioning skip=2")
	}
}

func TestTakeSkipCombined(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 10] = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
    nums.iter().skip(2).take(5).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected DJNZ for skip(2).take(5)")
	}

	// Should have enhanced DJNZ comment with both skip and take
	hasEnhancedComment := false
	for _, inst := range mainFn.Instructions {
		if strings.Contains(inst.Comment, "skip=2") && strings.Contains(inst.Comment, "take=5") {
			hasEnhancedComment = true
			break
		}
	}
	if !hasEnhancedComment {
		t.Error("expected enhanced DJNZ comment with skip=2 and take=5")
	}
}

func TestEnumerateCompiles(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().enumerate().forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected DJNZ for enumerate()")
	}
}

func TestReduceWithFunction(t *testing.T) {
	source := `
fun sum(acc: u8, x: u8) -> u8 { return acc + x; }

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().reduce(sum);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Reduce should use DJNZ
	if !hasOpcode(mainFn, ir.OpDJNZ) {
		t.Error("expected DJNZ for reduce(sum)")
	}

	// Should call sum
	hasSumCall := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpCall && strings.Contains(inst.Symbol, "sum") {
			hasSumCall = true
			break
		}
	}
	if !hasSumCall {
		t.Error("expected call to sum in reduce body")
	}
}

// --- Inline Filter Optimization Tests (ADR-0008) ---

func TestInlineFilterLambdaGt(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().filter(|x| => bool { x > 3 }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should generate OpJumpIfFlag (inline comparison), NOT OpCall to a lambda
	if !hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("expected OpJumpIfFlag for inline filter |x| x > 3")
		t.Log("Instructions:")
		for i, inst := range mainFn.Instructions {
			t.Logf("  %d: %s", i, inst.String())
		}
	}

	// Should NOT have a lambda function call for the filter
	hasFilterLambdaCall := false
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpCall && strings.Contains(inst.Symbol, "iter_lambda") {
			hasFilterLambdaCall = true
			break
		}
	}
	if hasFilterLambdaCall {
		t.Error("inline filter should NOT generate a lambda function call")
	}

	// Verify the flag condition is CY (for x > 3 → CP 4, JR C)
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfFlag {
			if ir.FlagCondition(inst.Imm) != ir.FlagCY {
				t.Errorf("expected FlagCY for x > 3, got %s", ir.FlagCondition(inst.Imm))
			}
			break
		}
	}

	// Verify the CP constant is 4 (x > 3 → CP 4)
	var jumpIfFlagInst *ir.Instruction
	for i, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfFlag {
			jumpIfFlagInst = &mainFn.Instructions[i]
			break
		}
	}
	if jumpIfFlagInst != nil {
		constReg := jumpIfFlagInst.Src2
		for _, inst := range mainFn.Instructions {
			if inst.Op == ir.OpLoadConst && inst.Dest == constReg {
				if inst.Imm != 4 {
					t.Errorf("expected CP constant = 4 (x > 3 → CP 4), got %d", inst.Imm)
				}
				break
			}
		}
	}
}

func TestInlineFilterLambdaEq(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().filter(|x| => bool { x == 3 }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("expected OpJumpIfFlag for inline filter |x| x == 3")
		t.Log("Instructions:")
		for i, inst := range mainFn.Instructions {
			t.Logf("  %d: %s", i, inst.String())
		}
	}

	// Verify the flag condition is NZ (for x == 3 → CP 3, JR NZ to skip)
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfFlag {
			if ir.FlagCondition(inst.Imm) != ir.FlagNZ {
				t.Errorf("expected FlagNZ for x == 3, got %s", ir.FlagCondition(inst.Imm))
			}
			break
		}
	}
}

func TestInlineFilterLambdaLt(t *testing.T) {
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().filter(|x| => bool { x < 3 }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("expected OpJumpIfFlag for inline filter |x| x < 3")
	}

	// x < 3 → CP 3, JR NC to skip (false when A >= 3)
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfFlag {
			if ir.FlagCondition(inst.Imm) != ir.FlagNC {
				t.Errorf("expected FlagNC for x < 3, got %s", ir.FlagCondition(inst.Imm))
			}
			break
		}
	}
}

func TestInlineFilterConstOnLeft(t *testing.T) {
	// Test |x| 5 > x which normalizes to x < 5
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().filter(|x| => bool { 5 > x }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	if !hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("expected OpJumpIfFlag for inline filter |x| 5 > x")
	}

	// 5 > x normalizes to x < 5 → CP 5, JR NC
	for _, inst := range mainFn.Instructions {
		if inst.Op == ir.OpJumpIfFlag {
			if ir.FlagCondition(inst.Imm) != ir.FlagNC {
				t.Errorf("expected FlagNC for x < 5, got %s", ir.FlagCondition(inst.Imm))
			}
			break
		}
	}
}

func TestComplexFilterFallback(t *testing.T) {
	// Complex predicate: |x| x > 5 && x < 10 — should NOT inline
	source := `
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.iter().filter(|x| => bool { x > 3 && x < 10 }).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should NOT use OpJumpIfFlag (too complex for inline)
	if hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("complex predicate (&&) should NOT be inlined as OpJumpIfFlag")
	}

	// Should fall back to OpCall + OpJumpIfNot
	if !hasOpcode(mainFn, ir.OpJumpIfNot) {
		t.Error("complex predicate should use OpCall + OpJumpIfNot fallback")
	}
}

func TestNamedFunctionFilterFallback(t *testing.T) {
	// Named function predicate — should NOT inline
	source := `
fun is_big(x: u8) -> bool { return x > 3; }
fun print_u8(x: u8) -> void {}

fun main() -> void {
    let nums: [u8; 5] = [1, 2, 3, 4, 5];
    nums.filter(is_big).forEach(print_u8);
}
`
	module := compileIteratorToMIR(t, source)
	mainFn := findMainFunction(module)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should NOT use OpJumpIfFlag (named function, not a lambda)
	if hasOpcode(mainFn, ir.OpJumpIfFlag) {
		t.Error("named function predicate should NOT be inlined as OpJumpIfFlag")
	}

	// Should use OpCall + OpJumpIfNot
	if !hasOpcode(mainFn, ir.OpJumpIfNot) {
		t.Error("named function predicate should use OpCall + OpJumpIfNot")
	}
}
