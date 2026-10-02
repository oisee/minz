package mir2

import (
	"slices"
	"strings"

	"github.com/minz/minzc/pkg/z80spec"
)

// ── Calls ─────────────────────────────────────────────────────────────────────

func (g *z80cg) genCall(inst *Inst) {
	clear(g.holdsPhys) // calls clobber all volatile registers

	// ── Built-in intrinsics (inlined, no CALL emitted) ────────────────────────
	switch inst.Sym {
	case "@mir.io.print.str":
		// Emit inline NUL-terminated string output loop via OUT ($23), A.
		// The string pointer must be in HL (ClassPointer).
		// HL is advanced in-place; A and F are clobbered.
		idx := g.trampIdx
		g.trampIdx++
		g.emitf(".print_str_%d:", idx)
		g.emit("    LD A, (HL)")
		g.emit("    AND A")
		g.emitf("    JR Z, .print_str_done_%d", idx)
		g.emit("    OUT (0x23), A")
		g.emit("    INC HL")
		g.emitf("    JR .print_str_%d", idx)
		g.emitf(".print_str_done_%d:", idx)
		// A, F, HL clobbered — invalidate holdsPhys entries for these.
		g.invalidate("A")
		g.invalidate("HL")
		return

	case "@mir.io.print.u8":
		// Emit single OUT ($23), A.  The value must already be in A.
		g.emit("    OUT (0x23), A")
		g.invalidate("A")
		return

	case "@mir.io.print.char":
		// Print ASCII character in A via OUT ($23).
		g.emit("    OUT (0x23), A")
		g.invalidate("A")
		return

	case "@mir.io.print.nl":
		// Emit newline (\n = 0x0A) via OUT ($23), A.
		g.emit("    LD A, 0x0A")
		g.emit("    OUT (0x23), A")
		g.invalidate("A")
		return

	case "@mir.io.print.dec":
		// Print u8 value in A as decimal ASCII digits via OUT ($23).
		// Inline: divide by 100, 10 (using repeated subtraction — small and fast
		// enough for 0-255).  Clobbers A, B, C, D, H, L.
		idx := g.trampIdx
		g.trampIdx++
		// Hundreds digit
		g.emitf("    LD C, A") // C = n
		g.emitf("    LD B, 0") // B = hundreds count
		g.emitf(".pdec_h%d:", idx)
		g.emitf("    LD A, C")
		g.emitf("    CP 100")
		g.emitf("    JR C, .pdec_ht%d", idx)
		g.emitf("    SUB 100")
		g.emitf("    LD C, A")
		g.emitf("    INC B")
		g.emitf("    JR .pdec_h%d", idx)
		g.emitf(".pdec_ht%d:", idx)
		// Only print hundreds if non-zero
		g.emitf("    LD A, B")
		g.emitf("    OR A")
		g.emitf("    JR Z, .pdec_t%d", idx)
		g.emitf("    ADD A, 48") // '0'
		g.emitf("    OUT (0x23), A")
		// Tens digit
		g.emitf(".pdec_t%d:", idx)
		g.emitf("    LD D, 0") // D = tens count
		g.emitf(".pdec_tl%d:", idx)
		g.emitf("    LD A, C")
		g.emitf("    CP 10")
		g.emitf("    JR C, .pdec_tt%d", idx)
		g.emitf("    SUB 10")
		g.emitf("    LD C, A")
		g.emitf("    INC D")
		g.emitf("    JR .pdec_tl%d", idx)
		g.emitf(".pdec_tt%d:", idx)
		// Only print tens if hundreds or tens non-zero
		g.emitf("    LD A, B")
		g.emitf("    OR D")
		g.emitf("    JR Z, .pdec_u%d", idx)
		g.emitf("    LD A, D")
		g.emitf("    ADD A, 48")
		g.emitf("    OUT (0x23), A")
		// Units digit (always print)
		g.emitf(".pdec_u%d:", idx)
		g.emitf("    LD A, C")
		g.emitf("    ADD A, 48")
		g.emitf("    OUT (0x23), A")
		g.invalidate("A")
		g.invalidate("B")
		g.invalidate("C")
		g.invalidate("D")
		return

	case "@mir.io.console.log":
		// console_log(n: u8) — OUT ($23), A  (mze/mzx stdout port)
		g.emit("    OUT (0x23), A")
		g.invalidate("A")
		return

	case "@mir.io.console.err":
		// console_err(n: u8) — OUT ($25), A  (mze/mzx stderr port)
		g.emit("    OUT (0x25), A")
		g.invalidate("A")
		return

	case "@error":
		// @error(N) — set carry flag, error code in A, return.
		// Used in fallible functions (name?). A = argument (error code).
		g.emit("    SCF") // CY = 1 (error)
		g.emit("    RET") // return to caller with CY set + A = code
		g.invalidate("A")
		return

	case "@check":
		// @check — jump to error handler if carry set.
		// Emits: JR NC, .skip / <handler will be next block> / .skip:
		// For now: just emit RET C (propagate). Handler block TBD.
		idx := g.trampIdx
		g.trampIdx++
		g.emitf("    JR NC, .check_ok_%d", idx)
		// The handler code follows in the next statement.
		// For simple propagation, emit RET:
		g.emit("    RET") // propagate error (CY + A intact)
		g.emitf(".check_ok_%d:", idx)
		return

	case "@propagate":
		// @propagate — conditional return on carry. 1 byte, 5T.
		// If CY=1 (error from previous call), return immediately.
		g.emit("    RET C")
		return
	}

	sym := sanitizeIdent(inst.Sym)

	var callee *Func
	indirect := inst.Op == OpCallIndirect
	if indirect {
		callee = &Func{}
		for i, arg := range inst.Args {
			ty := inferTyFromAlloc(g.ar, arg)
			callee.Contract.Params = append(callee.Contract.Params, Param{Ty: ty, Class: standardParamClass(ty, i)})
		}
		if inst.Dst != NoReg {
			callee.Contract.Returns = []Return{{Ty: inst.Ty, Class: ClassAcc}}
		}
	} else if g.mod != nil {
		callee = g.mod.FuncByName(inst.Sym)
	}

	// Save live caller values before argument copies, including reused args.
	var callerSavePairs []string
	if inst != g.tailCallInst {
		callerSavePairs = g.callerSavePairs(inst, callee)
		for _, pair := range callerSavePairs {
			g.emitf("    PUSH %s", pair)
		}
	}
	if indirect {
		// Snapshot the pointer before argument setup. IX keeps the target
		// separate from the standard ABI's HL word argument.
		g.pushWord(g.loc(inst.Src[0]))
	}
	if len(inst.Args) > 0 && callee != nil {
		g.emitCallArgs(inst.Args, callee.Contract.Params)
	}
	if indirect {
		g.emit("    POP IX")
	}

	// Tail call optimisation: if this is the tail call instruction detected by
	// genBlock, emit JP instead of CALL so the callee's RET returns to our caller.
	if inst == g.tailCallInst {
		if callee != nil && callee.Attrs.ExternAddr != 0 {
			g.emitf("    JP 0x%04X", callee.Attrs.ExternAddr)
		} else {
			g.emitf("    JP %s", sym)
		}
		return // genTerm will skip RET
	}

	// Check if callee has a fixed address (ExternAddr)
	if indirect {
		g.emit("    CALL __call_ix")
		g.needsCallIX = true
	} else if callee != nil && callee.Attrs.ExternAddr != 0 {
		addr := callee.Attrs.ExternAddr
		// RST addresses on Z80: 0x00, 0x08, 0x10, 0x18, 0x20, 0x28, 0x30, 0x38
		if addr <= 0x38 && addr%8 == 0 {
			g.emitf("    RST 0x%02X", addr)
		} else {
			g.emitf("    CALL 0x%04X", addr)
		}
	} else {
		g.emitf("    CALL %s", sym)
	}

	// Pick up every result without letting caller restores destroy it. If a
	// destination shares a saved pair, merge its bytes into the saved stack
	// image so POP restores both the live half and the returned half.
	var results []parallelCopy
	dstPhys := ""
	flagResult := false
	if inst.Dst != NoReg && callee != nil && len(callee.Contract.Returns) > 0 {
		ret := callee.Contract.Returns[0]
		if ret.Class == ClassFlag {
			flagResult = true
		} else {
			dstPhys = g.loc(inst.Dst)
			results = append(results, parallelCopy{srcName: canonicalReturnLoc(ret.Class, ret.Ty), dstName: dstPhys, ty: ret.Ty})
		}
	}
	for i, r := range inst.ExtraRets {
		if r == NoReg {
			continue
		}
		cls, ty := ClassIndex, Ty(TyU16)
		if callee != nil && i+1 < len(callee.Contract.Returns) {
			cls, ty = callee.Contract.Returns[i+1].Class, callee.Contract.Returns[i+1].Ty
		}
		if i < len(inst.ExtraRetClasses) {
			cls = inst.ExtraRetClasses[i]
		}
		if i < len(inst.ExtraRetTys) {
			ty = inst.ExtraRetTys[i]
		}
		results = append(results, parallelCopy{srcName: canonicalReturnLoc(cls, ty), dstName: g.loc(r), ty: ty})
	}
	var protected []string
	for r := range g.regsLiveAfterInst(inst) {
		if r != inst.Dst && !slices.Contains(inst.ExtraRets, r) {
			protected = append(protected, g.loc(r))
		}
	}
	g.pickupCallResults(results, callerSavePairs, flagResult, protected...)

	// CALL and argument setup invalidate cached physical contents, including
	// registers used only as scratch by the emitted callee/runtime sequences.
	g.invalidate("A")
	g.invalidate("F")
	for _, name := range []string{"B", "C", "D", "E", "H", "L", "IX", "IY"} {
		g.invalidate(name)
	}
	g.pendingAccReg = NoReg
	if dstPhys == "A" {
		g.pendingAccReg = inst.Dst
	}

	// Flag-return ABI: if the callee returns ClassFlag, the result is in the CPU
	// flag register (not A).  Record which condition means "true" so that a
	// subsequent TermBrIf on inst.Dst can emit JP <cond> directly without
	// AND A / CP 0.
	if inst.Dst != NoReg && callee != nil {
		if len(callee.Contract.Returns) > 0 {
			ret := callee.Contract.Returns[0]
			if ret.Class == ClassFlag {
				g.callFlags[inst.Dst] = ret.FlagCond
			}
		}
	}

}

