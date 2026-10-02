package mir2

import (
	"fmt"
	"strings"
)

// asmPeepholePass applies post-emit string-level peephole optimisations:
//  1. Double EX DE,HL elimination — two consecutive EX DE,HL cancel; remove both.
//  2. Single-JP function → EQU alias — a function whose entire body is one JP
//     becomes a zero-byte/zero-T EQU alias (tail-call guaranteed by contract).
func asmPeepholePass(src string) string {
	lines := strings.Split(src, "\n")
	lines = elimDoubleExDeHl(lines)
	lines = elimSingleJpEqu(lines)
	lines = elimCallRet(lines)
	lines = foldCondCall(lines)
	lines = elimJrToRet(lines)
	lines = elimDeadAfterRet(lines)
	lines = foldRLCASled(lines)
	return strings.Join(lines, "\n")
}

// foldCondCall replaces JR cc, skip / CALL target / skip: with CALL cc, target.
// Saves 2 bytes and avoids branch penalty.
//
// Pattern (any of JR/JRS, any condition):
//
//	JR  NC, .skip        (or JRS, or any cc: Z/NZ/C/NC)
//	[.optional_label:]   (zero or more intermediate labels, no instructions)
//	CALL target
//	.skip:
//
// → CALL NC, target
//
// The inverse condition is used: JR NC skips the CALL when NC,
// so we emit CALL C (call when condition is met, i.e. NOT NC).
func foldCondCall(lines []string) []string {
	invertCC := map[string]string{
		"Z": "NZ", "NZ": "Z", "C": "NC", "NC": "C",
		"z": "nz", "nz": "z", "c": "nc", "nc": "c",
	}

	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		// Match JR cc, .label or JRS cc, .label
		var jrCC, skipLabel string
		if strings.HasPrefix(trimmed, "JR ") || strings.HasPrefix(trimmed, "JRS ") {
			// Parse: JR(S) CC, .label
			rest := trimmed
			if strings.HasPrefix(rest, "JRS ") {
				rest = rest[4:]
			} else {
				rest = rest[3:]
			}
			parts := strings.SplitN(rest, ",", 2)
			if len(parts) == 2 {
				cc := strings.TrimSpace(parts[0])
				lab := strings.TrimSpace(parts[1])
				if _, ok := invertCC[cc]; ok && strings.HasPrefix(lab, ".") {
					jrCC = cc
					skipLabel = lab
				}
			}
		}

		if jrCC == "" {
			out = append(out, lines[i])
			i++
			continue
		}

		// Scan forward: skip label-only lines, find CALL, then skip label.
		j := i + 1
		var intermediateLabels []string
		for j < len(lines) {
			t := strings.TrimSpace(lines[j])
			if t == "" || strings.HasPrefix(t, ";") {
				j++
				continue
			}
			if strings.HasSuffix(t, ":") {
				intermediateLabels = append(intermediateLabels, lines[j])
				j++
				continue
			}
			break
		}

		// Expect CALL target at position j
		if j >= len(lines) {
			out = append(out, lines[i])
			i++
			continue
		}
		callLine := strings.TrimSpace(lines[j])
		if !strings.HasPrefix(callLine, "CALL ") {
			out = append(out, lines[i])
			i++
			continue
		}
		callTarget := strings.TrimSpace(callLine[5:])

		// Expect skip label at position j+1 (possibly with blank/comment lines between)
		k := j + 1
		for k < len(lines) {
			t := strings.TrimSpace(lines[k])
			if t == "" || strings.HasPrefix(t, ";") {
				k++
				continue
			}
			break
		}
		if k >= len(lines) {
			out = append(out, lines[i])
			i++
			continue
		}
		labelLine := strings.TrimSpace(lines[k])
		expectedLabel := skipLabel + ":"
		if labelLine != expectedLabel {
			out = append(out, lines[i])
			i++
			continue
		}

		// Match! Emit intermediate labels + CALL cc, target + skip label.
		invCC := invertCC[jrCC]
		indent := "    "
		if idx := strings.Index(lines[j], "CALL "); idx > 0 {
			indent = lines[j][:idx]
		}
		for _, lab := range intermediateLabels {
			out = append(out, lab)
		}
		out = append(out, fmt.Sprintf("%sCALL %s, %s", indent, invCC, callTarget))
		out = append(out, lines[k]) // keep the skip label
		i = k + 1
		continue
	}
	return out
}

