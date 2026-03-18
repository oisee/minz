// looprot.go — Loop rotation: header-tested while → bottom-tested do-while.
//
// Transforms:
//
//	head:  branch cond @body, @exit     ← header test
//	body:  ...work...; jump @head       ← back-edge
//
// Into:
//
//	guard: branch cond @body, @exit     ← one-time pre-check
//	body:  ...work...; branch cond @body, @exit  ← bottom test (eliminates head block)
//
// For counted loops where cond is a decrement-and-test, this enables DJNZ:
//
//	guard: branch cnt!=0 @body, @exit
//	body:  ...work...; djnz cnt, @body, @exit
//
// Why this matters:
// - Saves one jump per iteration (no head→body→head, just body→body)
// - On Z80: while(true)+break pattern becomes tight JR NZ or DJNZ
// - FatFS get_chain_length: SDCC generates JR NZ, MinZ does while(true)+break
//
// The pass works on LIR Prog (multi-block representation).
//
// Implementation: thin wrappers over ApplyBlockRules (Layer 4 CFG DSL).
package lir

// LoopRotateResult holds statistics from loop rotation.
type LoopRotateResult struct {
	Rotated int // number of loops rotated
}

// LoopRotate transforms header-tested loops into bottom-tested loops.
// Modifies prog in place. Delegates to ApplyBlockRules with the loop_rotate rule.
func LoopRotate(prog *Prog) *LoopRotateResult {
	rules := []BlockRewriteRule{loopRotateRule()}
	r := ApplyBlockRules(prog, rules, 5)
	return &LoopRotateResult{Rotated: r.Applied}
}

// LoopRotateDJNZ is a specialized rotation for counted loops.
// Detects: head branches on counter, body decrements counter and jumps to head.
// Transforms body's terminator to TermDJNZ.
//
// Pattern:
//
//	head:  branch cnt!=0 @body, @exit
//	body:  ...work...; cnt = sub(cnt, 1); jump @head
//
// After:
//
//	head:  branch cnt!=0 @body, @exit   (guard, executes once)
//	body:  ...work...; djnz cnt, @body, @exit
//
// The sub(cnt, 1) instruction is absorbed into the DJNZ terminator.
// Delegates to ApplyBlockRules with DJNZ (priority 35) + fallback rotate (priority 30).
func LoopRotateDJNZ(prog *Prog) *LoopRotateResult {
	rules := []BlockRewriteRule{loopRotateDJNZRule(), loopRotateRule()}
	r := ApplyBlockRules(prog, rules, 5)
	return &LoopRotateResult{Rotated: r.Applied}
}