// pickupCallResults never stores into code. Most calls need only parallel
// moves; overlapping destinations use a short-lived, reentrant stack frame.
func (g *z80cg) pickupCallResults(results []parallelCopy, saved []string, flagResult bool, protected ...string) {
	savedPair := map[string]bool{}
	for _, p := range saved {
		savedPair[p] = true
	}
	before, after := true, true
	for _, r := range results {
		before = before && !savedPair[regToPairMap[r.dstName]]
		after = after && !savedPair[regToPairMap[r.srcName]]
	}
	if flagResult && savedPair["AF"] {
		before, after = false, false
	}
	if before || after {
		if before {
			g.emitParallelCopy(results)
		}
		for i := len(saved) - 1; i >= 0; i-- {
			g.emitf("    POP %s", saved[i])
		}
		if !before {
			g.emitParallelCopy(results)
		}
		return
	}
	// A byte result can use an unsaved pair only when its scratch byte does
	// not hold a live value preserved by a narrow extern contract.
	if !flagResult && len(results) == 1 && results[0].ty.Width() <= 8 && regToPairMap[results[0].dstName] != "" {
		r := results[0]
		for _, scratch := range []string{"E", "H", "L", "D", "B", "C"} {
			if !savedPair[regToPairMap[scratch]] && !slices.ContainsFunc(protected, func(name string) bool {
				return z80spec.Overlaps(name, scratch)
			}) {
				g.emitLD8(scratch, r.srcName)
				for i := len(saved) - 1; i >= 0; i-- {
					g.emitf("    POP %s", saved[i])
				}
				g.emitLD8(r.dstName, scratch)
				return
			}
		}
	}

	// IX addresses snapshots and the caller-save image. Its original value
	// is restored separately, including when IX itself is a result destination.
	g.emit("    PUSH IX")
	pairs := []string{"AF"}
	for _, r := range results {
		p := regToPairMap[r.srcName]
		if !slices.Contains(pairs, p) {
			pairs = append(pairs, p)
		}
	}
	for _, p := range pairs {
		g.emitf("    PUSH %s", p)
	}
	g.emit("    LD IX, 0")
	g.emit("    ADD IX, SP")
	snapshot := map[string]int{}
	for i, p := range pairs {
		snapshot[p] = 2 * (len(pairs) - 1 - i)
	}
	base := 2 * len(pairs)
	restore := map[string]int{}
	for i, p := range saved {
		restore[p] = base + 2 + 2*(len(saved)-1-i)
	}
	byteOffset := func(name string) int {
		p := regToPairMap[name]
		o := snapshot[p]
		if name == "A" || name == highByte(p) {
			o++
		}
		return o
	}
	storeByte := func(src, dst int) {
		g.emitf("    LD A, (IX+%d)", src)
		g.emitf("    LD (IX+%d), A", dst)
	}
	// Patch saved destinations before loading any final register, since A
	// is scratch while merging the stack image.
	for _, r := range results {
		p := regToPairMap[r.dstName]
		o, ok := restore[p]
		if p == "IX" && !ok {
			o, ok = base, true
		}
		if !ok {
			continue
		}
		if r.ty.Width() > 8 {
			src := snapshot[regToPairMap[r.srcName]]
			storeByte(src, o)
			storeByte(src+1, o+1)
		} else {
			if r.dstName == "A" || r.dstName == highByte(p) {
				o++
			}
			storeByte(byteOffset(r.srcName), o)
		}
	}
	if flagResult && savedPair["AF"] {
		storeByte(snapshot["AF"], restore["AF"])
	}
	// Preserve the result in A in the AF snapshot until all scratch work is done.
	for _, r := range results {
		if r.dstName == "A" && !savedPair["AF"] {
			storeByte(byteOffset(r.srcName), snapshot["AF"]+1)
		}
	}
	for _, r := range results {
		p := regToPairMap[r.dstName]
		if savedPair[p] || p == "IX" || r.dstName == "A" {
			continue
		}
		if r.ty.Width() <= 8 {
			// Indexed memory cannot load IY halves directly; use A as scratch.
			if p != "" && p != "IY" {
				g.emitf("    LD %s, (IX+%d)", r.dstName, byteOffset(r.srcName))
			} else {
				g.emitf("    LD A, (IX+%d)", byteOffset(r.srcName))
				g.emitLD8(r.dstName, "A")
			}
		} else {
			src := snapshot[regToPairMap[r.srcName]]
			if p == "IY" || p == "" {
				g.emit("    PUSH HL")
				g.emitf("    LD L, (IX+%d)", src)
				g.emitf("    LD H, (IX+%d)", src+1)
				g.emitMov(r.dstName, "HL", r.ty.Width())
				g.emit("    POP HL")
			} else {
				g.emitf("    LD %s, (IX+%d)", lowByte(p), src)
				g.emitf("    LD %s, (IX+%d)", highByte(p), src+1)
			}
		}
	}
	// Restore returned flags (ADD IX,SP changed them) and the final A without
	// disturbing HL. Subsequent INC SP and POP instructions preserve flags.
	g.emit("    PUSH HL")
	g.emitf("    LD L, (IX+%d)", snapshot["AF"])
	g.emitf("    LD H, (IX+%d)", snapshot["AF"]+1)
	g.emit("    PUSH HL")
	g.emit("    POP AF")
	g.emit("    POP HL")
	for i := 0; i < base; i++ {
		g.emit("    INC SP")
	}
	g.emit("    POP IX")
	for i := len(saved) - 1; i >= 0; i-- {
		g.emitf("    POP %s", saved[i])
	}
}

