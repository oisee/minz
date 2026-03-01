package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ir"
)

// generateZ80 creates a Z80 code generator, generates code from the module,
// and returns the assembly output as a string.
func generateZ80(t *testing.T, module *ir.Module) string {
	t.Helper()
	var buf bytes.Buffer
	gen := NewZ80Generator(&buf)
	gen.SetTargetPlatform("cpm") // Use CP/M to avoid ZX Spectrum-specific headers
	if err := gen.Generate(module); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	return buf.String()
}

// TestDJNZCodegen verifies that OpDJNZ generates correct Z80 assembly
func TestDJNZCodegen(t *testing.T) {
	fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})
	counter := fn.AllocReg()
	fn.Instructions = append(fn.Instructions,
		ir.Instruction{Op: ir.OpLoadConst, Dest: counter, Imm: 5,
			Type: &ir.BasicType{Kind: ir.TypeU8}, Hint: ir.RegHintB},
	)
	fn.EmitLabel("loop")
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpNop, Comment: "loop body"})
	fn.Instructions = append(fn.Instructions,
		ir.Instruction{Op: ir.OpDJNZ, Src1: counter, Label: "loop", Hint: ir.RegHintB},
	)
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

	module := &ir.Module{Functions: []*ir.Function{fn}}
	asm := generateZ80(t, module)

	// Should contain DEC B (our manual DJNZ replacement)
	if !strings.Contains(asm, "DEC B") {
		t.Error("expected DEC B instruction in output")
	}
	// Should contain JR NZ for the loop back
	if !strings.Contains(asm, "JR NZ,") {
		t.Error("expected JR NZ instruction in output")
	}
	// Should contain the loop label
	if !strings.Contains(asm, "loop") {
		t.Error("expected 'loop' label in output")
	}
}

// TestDJNZCounterInit verifies counter initialization before DJNZ loop
func TestDJNZCounterInit(t *testing.T) {
	fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})
	counter := fn.AllocReg()
	fn.Instructions = append(fn.Instructions,
		ir.Instruction{Op: ir.OpLoadConst, Dest: counter, Imm: 10,
			Type: &ir.BasicType{Kind: ir.TypeU8}},
	)
	fn.EmitLabel("loop")
	fn.Instructions = append(fn.Instructions,
		ir.Instruction{Op: ir.OpDJNZ, Src1: counter, Label: "loop"},
	)
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

	module := &ir.Module{Functions: []*ir.Function{fn}}
	asm := generateZ80(t, module)

	// Should load 10 as the counter
	if !strings.Contains(asm, "LD A, 10") {
		t.Errorf("expected 'LD A, 10' for counter init, got:\n%s", asm)
	}
}

// TestIteratorForEachCodegen tests a simple forEach pattern through codegen
func TestIteratorForEachCodegen(t *testing.T) {
	// Build IR for: array[5].forEach(print_u8) using DJNZ
	fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})

	counter := fn.AllocReg()   // r1: DJNZ counter
	ptr := fn.AllocReg()       // r2: pointer
	element := fn.AllocReg()   // r3: element

	// Load counter = 5
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: counter, Imm: 5,
		Type: &ir.BasicType{Kind: ir.TypeU8}, Hint: ir.RegHintB,
		Comment: "DJNZ counter = 5",
	})
	// Load pointer to array
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: ptr, Imm: 0x8000,
		Type: &ir.BasicType{Kind: ir.TypeU16}, Hint: ir.RegHintHL,
		Comment: "Pointer to array",
	})

	fn.EmitLabel("forEach_loop")

	// Load element through pointer
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoad, Dest: element, Src1: ptr,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Comment: "Load element",
	})

	// Call print_u8(element)
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpCall, Symbol: "print_u8", Args: []ir.Register{element},
		Comment: "forEach callback",
	})

	// Increment pointer
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpInc, Dest: ptr, Src1: ptr, Hint: ir.RegHintHL,
		Comment: "Next element",
	})

	// DJNZ loop
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpDJNZ, Src1: counter, Label: "forEach_loop", Hint: ir.RegHintB,
	})

	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

	module := &ir.Module{Functions: []*ir.Function{fn}}
	asm := generateZ80(t, module)

	// Verify key patterns in the assembly
	if !strings.Contains(asm, "CALL print_u8") {
		t.Errorf("expected CALL print_u8 in output:\n%s", asm)
	}
	if !strings.Contains(asm, "DEC B") {
		t.Error("expected DEC B for DJNZ counter")
	}
	if !strings.Contains(asm, "JR NZ,") {
		t.Error("expected JR NZ for loop back-edge")
	}
}

// TestFilterConditionalJump tests that filter generates conditional jumps
func TestFilterConditionalJump(t *testing.T) {
	fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})

	counter := fn.AllocReg()
	element := fn.AllocReg()
	predicate := fn.AllocReg()

	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: counter, Imm: 5,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	})

	fn.EmitLabel("filter_loop")

	// Simulate predicate check: element > 3
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: element, Imm: 4,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	})
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpGt, Dest: predicate, Src1: element, Src2: counter,
		Type: &ir.BasicType{Kind: ir.TypeBool},
	})

	// Conditional jump (skip if filter fails)
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpJumpIfNot, Src1: predicate, Label: "filter_continue",
		Comment: "Skip if filter predicate is false",
	})

	// Filtered body: call print
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpCall, Symbol: "print_u8", Args: []ir.Register{element},
	})

	fn.EmitLabel("filter_continue")

	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpDJNZ, Src1: counter, Label: "filter_loop",
	})
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

	module := &ir.Module{Functions: []*ir.Function{fn}}
	asm := generateZ80(t, module)

	// Should have a conditional jump for the filter
	hasConditionalJump := strings.Contains(asm, "JR Z,") ||
		strings.Contains(asm, "JP Z,") ||
		strings.Contains(asm, "JR NZ,") ||
		strings.Contains(asm, "JP NZ,")
	if !hasConditionalJump {
		t.Errorf("expected conditional jump for filter, got:\n%s", asm)
	}
}

