package mir2

import (
	"maps"
	"strings"

	"github.com/minz/minzc/pkg/z80effects"
)

// Exact caller saves at a call site (S2).
//
// A callee whose emitted body is proven (exactWritesReason) can change only
// its proven registers. The call site adds the writes of its own argument
// copies. Those are not predicted: emitParallelCopy may pick scratch
// registers, use A for literals or flags, or the stack for exchanges. They are
// emitted first into a separate buffer and measured with z80effects, then the
// saves are chosen from body ∪ copies and pushed in front of the copies.
//
// Emitting the saves after the copies were generated is equivalent to the
// ordinary order:
//   - the live homes are snapshotted (callSaveLive) before the copies, as the
//     ordinary path reads them before the copies;
//   - PUSH writes only SP and memory below it, so the emission hooks the
//     copies ran through (cache invalidation, carry tracking) see the same
//     register state either way; runtime stack checks and native frame
//     annotations are only active in products that never narrow (Z80Codegen);
//   - spill homes are pushed before both, exactly as before
//     (pushCallSpillHomes), and nothing in the copies addresses the stack
//     relative to SP: fragmentWrites rejects any SP read and any EX (SP) or
//     POP below the copies' own pushes, and such a site keeps the full ABI.
//
// The copies are preceded by a marker comment naming the measured writes;
// Z80Codegen re-measures every marked fragment in the final product.

const exactArgsMarker = "    ; exact args: "

// exactCallClobbers returns the proven write set of a direct callee when this
// site can use it: not a tail call or indirect call, and no extra results
// (multi-return sites keep the full ABI).
func (g *z80cg) exactCallClobbers(inst *Inst, callee *Func, indirect bool) (map[string]bool, bool) {
	if g.legacyLeafClobbers || inst == g.tailCallInst || indirect || callee == nil || callee.Attrs.IsExtern || len(inst.ExtraRets) != 0 {
		return nil, false
	}
	exact, ok := g.clobberCache[callee.Name]
	return exact, ok
}

// exactCallerSaves emits the argument copies into a buffer, measures them and
// returns the saves for body ∪ copies, the copy text and its marker. When
// the copies cannot be measured the site saves for the full ABI.
func (g *z80cg) exactCallerSaves(inst *Inst, callee *Func, exact map[string]bool) (pairs, shadow []string, argText, marker string) {
	live := g.callSaveLive(inst)
	clobbered := maps.Clone(exact)
	if len(inst.Args) > 0 {
		outer := g.sb
		var buf strings.Builder
		g.sb = &buf
		g.emitCallArgsForInst(inst.Args, callee, inst)
		g.sb = outer
		argText = buf.String()
		if w, reason := fragmentWrites(argText); reason != "" {
			for _, name := range allZ80Clobbers {
				clobbered[name] = true
			}
		} else {
			names := writeNames(w)
			maps.Copy(clobbered, names)
			marker = exactArgsMarker + sanitizeIdent(callee.Name) + " writes " + strings.Join(clobberNames(names), ", ")
		}
	}
	pairs = savePairsFor(live, clobbered)
	for _, pair := range pairs {
		for r, home := range live {
			if info, ok := g.regInfo[r]; ok && isZ80WideInt(info.Ty) && home == pair {
				shadow = append(shadow, pair)
				break
			}
		}
	}
	return pairs, shadow, argText, marker
}

// verifyExactArgSites re-measures, in a final product, the copies after each
// exact-args marker up to the CALL they feed. They must still be measurable
// and write nothing the marker does not name.
func verifyExactArgSites(asm string) bool {
	lines := strings.Split(asm, "\n")
	for i, line := range lines {
		rest, ok := strings.CutPrefix(line, exactArgsMarker)
		if !ok {
			continue
		}
		sym, declared, ok := strings.Cut(rest, " writes ")
		if !ok {
			return false
		}
		allowed := z80effects.Registers(0)
		for _, name := range strings.Split(declared, ",") {
			if name = strings.TrimSpace(name); name != "" {
				allowed |= z80effects.Register(name)
			}
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			code := strings.Fields(strings.SplitN(lines[j], ";", 2)[0])
			if len(code) == 2 && code[0] == "CALL" && code[1] == sym {
				end = j
				break
			}
			if len(code) > 0 && (code[0] == "CALL" || code[0] == "RET" || code[0] == "JP" && !strings.HasPrefix(code[len(code)-1], ".")) {
				return false
			}
		}
		if end < 0 {
			return false
		}
		w, reason := fragmentWrites(strings.Join(lines[i+1:end], "\n"))
		if reason != "" || w&^allowed != 0 {
			return false
		}
	}
	return true
}