// emitCallArgs emits the parallel-copy argument setup for a CALL instruction.
// It maps the current physical locations of the call arguments to the physical
// locations expected by the callee's parameter contract and uses emitParallelCopy
// to resolve any cycles (e.g. A→B, C→A).
//
// Argument locations are transient. Caller values keep their original locations
// after caller-save restoration, so argument setup does not change physOverride.
func (g *z80cg) emitCallArgs(args []Reg, params []Param) {
	var copies []parallelCopy
	for i, arg := range args {
		if i >= len(params) {
			break
		}
		srcPhys := g.loc(arg)
		// Use the callee's actual allocated register for the param if available
		// (from PBQP). Fall back to canonical class-based location otherwise.
		dstPhys := ""
		if loc, ok := g.ar.Locs[params[i].Reg]; ok && loc.Name != "" {
			dstPhys = physName(loc)
		}
		if dstPhys == "" {
			dstPhys = canonicalReturnLoc(params[i].Class, params[i].Ty)
		}
		// Always include in copies — even no-ops (src==dst). This ensures the
		// parallel copy scratch picker knows ALL live arg registers and won't
		// clobber them when resolving cycles (e.g. B↔C using A as scratch
		// when A holds arg0).
		copies = append(copies, parallelCopy{srcName: srcPhys, dstName: dstPhys, ty: params[i].Ty})
	}
	if len(copies) == 0 {
		return
	}
	g.emitParallelCopy(copies)

}