// TestDJNZStoreBackPersistence verifies the counter store-back fix
func TestDJNZStoreBackPersistence(t *testing.T) {
	// This test verifies that the DEC B + store-back sequence is present.
	// The old bug was: DJNZ decremented B and jumped, but the virtual register
	// was never updated, so the next iteration's loadToB read a stale value.
	fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})

	counter := fn.AllocReg()
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: counter, Imm: 3,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	})
	fn.EmitLabel("persist_loop")
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpNop})
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpDJNZ, Src1: counter, Label: "persist_loop",
	})
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

	module := &ir.Module{Functions: []*ir.Function{fn}}
	asm := generateZ80(t, module)

	// After our fix, the sequence should be: DEC B + LD A, B + store + JR NZ
	// Verify DEC B is present (not bare DJNZ which doesn't store back)
	if strings.Contains(asm, "DJNZ ") {
		t.Error("expected manual DEC B + JR NZ instead of bare DJNZ instruction")
	}
	if !strings.Contains(asm, "DEC B") {
		t.Error("expected DEC B instruction for counter persistence")
	}
	if !strings.Contains(asm, "LD A, B") {
		t.Error("expected LD A, B to save counter value")
	}
}

// --- Inline Filter Codegen Tests (ADR-0008) ---

// TestInlineFilterCodegen verifies OpJumpIfFlag generates CP N + JR cond, label
func TestInlineFilterCodegen(t *testing.T) {
	fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})

	counter := fn.AllocReg()
	element := fn.AllocReg()
	constReg := fn.AllocReg()

	// Load counter = 5
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: counter, Imm: 5,
		Type: &ir.BasicType{Kind: ir.TypeU8}, Hint: ir.RegHintB,
	})

	fn.EmitLabel("loop")

	// Load element (simulate)
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: element, Imm: 42,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	})

	// Load CP constant (x > 67 → CP 68)
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpLoadConst, Dest: constReg, Imm: 68,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	})

	// OpJumpIfFlag: CP 68 + JR C, filter_continue
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op:    ir.OpJumpIfFlag,
		Src1:  element,
		Src2:  constReg,
		Imm:   int64(ir.FlagCY),
		Label: "filter_continue",
		Comment: "Inline filter: CP 68 + JR CY",
	})

	// Filtered body: call print
	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpCall, Symbol: "print_u8", Args: []ir.Register{element},
	})

	fn.EmitLabel("filter_continue")

	fn.Instructions = append(fn.Instructions, ir.Instruction{
		Op: ir.OpDJNZ, Src1: counter, Label: "loop", Hint: ir.RegHintB,
	})
	fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

	module := &ir.Module{Functions: []*ir.Function{fn}}
	asm := generateZ80(t, module)

	// Should contain CP 68
	if !strings.Contains(asm, "CP 68") {
		t.Errorf("expected 'CP 68' in output, got:\n%s", asm)
	}

	// Should contain JR C, (jump on carry)
	if !strings.Contains(asm, "JR C,") {
		t.Errorf("expected 'JR C,' in output, got:\n%s", asm)
	}

	// Should NOT contain OR A (that's the old boolean-testing path)
	// The OR A before CP would destroy the flags.
	// Count OR A occurrences — there should be none between the element load and the CP
	lines := strings.Split(asm, "\n")
	for i, line := range lines {
		if strings.Contains(line, "CP 68") {
			// Check the line immediately before is NOT "OR A"
			if i > 0 && strings.TrimSpace(lines[i-1]) == "OR A" {
				t.Error("inline filter should NOT have OR A before CP")
			}
			break
		}
	}
}

// TestInlineFilterAllConditions tests all flag conditions generate correctly
func TestInlineFilterAllConditions(t *testing.T) {
	tests := []struct {
		name     string
		flag     ir.FlagCondition
		expected string
	}{
		{"FlagCY", ir.FlagCY, "JR C,"},
		{"FlagNC", ir.FlagNC, "JR NC,"},
		{"FlagZ", ir.FlagZ, "JR Z,"},
		{"FlagNZ", ir.FlagNZ, "JR NZ,"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fn := ir.NewFunction("main", &ir.BasicType{Kind: ir.TypeVoid})

			element := fn.AllocReg()
			constReg := fn.AllocReg()

			fn.Instructions = append(fn.Instructions, ir.Instruction{
				Op: ir.OpLoadConst, Dest: element, Imm: 42,
				Type: &ir.BasicType{Kind: ir.TypeU8},
			})
			fn.Instructions = append(fn.Instructions, ir.Instruction{
				Op: ir.OpLoadConst, Dest: constReg, Imm: 10,
				Type: &ir.BasicType{Kind: ir.TypeU8},
			})
			fn.Instructions = append(fn.Instructions, ir.Instruction{
				Op:    ir.OpJumpIfFlag,
				Src1:  element,
				Src2:  constReg,
				Imm:   int64(tc.flag),
				Label: "skip",
			})
			fn.EmitLabel("skip")
			fn.Instructions = append(fn.Instructions, ir.Instruction{Op: ir.OpReturn})

			module := &ir.Module{Functions: []*ir.Function{fn}}
			asm := generateZ80(t, module)

			if !strings.Contains(asm, tc.expected) {
				t.Errorf("expected %q in output, got:\n%s", tc.expected, asm)
			}
			if !strings.Contains(asm, "CP 10") {
				t.Errorf("expected 'CP 10' in output, got:\n%s", asm)
			}
		})
	}
}
