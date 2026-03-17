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
package lir

// LoopRotateResult holds statistics from loop rotation.
type LoopRotateResult struct {
	Rotated int // number of loops rotated
}

// LoopRotate transforms header-tested loops into bottom-tested loops.
// Modifies prog in place.
func LoopRotate(prog *Prog) *LoopRotateResult {
	result := &LoopRotateResult{}

	// Build label → index map
	blockIdx := make(map[string]int, len(prog.Blocks))
	for i, b := range prog.Blocks {
		blockIdx[b.Label] = i
	}

	// Find rotatable loops: head block with branch + body block with jump to head
	for hi := range prog.Blocks {
		head := &prog.Blocks[hi]
		if head.Term.Kind != TermBranch || len(head.Term.Targets) < 2 {
			continue
		}

		bodyLabel := head.Term.Targets[0] // then-target (cond!=0 → body)
		exitLabel := head.Term.Targets[1] // else-target (cond==0 → exit)

		bi, ok := blockIdx[bodyLabel]
		if !ok {
			continue
		}
		body := &prog.Blocks[bi]

		// Body must jump back to head (back-edge)
		if body.Term.Kind != TermJump || len(body.Term.Targets) == 0 {
			continue
		}
		if body.Term.Targets[0] != head.Label {
			continue
		}

		// Only rotate if head has no instructions (pure branch header).
		// Headers with instructions need the instructions duplicated into the
		// body, which we'll support later.
		if len(head.Insts) > 0 {
			continue
		}

		// Rotate: head becomes guard (one-time pre-check),
		// body gets the branch as its terminator instead of jump-to-head.
		body.Term = Term{
			Kind:    TermBranch,
			Cond:    head.Term.Cond,
			Targets: []string{bodyLabel, exitLabel},
		}

		result.Rotated++
	}

	return result
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
func LoopRotateDJNZ(prog *Prog) *LoopRotateResult {
	result := &LoopRotateResult{}

	blockIdx := make(map[string]int, len(prog.Blocks))
	for i, b := range prog.Blocks {
		blockIdx[b.Label] = i
	}

	for hi := range prog.Blocks {
		head := &prog.Blocks[hi]
		if head.Term.Kind != TermBranch || len(head.Term.Targets) < 2 {
			continue
		}

		bodyLabel := head.Term.Targets[0]
		exitLabel := head.Term.Targets[1]
		cntPhys := head.Term.Cond.Phys

		bi, ok := blockIdx[bodyLabel]
		if !ok {
			continue
		}
		body := &prog.Blocks[bi]

		if body.Term.Kind != TermJump || len(body.Term.Targets) == 0 {
			continue
		}
		if body.Term.Targets[0] != head.Label {
			continue
		}

		// Only rotate pure-branch headers
		if len(head.Insts) > 0 {
			continue
		}

		// Find the sub(cnt, 1) instruction in body that decrements the counter.
		// It must write to the same phys as the branch condition.
		subIdx := -1
		for i, inst := range body.Insts {
			if inst.Pat != nil && inst.Pat.MIROp == OpSub &&
				inst.Dst.Phys == cntPhys {
				subIdx = i
			}
		}

		if subIdx >= 0 {
			// Remove the sub instruction — DJNZ absorbs it
			body.Insts = append(body.Insts[:subIdx], body.Insts[subIdx+1:]...)

			// Replace body terminator with DJNZ
			body.Term = Term{
				Kind:    TermDJNZ,
				Counter: Operand{Phys: cntPhys},
				Targets: []string{bodyLabel, exitLabel},
			}
		} else {
			// No decrement found — fall back to regular branch rotation
			body.Term = Term{
				Kind:    TermBranch,
				Cond:    head.Term.Cond,
				Targets: []string{bodyLabel, exitLabel},
			}
		}

		result.Rotated++
	}

	return result
}