// ReturnLocation exposes the physical return ABI to execution judges.
func ReturnLocation(cls RegClass, ty Ty) string {
	return canonicalReturnLoc(cls, ty)
}

// canonicalReturnLoc returns the physical register name where a return value of
// the given class and type must reside on function exit (calling convention).
func canonicalReturnLoc(cls RegClass, ty Ty) string {
	w := ty.Width()
	switch cls {
	case ClassFlag:
		// Flag-return ABI: result lives in the CPU flag register (F), not in A.
		// Callers test with JP <FlagCond> directly after the CALL instruction.
		// "F" is the sentinel name for LocFlag in Z80PhysLocs.
		return "F"
	case ClassAcc:
		if w <= 8 {
			return "A"
		}
		return "HL" // u16 [acc] → HL
	case ClassPointer, ClassPair:
		return "HL"
	case ClassIndex:
		return "DE"
	case ClassCounter:
		return "B"
	case ClassRegC:
		return "C"
	case ClassRegD:
		return "D"
	case ClassRegE:
		return "E"
	case ClassRegH:
		return "H"
	case ClassRegL:
		return "L"
	case ClassIX:
		return "IX"
	case ClassIY:
		return "IY"
	case ClassGeneral:
		if w <= 8 {
			return "C"
		}
		return "HL"
	default:
		if w <= 8 {
			return "A"
		}
		return "HL"
	}
}