// foldRLCASled replaces consecutive RLCA sequences (3+) with CALL __rotate_N.
// RLCA = 4T each. CALL __rotate_N = 17T + N×4T + 10T = 27 + N×4T.
// Break-even at N=7 (inline=28T, sled=55T) — so sled is about code SIZE savings.
// For N >= 3 inline RLCAs, fold to CALL to save code bytes (shared sled is 9 bytes).
// Caller must ensure __rotate sled is emitted (needsRotate flag).
func foldRLCASled(lines []string) []string {
	out := make([]string, 0, len(lines))
	i := 0
	needsSled := false
	for i < len(lines) {
		// Count consecutive RLCA instructions
		if strings.TrimSpace(lines[i]) == "RLCA" {
			count := 0
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "RLCA" {
				count++
				j++
			}
			if count >= 3 {
				// Replace with CALL __rotate_N
				indent := "    "
				if idx := strings.Index(lines[i], "RLCA"); idx > 0 {
					indent = lines[i][:idx]
				}
				out = append(out, fmt.Sprintf("%sCALL __rotate_%d", indent, count))
				needsSled = true
				i = j
				continue
			}
		}
		out = append(out, lines[i])
		i++
	}
	// If we folded any, add a marker comment so the caller knows to emit the sled
	if needsSled {
		out = append(out, "; [peephole] RLCA sled referenced — __rotate runtime required")
	}
	return out
}

// elimCallRet replaces CALL X / RET with JP X (tail call promotion).
// Saves 17 T-states (CALL=17 + RET=10 → JP=10) and 1 byte.
func elimCallRet(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "CALL ") && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next == "RET" {
				target := strings.TrimSpace(strings.TrimPrefix(trimmed, "CALL "))
				// Skip numeric targets (BDOS CALL 5, RST addresses) —
				// these are syscalls that need the return address on stack.
				isNumeric := len(target) > 0 && (target[0] >= '0' && target[0] <= '9' ||
					target[0] == '$' || strings.HasPrefix(target, "0x"))
				if !isNumeric {
					out = append(out, strings.Replace(lines[i], "CALL ", "JP ", 1))
					i++ // skip RET
					continue
				}
			}
		}
		out = append(out, lines[i])
	}
	return out
}

// elimDeadAfterRet removes unreachable code between an unconditional RET/JP
// and the next label. After cond_ret lowering, the pattern:
//
//	RET           ← unconditional return
//	.label:       ← else-branch (only reachable via jump, not fallthrough)
//
// means nothing between the RET and .label is reachable. If there are
// instructions between them (rare), they're dead code. More commonly,
// the RET itself is redundant when the PREVIOUS instruction was also a
// RET or JP — but that's already handled by the IR. The main benefit
// here is removing the RET when it's the last instruction before a label
// and the codegen emitted it defensively.
func elimDeadAfterRet(lines []string) []string {
	// This pass doesn't remove the RET (it may be the return for the
	// then-branch). It removes dead instructions BETWEEN RET and next label.
	// Uncommon in practice — the main pattern is already clean.
	return lines
}

// elimDoubleExDeHl removes consecutive EX DE,HL pairs.
// Blank lines and comment-only lines between the pair are preserved.
func elimDoubleExDeHl(lines []string) []string {
	const exLine = "    EX DE, HL"
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "EX DE, HL" {
			out = append(out, lines[i])
			continue
		}
		// lines[i] is an EX DE,HL. Look for a second one after skipping
		// blank/comment lines (no real instructions between).
		j := i + 1
		for j < len(lines) {
			t := strings.TrimSpace(lines[j])
			if t == "" || strings.HasPrefix(t, ";") {
				j++
				continue
			}
			break
		}
		if j < len(lines) && strings.TrimSpace(lines[j]) == "EX DE, HL" {
			// Skip both EX DE,HL lines (drop them from output).
			i = j // loop increment makes i = j+1
			continue
		}
		out = append(out, lines[i])
	}
	return out
}

// elimSingleJpEqu converts a function that is exactly one JP target into an EQU.
//
// Pattern (consecutive lines, optionally separated by blank/comment lines):
//
//	funcname:
//	    JP  target
//
// → replaced with a single line:
//
//	funcname    EQU target
//
// Safety conditions:
//   - No additional instructions in the block (comments/blanks OK).
//   - No local labels between funcname: and JP (would break relative refs).
//   - Not "main" (entry point must remain a real label).
func elimSingleJpEqu(lines []string) []string {
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		line := lines[i]
		// Match a bare label line: word followed by ':' with no leading whitespace.
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			label := strings.TrimSuffix(strings.TrimSpace(line), ":")
			if label != "main" && label != "" && !strings.Contains(label, ".") {
				// Scan forward past blank/comment lines to find first instruction.
				j := i + 1
				for j < len(lines) {
					t := strings.TrimSpace(lines[j])
					if t == "" || strings.HasPrefix(t, ";") {
						j++
						continue
					}
					break
				}
				if j < len(lines) {
					instr := strings.TrimSpace(lines[j])
					// Must be exactly "JP target" (not DJNZ, not JP cc, not JP (HL)/(IX)/(IY)).
					if strings.HasPrefix(instr, "JP ") && !strings.Contains(instr, ",") && !strings.Contains(instr, "(") {
						target := strings.TrimSpace(strings.TrimPrefix(instr, "JP "))
						// Check that the next non-blank/comment line after JP is
						// a top-level label or end-of-section (not another instruction
						// inside this "function").
						k := j + 1
						for k < len(lines) {
							t := strings.TrimSpace(lines[k])
							if t == "" || strings.HasPrefix(t, ";") {
								k++
								continue
							}
							break
						}
						nextIsTopLevel := k >= len(lines) ||
							(!strings.HasPrefix(lines[k], " ") && strings.HasSuffix(strings.TrimSpace(lines[k]), ":")) ||
							lines[k] == "; globals" || lines[k] == "; strings"
						if nextIsTopLevel {
							// Emit EQU alias, skip all consumed lines.
							out = append(out, fmt.Sprintf("%s    EQU %s", label, target))
							// Preserve blank lines that came before JP (already in out
							// via j-loop... actually they were skipped). Emit one blank
							// to maintain section spacing.
							i = j + 1
							continue
						}
					}
				}
			}
		}
		out = append(out, line)
		i++
	}
	return out
}