// standardParamClass returns the register class for position i in the
// standard callable ABI (used by function pointers / indirect calls).
// Matches classForParam in hir/lower.go.
func standardParamClass(ty Ty, pos int) RegClass {
	if ty == TyPtr {
		return ClassPointer
	}
	if ty.Width() > 8 {
		if pos == 0 {
			return ClassPointer // HL
		}
		return ClassIndex // DE
	}
	switch pos {
	case 0:
		return ClassAcc // A
	case 1:
		return ClassGeneral // C
	case 2:
		return ClassCounter // B
	default:
		return ClassGeneral
	}
}

// isIntrinsic reports whether sym is a built-in intrinsic handled inline by
// genCall (i.e. the call does NOT lower to a real CALL/JP instruction).
// Intrinsics must NOT be treated as tail calls because genCall returns early
// without consuming g.tailCallInst, which would cause genTerm to skip RET.
func isIntrinsic(sym string) bool {
	return strings.HasPrefix(sym, "@mir.")
}

// multiReturnLocs returns the physical register name for each return position
// by scanning the function's TermRet instructions and looking up the allocator
// result.  This is more accurate than classToRegName(Contract.Returns[i].Class)
// because the contract optimizer may reassign classes after lowering.
// Returns a slice with one entry per return position; entries may be "" if unknown.
func multiReturnLocs(f *Func, ar *AllocResult) []string {
	n := len(f.Contract.Returns)
	if n == 0 {
		return nil
	}
	for _, b := range f.Blocks {
		if ret, ok := b.Term.(*TermRet); ok && len(ret.Vals) == n {
			locs := make([]string, n)
			for i, v := range ret.Vals {
				locs[i] = ar.Loc(v).Name
			}
			return locs
		}
	}
	return nil
}

// classToRegName maps a RegClass to the canonical Z80 register name for that class.
// ty is used to disambiguate (e.g. ClassAcc is always A, ClassPointer is HL for 16-bit).
func classToRegName(cls RegClass, ty Ty) string {
	switch cls {
	case ClassAcc:
		return "A"
	case ClassPointer:
		return "HL"
	case ClassIndex:
		return "DE"
	case ClassCounter:
		return "B"
	case ClassGeneral:
		return "C"
	}
	return ""
}

// isRecursive reports whether f contains a direct recursive call to itself.
func isRecursive(f *Func) bool {
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpCall && inst.Sym == f.Name {
				return true
			}
		}
	}
	return false
}

// callerSavePairs returns the Z80 register pairs that must be PUSH'd/POP'd
// around a CALL to protect live values from the callee's clobbers.
//
// Strategy: compute the callee's clobber set (physical reg names), then check
// which pushable pairs contain registers allocated to virtual regs in the
// caller that remain live after the call, including arguments reused later.
// The call results are excluded because their definitions begin after CALL.
func (g *z80cg) callerSavePairs(inst *Inst, callee *Func) []string {
	// Unknown and compiled callees remain conservative. Only an explicit
	// extern implementation contract can narrow the callee's write set.
	clobberedRegs := map[string]bool{}
	for _, name := range computeClobbers(callee, g.ar) {
		clobberedRegs[name] = true
	}

	// The declaration describes the callee, not the argument shuffle. Include
	// its destination writes as well. Complex copies can use implicit scratch
	// registers; retain the conservative contract for those call sites.
	if callee != nil && callee.Attrs.IsExtern && callee.Contract.ExternClobbers != nil {
		moves := 0
		for i, arg := range inst.Args {
			if i >= len(callee.Contract.Params) {
				break
			}
			p := callee.Contract.Params[i]
			dst := canonicalReturnLoc(p.Class, p.Ty)
			if loc, ok := g.ar.Locs[p.Reg]; ok && loc.Name != "" {
				dst = physName(loc)
			}
			src := g.loc(arg)
			if src == dst {
				continue
			}
			moves++
			clobberedRegs[dst] = true
			if isSpill(src) || src == "F" || dst == "F" {
				moves += 2 // implicit ALU/spill scratch
			}
		}
		if moves > 1 || len(inst.ExtraRets) > 0 {
			for _, name := range allZ80Clobbers {
				clobberedRegs[name] = true
			}
		}
	}

	// Compute which virtual regs are live AFTER this call instruction.
	// A reg is live-across-call if it's used by any instruction after the call
	// in the same block, or by the block's terminator, or by a successor block
	// using backward CFG liveness and instruction uses/definitions.
	liveAfter := g.regsLiveAfterInst(inst)

	// Collect physical regs that are (a) clobbered by callee and (b) hold a
	// value that is live after the call. Exclude result definitions only.
	excluded := make(map[Reg]bool)
	if inst.Dst != NoReg {
		excluded[inst.Dst] = true
	}
	for _, er := range inst.ExtraRets {
		excluded[er] = true
	}

	livePhys := make(map[string]bool)
	for r := range liveAfter {
		if excluded[r] {
			continue
		}
		// Use g.loc() which respects physOverride — a vreg may have been
		// relocated to a different register by materializePendingAcc or
		// save-before-overwrite. The static ar.Locs doesn't reflect these
		// runtime relocations.
		locName := g.loc(r)
		if locName == "" || locName == "?" {
			continue
		}
		if isSpill(locName) {
			continue // spilled to memory — not a register
		}
		for name := range clobberedRegs {
			if z80spec.Overlaps(name, locName) {
				livePhys[locName] = true
				break
			}
		}
	}

	// Map individual registers to PUSH-able pairs.
	// Z80 can only PUSH/POP pairs: BC, DE, HL (and AF, IX, IY).
	pairNeeded := make(map[string]bool)
	for reg := range livePhys {
		if pair, ok := regToPairMap[reg]; ok {
			pairNeeded[pair] = true
		}
	}

	// Return in stable order.
	var result []string
	for _, pair := range []string{"AF", "BC", "DE", "HL", "IX", "IY"} {
		if pairNeeded[pair] {
			result = append(result, pair)
		}
	}
	return result
}