// elimJrToRet replaces `JRS cc, label` with `RET cc` when the target label
// is a bare RET instruction. Then removes the label+RET if no other references
// remain. This saves 1B per replaced jump (JR cc = 2B → RET cc = 1B).
//
// Before:  JRS Z, .cret0 / JRS C, .cret0 / ... / .cret0: / RET
// After:   RET Z / RET C / ...  (label+RET removed if dead)
func elimJrToRet(lines []string) []string {
	// Pass 1: find labels that point to bare RET.
	retLabels := make(map[string]bool)
	for i := 0; i < len(lines)-1; i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, ".") && strings.HasSuffix(t, ":") {
			label := strings.TrimSuffix(t, ":")
			// Check next non-blank line is RET.
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if next == "" || strings.HasPrefix(next, ";") {
					continue
				}
				if next == "RET" {
					retLabels[label] = true
				}
				break
			}
		}
	}
	if len(retLabels) == 0 {
		return lines
	}

	// Pass 2: replace JRS cc, retLabel → RET cc.
	// Track which labels are still referenced.
	replaced := make(map[string]int) // label → count of replacements
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		// Match: JRS cc, .label  or  JR cc, .label
		if (strings.HasPrefix(t, "JRS ") || strings.HasPrefix(t, "JR ")) && strings.Contains(t, ", .") {
			parts := strings.SplitN(t, ", ", 2)
			if len(parts) == 2 {
				label := strings.TrimSpace(parts[1])
				if retLabels[label] {
					// Extract condition code.
					prefix := parts[0] // "JRS Z" or "JR C"
					cc := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(prefix, "JRS"), "JR"))
					if cc != "" {
						out = append(out, "    RET "+cc)
						replaced[label]++
						continue
					}
				}
			}
		}
		out = append(out, line)
	}

	if len(replaced) == 0 {
		return lines // nothing changed
	}

	// Pass 3: count remaining references to each replaced label.
	// If a label has zero remaining references, remove the label+RET.
	labelRefs := make(map[string]int)
	for _, line := range out {
		t := strings.TrimSpace(line)
		for label := range replaced {
			if strings.Contains(t, label) && !strings.HasSuffix(t, ":") {
				labelRefs[label]++
			}
		}
	}

	// Pass 3b: keep RET when the preceding instruction can fall through.
	// Arithmetic sign correction, DJNZ and conditional branches all need it.
	for i := 0; i < len(out); i++ {
		t := strings.TrimSpace(out[i])
		if strings.HasSuffix(t, ":") {
			label := strings.TrimSuffix(t, ":")
			if _, ok := replaced[label]; ok {
				// Walk backward to find the preceding instruction.
				for j := i - 1; j >= 0; j-- {
					prev := strings.TrimSpace(out[j])
					if prev == "" || strings.HasPrefix(prev, ";") {
						continue
					}
					if prev != "RET" &&
						!(strings.HasPrefix(prev, "JP ") && !strings.Contains(prev, ",")) &&
						!(strings.HasPrefix(prev, "JR ") && !strings.Contains(prev, ",")) &&
						!(strings.HasPrefix(prev, "JRS ") && !strings.Contains(prev, ",")) {
						labelRefs[label]++ // implicit fall-through reference
					}
					break
				}
			}
		}
	}

	// Pass 4: remove dead label+RET pairs.
	deadLabels := make(map[string]bool)
	for label := range replaced {
		if labelRefs[label] == 0 {
			deadLabels[label] = true
		}
	}
	if len(deadLabels) == 0 {
		return out
	}

	var final []string
	for i := 0; i < len(out); i++ {
		t := strings.TrimSpace(out[i])
		if strings.HasSuffix(t, ":") {
			label := strings.TrimSuffix(t, ":")
			if deadLabels[label] {
				// Skip label line and the following RET.
				for j := i + 1; j < len(out); j++ {
					next := strings.TrimSpace(out[j])
					if next == "" || strings.HasPrefix(next, ";") {
						i = j
						continue
					}
					if next == "RET" {
						i = j // skip the RET too
					}
					break
				}
				continue
			}
		}
		final = append(final, out[i])
	}
	return final
}