// regToPairMap maps individual Z80 registers to their PUSH-able pair.
var regToPairMap = map[string]string{
	"A": "AF", "F": "AF", "AF": "AF",
	"BC": "BC", "DE": "DE", "HL": "HL", "IX": "IX", "IY": "IY",
	"IXH": "IX", "IXL": "IX", "IYH": "IY", "IYL": "IY",
	"B": "BC", "C": "BC",
	"D": "DE", "E": "DE",
	"H": "HL", "L": "HL",
}

// saveABeforeOverwrite checks if A currently holds a vreg that is still live
// after the current instruction. If so, saves it to a scratch register.
// Used before emitLDA in ALU paths to prevent clobbering live values.
func (g *z80cg) saveABeforeOverwrite(inst *Inst) {
	if g.curBlock == nil {
		return
	}
	// Ties between entry parameters prefer the lowest virtual register ID.
	// Find the vreg MOST RECENTLY defined in A, but only considering
	// definitions BEFORE the current instruction. Only that vreg's
	// value is actually in A right now.
	instIdx := -1
	for idx, bi := range g.curBlock.Insts {
		if bi == inst {
			instIdx = idx
			break
		}
	}

	var bestReg Reg
	bestIdx := -2
	for r, loc := range g.ar.Locs {
		if loc.Kind != LocReg || loc.Name != "A" {
			continue
		}
		if _, ok := g.physOverride[r]; ok {
			continue // already saved elsewhere
		}
		// Find definition index in the current block, only BEFORE inst
		defIdx := -2
		for idx, bi := range g.curBlock.Insts {
			if idx >= instIdx {
				break // stop before current instruction
			}
			if bi.Dst == r {
				defIdx = idx
			}
		}
		// Params are defined before block — use -1 as sentinel
		for _, cp := range g.fn.Contract.Params {
			if cp.Reg == r && defIdx == -2 {
				defIdx = -1
			}
		}
		if defIdx > bestIdx || (defIdx == bestIdx && defIdx > -2 && r < bestReg) {
			bestIdx = defIdx
			bestReg = r
		}
	}
	if bestReg == NoReg {
		return
	}
	// Skip if this vreg is not live after the current instruction
	if !g.isVregLiveAfter(bestReg, inst) {
		return
	}
	// Skip if this vreg is a source of the current instruction
	for _, s := range inst.Src[:] {
		if s == bestReg {
			return
		}
	}
	scratch := g.pickScratch8(inst)
	g.emitf("    LD %s, A    ; save r%d before A overwrite", scratch, bestReg)
	g.physOverride[bestReg] = scratch
}

// isVregLiveAfter checks if vreg is used by any instruction after target in the
// current block, or by the block's terminator.
// saveAccOperandIfLive copies the operand vreg held in A to a scratch
// register when an 8-bit ALU result is about to overwrite A and the operand is
// still used after inst. Later reads of the vreg are redirected via physOverride.
func (g *z80cg) saveAccOperandIfLive(src Reg, inst *Inst) {
	if src == NoReg || src == inst.Dst || !g.isVregLiveAfter(src, inst) {
		return
	}
	scratch := g.pickScratch8(inst)
	g.emitf("    LD %s, A    ; save r%d before ALU overwrite", scratch, src)
	g.physOverride[src] = scratch
	if g.pendingAccReg == src {
		g.pendingAccReg = NoReg
	}
}

func (g *z80cg) isVregLiveAfter(vreg Reg, target *Inst) bool {
	if g.curBlock == nil || vreg == NoReg {
		return false
	}
	found := false
	for _, inst := range g.curBlock.Insts {
		if !found {
			if inst == target {
				found = true
			}
			continue
		}
		// Check if vreg is used by this instruction
		for _, r := range inst.Uses() {
			if r == vreg {
				return true
			}
		}
	}
	// Check terminator
	if g.curBlock.Term != nil {
		for _, r := range g.curBlock.Term.termUses() {
			if r == vreg {
				return true
			}
		}
	}
	return false
}

// regsLiveAfterInst includes successor and loop-backedge uses via CFG liveness.
func (g *z80cg) regsLiveAfterInst(target *Inst) map[Reg]bool {
	live := make(map[Reg]bool)
	if g.curBlock == nil {
		return live
	}
	lr := ComputeLiveness(g.fn)
	if out := lr.LiveOutOf(g.fn, g.curBlock); out != nil {
		for _, r := range out.Slice() {
			live[r] = true
		}
	}
	if g.curBlock.Term != nil {
		for _, r := range g.curBlock.Term.termUses() {
			live[r] = true
		}
	}
	for i := len(g.curBlock.Insts) - 1; i >= 0; i-- {
		inst := g.curBlock.Insts[i]
		if inst == target {
			break
		}
		delete(live, inst.Dst)
		for _, r := range inst.ExtraRets {
			delete(live, r)
		}
		for _, r := range inst.Uses() {
			if r != NoReg {
				live[r] = true
			}
		}
	}
	return live
}

// classPhysRegs returns physical register names associated with a register class.
func classPhysRegs(cls RegClass) []string {
	switch cls {
	case ClassAcc:
		return []string{"A"}
	case ClassCounter:
		return []string{"B"}
	case ClassGeneral:
		return []string{"B", "C", "D", "E", "H", "L"}
	case ClassPointer:
		return []string{"H", "L", "D", "E"}
	case ClassIndex:
		return []string{"D", "E"}
	case ClassPair:
		return []string{"H", "L", "D", "E", "B", "C"}
	}
	return nil
}

// ValidZ80Clobber reports whether a register is supported by caller-save.
// SP, PC, special and shadow registers cannot be preserved by this ABI.
func ValidZ80Clobber(name string) bool {
	_, ok := regToPairMap[name]
	return ok
}

var allZ80Clobbers = []string{"A", "B", "BC", "C", "D", "DE", "E", "F", "H", "HL", "IX", "IXH", "IXL", "IY", "IYH", "IYL", "L"}

// computeClobbers returns a conservative sorted superset of emitted writes.
func computeClobbers(f *Func, ar *AllocResult) []string {
	if f != nil && f.Attrs.IsExtern && f.Contract.ExternClobbers != nil {
		var names []string
		for _, name := range f.Contract.ExternClobbers {
			// Other frontends must not accidentally turn invalid contracts into
			// preservation guarantees.
			if !ValidZ80Clobber(name) {
				return allZ80Clobbers
			}
			names = append(names, name)
		}
		// Outputs are writes even when omitted from the declaration.
		for _, ret := range f.Contract.Returns {
			names = append(names, canonicalReturnLoc(ret.Class, ret.Ty))
		}
		sortStrings(names)
		return slices.Compact(names)
	}
	// Allocation destinations omit implicit writes by ALU lowering, parallel
	// copies, runtime helpers and transitive calls. Keep the default conservative.
	return allZ80Clobbers
}

func sortStrings(ss []string) {
	// Simple insertion sort for small lists.
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// pushWord stages a value while preserving all register operands and flags.
// EX (SP),HL restores HL and leaves the loaded word on the stack.
func (g *z80cg) pushWord(src string) {
	if isPairReg(src) || isIXY(src) {
		g.emitf("    PUSH %s", src)
		return
	}
	g.emit("    PUSH HL")
	if isSpill(src) {
		g.loadSpill16("HL", src)
	} else {
		g.emit("    PUSH AF")
		g.emitLD8("L", src)
		g.emit("    LD H, 0")
		g.emit("    POP AF")
	}
	g.emit("    EX (SP), HL")
	g.invalidate("HL")
}
