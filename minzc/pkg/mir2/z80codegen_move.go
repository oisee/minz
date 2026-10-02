package mir2

import (
	"fmt"
	"strings"
)

// smcPatcherInfo records metadata for a synthesised @smc patcher function.
// Sym is "funcName$paramName"; eqLabel is the EQU pointing at the imm16 bytes.
type smcPatcherInfo struct {
	sym     string // e.g. "draw_player$r0"
	eqLabel string // e.g. "draw_player$r0$imm"
}

// isIXY reports whether reg is an index register (IX or IY).
// Z80 indexed loads/stores require explicit displacement: (IX+d), never bare (IX).
func isIXY(reg string) bool { return reg == "IX" || reg == "IY" }

// isIXYReg returns true for the undocumented IXH/IXL/IYH/IYL half-registers.
// These share the DD/FD prefix with (IX+d)/(IY+d) addressing, so they cannot
// be combined with (HL) in a single instruction.
func isIXYReg(reg string) bool {
	return reg == "IXH" || reg == "IXL" || reg == "IYH" || reg == "IYL"
}

// ptrIndirect returns the memory-operand string for a pointer register.
// For IX/IY the Z80 requires an explicit displacement byte — even zero:
//
//	HL → "(HL)"
//	IX → "(IX+0)"   DE → "(DE)"
//
// An optional byte offset d shifts the displacement for 16-bit multi-byte accesses.
func ptrIndirect(ptr string, d int) string {
	if isIXY(ptr) {
		if d == 0 {
			return fmt.Sprintf("(%s+0)", ptr)
		}
		return fmt.Sprintf("(%s+%d)", ptr, d)
	}
	return fmt.Sprintf("(%s)", ptr)
}

// isSimpleReg reports whether s names a plain physical register (no parens,
// no digits) — safe to use as an alias key in holdsPhys.
func isSimpleReg(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return false
		}
	}
	return true
}

// ── Register topology ─────────────────────────────────────────────────────────
//
// Writing to a 16-bit pair register (e.g. HL) implicitly changes its bytes
// (H, L), and vice versa: writing to H invalidates HL.  These tables encode
// that topology so invalidate() can clear all affected aliases at once.

// regBytes maps a 16-bit register to its 8-bit byte components.
var regBytes = map[string][]string{
	"BC": {"B", "C"},
	"DE": {"D", "E"},
	"HL": {"H", "L"},
	"IX": {"IXH", "IXL"},
	"IY": {"IYH", "IYL"},
}

// regParent maps an 8-bit component to its 16-bit parent register.
var regParent = map[string]string{
	"B": "BC", "C": "BC",
	"D": "DE", "E": "DE",
	"H": "HL", "L": "HL",
	"IXH": "IX", "IXL": "IX",
	"IYH": "IY", "IYL": "IY",
}

// killOne removes a single register and its bidirectional partner from holdsPhys.
func (g *z80cg) killOne(r string) {
	if partner, ok := g.holdsPhys[r]; ok {
		delete(g.holdsPhys, partner)
		delete(g.holdsPhys, r)
	}
}

// invalidate removes all tracking for r: kills r itself, its byte components
// (if r is a 16-bit pair), and its parent pair (if r is an 8-bit component).
func (g *z80cg) invalidate(r string) {
	g.killOne(r)
	if bytes, ok := regBytes[r]; ok {
		for _, b := range bytes {
			g.killOne(b)
		}
	}
	if parent, ok := regParent[r]; ok {
		g.killOne(parent)
	}
}

// setCopy records that dst was loaded from src (they now hold the same value).
// Invalidates dst's old aliases and clears src's previous bidirectional link
// before establishing the new one, preventing stale reverse entries.
func (g *z80cg) setCopy(dst, src string) {
	g.invalidate(dst)
	g.killOne(src) // break src's old reverse-link (could be left by a prior setCopy)
	if isSimpleReg(dst) && isSimpleReg(src) {
		g.holdsPhys[dst] = src
		g.holdsPhys[src] = dst
	}
}

// holdsValue reports whether physical register a currently holds the same
// value as b (either they are the same register, or a copy alias exists).
func (g *z80cg) holdsValue(a, b string) bool {
	return a == b || g.holdsPhys[a] == b
}

// emitLDA emits "LD A, src" and records the copy alias A ≡ src.
// When src is a LocMem address ($Fxxx), emits "LD A, ($Fxxx)" — the only
// valid Z80 form for loading an 8-bit value from an absolute 16-bit address.
func (g *z80cg) emitLDA(src string) {
	if src == "F" {
		// F register cannot be loaded directly. Materialise carry flag.
		g.emit("    SBC A, A") // A=0xFF if C=1, A=0x00 if C=0
		g.invalidate("A")
		return
	}
	if isSpill(src) {
		g.emitf("    LD A, (%s)", src)
	} else if isPairReg(src) {
		// Width mismatch: pair→8bit. Take low byte.
		g.emitf("    LD A, %s", lowByte(src))
	} else {
		g.emitf("    LD A, %s", src)
	}
	g.setCopy("A", src)
}

// loc returns the physical location name for virtual register r.
// For LocMem registers the allocator stores the actual byte address in Offset;
// return it as a hex literal so the assembler resolves it correctly.
// e.g. Offset=0xF001 → "$F001", which assembles as LD D, ($F001) / LD ($F001), A etc.
func (g *z80cg) loc(r Reg) string {
	if r == NoReg {
		return "?"
	}
	if phys, ok := g.physOverride[r]; ok {
		return phys
	}
	pl := g.ar.Loc(r)
	if pl.Kind == LocMem {
		return g.spillLabel(r)
	}
	return pl.Name
}

// spillLabel returns a named label for a spilled virtual register.
// Format: _spill_{funcName}_r{regNum}
// The label is emitted in the data section at end of function.
func (g *z80cg) spillLabel(r Reg) string {
	return Z80SpillLabel(g.fn.Name, r)
}

// Z80SpillLabel names the storage used by production codegen for a spilled
// virtual register. Assertion bootstraps must initialize this same storage.
func Z80SpillLabel(funcName string, r Reg) string {
	return fmt.Sprintf("_spill_%s_r%d", sanitizeIdent(funcName), int(r))
}

// spillWidth returns the byte width of a spilled register (1, 2, or 3 for eZ80).
func (g *z80cg) spillWidth(r Reg) int {
	if info, ok := collectRegInfo(g.fn)[r]; ok {
		return (info.Ty.Width() + 7) / 8
	}
	return 1
}

// physName returns the assembly-level name for a PhysLoc.
// For LocMem, returns the named spill label; for everything else, returns pl.Name.
func physName(pl PhysLoc) string {
	// Note: physName is used outside genFunc context (e.g. buildBlockCopies)
	// where we don't have access to g.fn. Fall back to $F0xx for those cases.
	if pl.Kind == LocMem {
		return fmt.Sprintf("$%04X", pl.Offset)
	}
	return pl.Name
}

// physNameFor returns the assembly-level name for a PhysLoc within function context.
func (g *z80cg) physNameFor(pl PhysLoc, r Reg) string {
	if pl.Kind == LocMem {
		return g.spillLabel(r)
	}
	return pl.Name
}

// emitLD8 emits a safe 8-bit LD, handling spill labels.
//
// Backend policy: IXH/IXL/IYH/IYL are treated as first-class 8-bit registers,
// not as exotic fallbacks reserved only for bit operations. Direct half-reg
// paths should be preferred whenever they are encoding-correct; alternative
// routes through A / pair copies / memory are justified only by real DD/FD
// prefix conflicts or by proven size/timing wins elsewhere.
//
// Z80 only supports LD A,(nn) and LD (nn),A for absolute memory.
// Routes through shadow A (EX AF,AF') to preserve main A register.
func (g *z80cg) emitLD8(dst, src string) {
	if dst == src {
		return
	}
	// Width mismatch: pair→8bit truncation.
	if isPairReg(src) && !isPairReg(dst) {
		src = lowByte(src)
	}
	// Width mismatch: 8bit→pair widening (u8 result stored in pair reg).
	if !isPairReg(src) && isPairReg(dst) {
		g.emitf("    LD %s, %s", lowByte(dst), src)
		g.emitf("    LD %s, 0", highByte(dst))
		return
	}
	// DD/FD prefix conflicts — route through shadow A:
	if (isIXYReg(src) && (dst == "H" || dst == "L")) ||
		(isIXYReg(dst) && (src == "H" || src == "L")) ||
		(isIXYReg(src) && isIXYReg(dst) && src[:2] != dst[:2]) {
		g.emitMovViaAltA(dst, src)
		return
	}
	dstSpill := isSpill(dst)
	srcSpill := isSpill(src)
	if srcSpill && dstSpill {
		// Spill-to-spill: route through shadow A.
		g.emit("    EX AF, AF'")
		g.emitf("    LD A, (%s)", src)
		g.emitf("    LD (%s), A", dst)
		g.emit("    EX AF, AF'")
	} else if srcSpill && dst == "A" {
		g.emitf("    LD A, (%s)", src)
	} else if srcSpill {
		// Load spill to non-A register via shadow A.
		g.emit("    EX AF, AF'")
		g.emitf("    LD A, (%s)", src)
		g.emitLD8(dst, "A")
		g.emit("    EX AF, AF'")
	} else if dstSpill && src == "A" {
		g.emitf("    LD (%s), A", dst)
	} else if dstSpill {
		// Store non-A register to spill via shadow A.
		g.emit("    EX AF, AF'")
		g.emitf("    LD A, %s", src)
		g.emitf("    LD (%s), A", dst)
		g.emit("    EX AF, AF'")
	} else {
		g.emitf("    LD %s, %s", dst, src)
	}
}

// emitMovViaAltA routes a single 8-bit LD through shadow A (EX AF,AF')
// to avoid clobbering the main A register. Used for DD/FD prefix conflicts
// where LD H,IXH would encode as NOP.
// Cost: 20T (EX 4T + LD A,src 4-8T + LD dst,A 4T + EX 4T).
func (g *z80cg) emitMovViaAltA(dst, src string) {
	g.emit("    EX AF, AF'")
	g.emitf("    LD A, %s", src)
	g.emitLD8(dst, "A")
	g.emit("    EX AF, AF'")
}

// isSpill returns true if the operand is a LocMem spill (named _spill_ label or legacy $F0xx).
func isSpill(s string) bool {
	return strings.HasPrefix(s, "$") || strings.HasPrefix(s, "_spill_") || strings.HasPrefix(s, "._call_result_")
}

// loadSpill8 loads an 8-bit spill value into the target register.
// Uses A as intermediary (LD A,(nn) is the only valid 8-bit absolute load).
func (g *z80cg) loadSpill8(dst, src string) {
	if dst == "A" {
		if pair := g.tsmcPairByAddr(src); pair != nil {
			label := g.tsmcReloadLabel(pair.reg, g.blockIdx, g.curInstIdx, 0)
			if label != "" {
				g.emitTSMCReload(label, "A", 8)
				return
			}
		}
		g.emitf("    LD A, (%s)", src)
	} else {
		if pair := g.tsmcPairByAddr(src); pair != nil {
			label := g.tsmcReloadLabel(pair.reg, g.blockIdx, g.curInstIdx, 0)
			if label != "" {
				g.emitTSMCReload(label, dst, 8)
				return
			}
		}
		g.emitf("    LD A, (%s)", src)
		g.emitLD8(dst, "A")
		g.invalidate("A")
	}
}

// storeSpill8 stores an 8-bit register value to a spill address.
func (g *z80cg) storeSpill8(dst, src string) {
	if src != "A" {
		g.emitLDA(src)
	}
	if pair := g.tsmcPairByAddr(dst); pair != nil {
		g.emitTSMCSpill(pair, "A")
	} else {
		g.emitf("    LD (%s), A", dst)
	}
}

// loadSpill16 loads a 16-bit spill value into a register pair.
func (g *z80cg) loadSpill16(dst, src string) {
	if pair := g.tsmcPairByAddr(src); pair != nil {
		label := g.tsmcReloadLabel(pair.reg, g.blockIdx, g.curInstIdx, 0)
		if label != "" {
			g.emitTSMCReload(label, dst, 16)
			return
		}
	}
	g.emitf("    LD %s, (%s)", dst, src)
}

// storeSpill16 stores a 16-bit pair to a spill address.
func (g *z80cg) storeSpill16(dst, src string) {
	if pair := g.tsmcPairByAddr(dst); pair != nil {
		g.emitTSMCSpill(pair, src)
	} else {
		// Z80: only LD (nn),HL is native. DE/BC/SP need ED prefix (supported).
		g.emitf("    LD (%s), %s", dst, src)
	}
}

// fixOrphanedTSMCStores scans the emitted asm for _tsmc_ labels that are
// referenced (by LD (_tsmc_label+N), A store patches) but never defined
// (no _tsmc_label: reload instruction). For each orphan, replaces the
// TSMC store patches with regular _spill_ stores:
//
//	LD (_tsmc_func_rN_M+1), A  →  LD (_spill_func_rN), A
//	LD (_tsmc_func_rN_M+2), A  →  LD (_spill_func_rN+1), A
func fixOrphanedTSMCStores(sb *strings.Builder, funcLabel string) {
	asm := sb.String()
	tsmcPrefix := "_tsmc_" + funcLabel + "_r"

	// Collect all _tsmc_ labels that appear as references (in store patches)
	referenced := make(map[string]bool)
	for idx := 0; ; {
		pos := strings.Index(asm[idx:], tsmcPrefix)
		if pos < 0 {
			break
		}
		pos += idx
		// Extract: _tsmc_func_rN_M
		end := pos
		for end < len(asm) && asm[end] != '+' && asm[end] != ')' && asm[end] != ':' &&
			asm[end] != ' ' && asm[end] != '\n' {
			end++
		}
		label := asm[pos:end]
		referenced[label] = true
		idx = end
	}

	// Check which are defined (have "label:" in the asm)
	changed := false
	for label := range referenced {
		if strings.Contains(asm, label+":") {
			continue // defined — TSMC reload is working
		}
		// Orphaned: extract register number to build _spill_ label.
		// Label format: _tsmc_func_rN_M → _spill_func_rN
		tsmcSuffix := label[len("_tsmc_"):]
		// Find last underscore (the _M part)
		lastUS := strings.LastIndex(tsmcSuffix, "_")
		if lastUS < 0 {
			continue
		}
		spillLabel := "_spill_" + tsmcSuffix[:lastUS]
		// Replace TSMC store patches with regular spill stores
		asm = strings.ReplaceAll(asm, "LD ("+label+"+1), A", "LD ("+spillLabel+"), A")
		asm = strings.ReplaceAll(asm, "LD ("+label+"+2), A", "LD ("+spillLabel+"+1), A")
		changed = true
	}

	if changed {
		sb.Reset()
		sb.WriteString(asm)
	}
}

// ── Type conversions ──────────────────────────────────────────────────────────

func (g *z80cg) genExt(inst *Inst) {
	dst := g.loc(inst.Dst)
	src := g.loc(inst.Src[0])
	srcW := inst.SrcTy.Width()
	dstW := inst.Ty.Width()
	if inst.SrcTy == TyBool && dstW == 8 && src == "F" {
		// A comparison in F is a predicate, not a byte. Materialize its
		// actual condition as 0/1; carry alone is wrong for GE/LE/EQ.
		g.emitFlagPredicateByte(g.condCode(g.fn, inst.Src[0]))
		if dst != "A" {
			g.emitLD8(dst, "A")
		}
		g.pendingFlagReg = NoReg
		return
	}

	if srcW == 8 && dstW == 16 {
		// Zero-extend u8 → u16.
		// dst should be a 16-bit reg (HL/DE/BC/IX/IY).
		// Most natural: put source in low byte, zero high byte.
		lo := lowByte(dst)
		hi := highByte(dst)
		if src != lo {
			if (isIXYReg(src) && (lo == "H" || lo == "L")) ||
				((src == "H" || src == "L") && isIXYReg(lo)) {
				g.emitMovViaAltA(lo, src)
			} else {
				g.emitLD8(lo, src)
			}
		}
		g.emitLD8(hi, "0")
		return
	}
	// Fallback.
	g.comment(fmt.Sprintf("TODO: ext %s→%s %s→%s", inst.SrcTy, inst.Ty, src, dst))
}

func (g *z80cg) genSext(inst *Inst) {
	// Sign extension uses A as scratch, but the narrow source may remain live.
	g.emit("    PUSH AF")
	g.genSextValue(inst)
	g.emit("    POP AF")
	g.invalidate("A")
	g.invalidate(g.loc(inst.Dst))
}

func (g *z80cg) genSextValue(inst *Inst) {
	dst := g.loc(inst.Dst)
	src := g.loc(inst.Src[0])
	srcW := inst.SrcTy.Width()
	dstW := inst.Ty.Width()

	if srcW == 24 && dstW == 32 {
		if src != dst {
			g.emitMov32(dst, src)
		}
		g.emit("    EXX")
		g.emitLDA(lowByte(dst))
		g.emit("    RLCA")
		g.emit("    SBC A, A")
		g.emitLD8(highByte(dst), "A")
		g.emit("    EXX")
		g.invalidate("A")
		g.invalidate(dst)
		return
	}

	if dstW >= 24 && (srcW == 8 || srcW == 16) {
		hi, lo := highByte(dst), lowByte(dst)
		if srcW == 8 {
			if src != "A" {
				g.emitLDA(src)
			}
			g.emitLD8(lo, "A")
			g.emit("    RLCA")
			g.emit("    SBC A, A")
			g.emitLD8(hi, "A")
		} else {
			if src != dst {
				g.emitMov(dst, src, 16)
			}
			g.emitLDA(hi)
			g.emit("    RLCA")
			g.emit("    SBC A, A")
		}
		g.emit("    EXX")
		g.emitLD8(lo, "A")
		if dstW == 24 {
			g.emitf("    LD %s, 0", hi)
		} else {
			g.emitLD8(hi, "A")
		}
		g.emit("    EXX")
		g.invalidate("A")
		g.invalidate(dst)
		return
	}

	if srcW == 8 && dstW == 16 {
		// Sign-extend u8 → u16 via A.
		lo := lowByte(dst)
		hi := highByte(dst)
		if src != "A" {
			g.emitLDA(src)
		}
		if lo != "A" {
			g.emitLD8(lo, "A")
		}
		// Propagate sign bit into high byte: RLCA; SBC A,A gives 0x00 or 0xFF.
		g.emit("    RLCA")
		g.emit("    SBC A, A")
		if hi != "A" {
			g.emitLD8(hi, "A")
		}
		return
	}
	g.comment(fmt.Sprintf("TODO: sext %s→%s %s→%s", inst.SrcTy, inst.Ty, src, dst))
}

// ── 32-bit DWord helpers ──────────────────────────────────────────────────────

// isDWord reports whether virtual register r is allocated to a LocDWord.
func (g *z80cg) isDWord(r Reg) bool {
	return r != NoReg && g.ar.Loc(r).Kind == LocDWord
}

// emitMov32 moves a 32-bit DWord value from src pair to dst pair using PUSH/EXX.
//
// Strategy (47T, 5 instructions):
//
//	PUSH src      ; push main src_lo  (11T)
//	EXX           ; switch to shadow  (4T)
//	PUSH src      ; push shadow src_hi (11T)
//	POP dst       ; pop into shadow dst_hi (10T)
//	EXX           ; switch back (4T)
//	POP dst       ; pop into main dst_lo (10T)
//
// The push of src_lo happens before EXX, so it's the main-bank value.
// After EXX, src refers to the shadow-bank value (src_hi).
func (g *z80cg) emitMov32(dst, src string) {
	if dst == src {
		return
	}
	if isSpill(src) && isSpill(dst) {
		g.emit("    PUSH HL")
		g.emit("    EXX")
		g.emit("    PUSH HL")
		g.emit("    EXX")
		g.emitMov32("HL", src)
		g.emitMov32(dst, "HL")
		g.emit("    EXX")
		g.emit("    POP HL")
		g.emit("    EXX")
		g.emit("    POP HL")
		return
	}
	if isSpill(src) && isPairReg(dst) {
		g.emitf("    LD %s, (%s)", dst, src)
		g.emit("    EXX")
		g.emitf("    LD %s, (%s+2)", dst, src)
		g.emit("    EXX")
		return
	}
	if isSpill(dst) && isPairReg(src) {
		g.emitf("    LD (%s), %s", dst, src)
		g.emit("    EXX")
		g.emitf("    LD (%s+2), %s", dst, src)
		g.emit("    EXX")
		return
	}
	g.emitf("    PUSH %s", src) // save main src_lo
	g.emit("    EXX")           // switch to shadow
	g.emitf("    PUSH %s", src) // save shadow src_hi
	g.emitf("    POP %s", dst)  // restore shadow dst_hi
	g.emit("    EXX")           // switch back to main
	g.emitf("    POP %s", dst)  // restore main dst_lo
}

// ── Move helper ───────────────────────────────────────────────────────────────

func (g *z80cg) emitMov(dst, src string, widthBits int) {
	if dst == src {
		return
	}

	// ── Flag-register special cases ───────────────────────────────────────────

	if dst == "F" {
		// Materialise a register value into the flag register.
		// Convention: Z=1 (zero) means false, NZ means true — i.e. AND A / OR A.
		// Callers using CmpNe (NZ) as the flag condition get this for free.
		if src == "A" {
			g.emit("    AND A") // sets Z/NZ and clears C based on A's value
		} else {
			// Load into A first, then set flags.
			g.emitf("    LD A, %s", src)
			g.emit("    AND A")
		}
		g.invalidate("A")
		return
	}

	if src == "F" {
		if g.emitKnownFlagCopy(dst, widthBits) {
			return
		}
		// Materialise a flag result into a register.
		// Uses the SBC A,A trick for C-flag convention: A = 0xFF (carry set = true)
		// or 0x00 (carry clear = false).  Works for CmpLt/CmpUlt/CmpGe/CmpUge.
		//
		// For Z-flag convention (CmpEq/CmpNe), the sequence is:
		//   JR NZ, $+3 / SCF / then SBC A,A — converts Z to C, then materialise.
		// We use the simpler carry path by default; HIR lowers bool-returning
		// functions to carry-based comparisons where possible.
		if dst == "A" {
			g.emit("    SBC A, A") // A=0xFF if C=1 (true), A=0x00 if C=0 (false)
		} else if isPairReg(dst) {
			// Flag → 16-bit pair: materialise to A, then zero-extend.
			// SBC A,A gives 0xFF (true) or 0x00 (false).
			g.emit("    SBC A, A")
			g.emitf("    LD %s, A", lowByte(dst))
			g.emitf("    LD %s, 0", highByte(dst))
		} else {
			g.emit("    SBC A, A")
			g.emitLD8(dst, "A")
		}
		g.invalidate("A")
		return
	}

	// ── Normal register moves ─────────────────────────────────────────────────

	if widthBits <= 8 {
		// LocMem spill slot: "LD r, $Fxxx" and "LD $Fxxx, r" are not valid Z80.
		// Only "LD A, (nn)" and "LD (nn), A" work for absolute 8-bit memory I/O.
		// TSMC spill: use self-modifying code when eligible.
		switch {
		case isSpill(src) && dst == "A":
			if pair := g.tsmcPairByAddr(src); pair != nil {
				g.emitTSMCReload(g.tsmcReloadLabel(pair.reg, g.blockIdx, g.curInstIdx, 0), "A", 8)
			} else {
				g.emitf("    LD A, (%s)", src)
			}
		case isSpill(src):
			if pair := g.tsmcPairByAddr(src); pair != nil {
				g.emitTSMCReload(g.tsmcReloadLabel(pair.reg, g.blockIdx, g.curInstIdx, 0), dst, 8)
			} else {
				g.emitf("    LD A, (%s)", src)
				g.emitLD8(dst, "A")
				g.invalidate("A")
			}
		case isSpill(dst) && src == "A":
			if pair := g.tsmcPairByAddr(dst); pair != nil {
				g.emitTSMCSpill(pair, "A")
			} else {
				g.emitf("    LD (%s), A", dst)
			}
		case isSpill(dst):
			if pair := g.tsmcPairByAddr(dst); pair != nil {
				g.emitLDA(src)
				g.emitTSMCSpill(pair, "A")
			} else {
				g.emitLDA(src)
				g.emitf("    LD (%s), A", dst)
			}
		default:
			// Width mismatch: 8-bit move but one operand is a register pair.
			// DD/FD prefix conflict must be checked BEFORE width mismatch,
			// because lowByte(HL)="L" + IXL src = DD NOP.
			if (isIXYReg(dst) && (src == "H" || src == "L")) ||
				(isIXYReg(src) && (dst == "H" || dst == "L")) {
				g.emitMovViaAltA(dst, src)
			} else if isPairReg(src) && !isPairReg(dst) {
				// pair→8bit: truncate (take low byte).
				lo := lowByte(src)
				if (isIXYReg(lo) && (dst == "H" || dst == "L")) ||
					((lo == "H" || lo == "L") && isIXYReg(dst)) {
					g.emitMovViaAltA(dst, lo)
				} else {
					g.emitLD8(dst, lo)
				}
			} else if !isPairReg(src) && isPairReg(dst) {
				// 8bit→pair: zero-extend.
				lo := lowByte(dst)
				if (isIXYReg(src) && (lo == "H" || lo == "L")) ||
					((src == "H" || src == "L") && isIXYReg(lo)) {
					g.emitMovViaAltA(lo, src)
				} else {
					g.emitLD8(lo, src)
				}
				g.emitf("    LD %s, 0", highByte(dst))
			} else {
				g.emitLD8(dst, src)
				if isSimpleReg(dst) && isSimpleReg(src) {
					g.setCopy(dst, src)
				}
			}
		}
		return
	}

	// 32-bit DWord move via PUSH/EXX sequence.
	if widthBits == 32 {
		g.emitMov32(dst, src)
		return
	}

	// 16-bit register move.
	switch {
	case dst == "HL" && src == "DE":
		// Copy DE→HL: must NOT use EX DE,HL (swap destroys DE).
		g.emitf("    LD %s, %s", highByte(dst), highByte(src)) // LD H, D
		g.emitf("    LD %s, %s", lowByte(dst), lowByte(src))   // LD L, E
	case dst == "DE" && src == "HL":
		// Copy HL→DE: must NOT use EX DE,HL (swap destroys HL).
		g.emitf("    LD %s, %s", highByte(dst), highByte(src)) // LD D, H
		g.emitf("    LD %s, %s", lowByte(dst), lowByte(src))   // LD E, L
	case (dst == "IX" || dst == "IY") && (src == "DE" || src == "BC"):
		// DE/BC → IX/IY: undocumented byte-copy (DD/FD prefix), 2×8T = 16T.
		// Safe because D, E, B, C are NOT substituted by the DD/FD prefix.
		// e.g. DE→IX: LD IXH, D (DD 62) / LD IXL, E (DD 6B)
		//      BC→IX: LD IXH, B (DD 60) / LD IXL, C (DD 69)
		g.emitf("    LD %s, %s", highByte(dst), highByte(src))
		g.emitf("    LD %s, %s", lowByte(dst), lowByte(src))
	case (dst == "IX" || dst == "IY") && src == "HL":
		// HL→IX: byte-copy is INVALID — DD prefix substitutes H→IXH, L→IXL,
		// so LD IXH,H encodes as LD IXH,IXH (NOP). Must use PUSH/POP.
		g.emitf("    PUSH HL")
		g.emitf("    POP %s", dst)
	case (src == "IX" || src == "IY") && (dst == "DE" || dst == "BC"):
		// IX/IY → DE/BC: safe byte-copy, D/E/B/C not substituted by DD prefix.
		// e.g. IX→DE: LD D, IXH (DD 57? no — DD 7A? no) actually DD prefix on LD D,H:
		// opcode 0x54 = LD D,H → DD 54 = LD D,IXH ✓ (D is destination, IXH is source)
		g.emitf("    LD %s, %s", highByte(dst), highByte(src))
		g.emitf("    LD %s, %s", lowByte(dst), lowByte(src))
	case (src == "IX" || src == "IY") && dst == "HL":
		// IX→HL: byte-copy INVALID (H,L substituted). Must use PUSH/POP.
		g.emitf("    PUSH %s", src)
		g.emit("    POP HL")
	default:
		// If src is an 8-bit register being widened to a 16-bit pair,
		// PUSH/POP is wrong: PUSH A doesn't exist (only PUSH AF, which
		// would corrupt the low byte with the flags register).
		// Use zero-extension instead: LD lo, src ; LD hi, 0.
		if isSimpleReg(src) && !isPairReg(src) && isPairReg(dst) {
			lo := lowByte(dst)
			hi := highByte(dst)
			if lo != src {
				g.emitLD8(lo, src)
			}
			g.emitf("    LD %s, 0", hi)
			break
		}
		// Spill-to-spill: route through HL.
		if isSpill(src) && isSpill(dst) {
			g.emitf("    LD HL, (%s)", src)
			g.emitf("    LD (%s), HL", dst)
			g.invalidate("HL")
			break
		}
		// LocMem spill slot → register pair: use LD rr, (nn).
		if isSpill(src) {
			g.emitf("    LD %s, (%s)", dst, src)
			break
		}
		// register pair → LocMem spill slot: use LD (nn), rr.
		if isSpill(dst) {
			if !isPairReg(src) {
				g.emitLD8(dst, src)
				g.emitLD8(highByte(dst), "0")
				break
			}
			g.emitf("    LD (%s), %s", dst, src)
			break
		}
		// 16-bit pair → 8-bit: truncate (take low byte).
		if isPairReg(src) && !isPairReg(dst) {
			lo := lowByte(src)
			if (isIXYReg(lo) && (dst == "H" || dst == "L")) ||
				((lo == "H" || lo == "L") && isIXYReg(dst)) {
				g.emitMovViaAltA(dst, lo)
			} else {
				g.emitLD8(dst, lo)
			}
			break
		}
		// 8-bit → 16-bit pair: zero-extend.
		if !isPairReg(src) && isPairReg(dst) {
			lo := lowByte(dst)
			if (isIXYReg(src) && (lo == "H" || lo == "L")) ||
				((src == "H" || src == "L") && isIXYReg(lo)) {
				g.emitMovViaAltA(lo, src)
			} else {
				g.emitLD8(lo, src)
			}
			g.emitf("    LD %s, 0", highByte(dst))
			break
		}
		// General 16-bit move via PUSH/POP.
		g.emitf("    PUSH %s", src)
		g.emitf("    POP %s", dst)
	}
	// Track the copy alias so downstream redundant moves can be suppressed.
	g.setCopy(dst, src)
}

// inferTyFromAlloc guesses the type of a virtual register from its allocated
// physical location. 16-bit pair → TyU16; 8-bit reg → TyU8.
func inferTyFromAlloc(ar *AllocResult, r Reg) Ty {
	loc := ar.Loc(r)
	switch loc.Kind {
	case LocReg:
		if isPairReg(loc.Name) {
			return TyU16
		}
		return TyU8
	case LocIXY:
		return TyU16
	case LocIXY8, LocShadow:
		return TyU8
	default:
		return TyU8
	}
}

// ── 16-bit register half helpers ──────────────────────────────────────────────
//
// These helpers encode the project's "first-class half-register" policy:
// HL/DE/BC/IX/IY all expose stable low/high 8-bit halves for generic backend
// reasoning. IXH/IXL/IYH/IYL must be considered alongside H/L/D/E/B/C in all
// legal direct paths; they are not special-cased only for bit ops.

// lowByte returns the low-byte name of a 16-bit register.
func lowByte(rr string) string {
	switch rr {
	case "HL":
		return "L"
	case "DE":
		return "E"
	case "BC":
		return "C"
	case "IX":
		return "IXL"
	case "IY":
		return "IYL"
	}
	return rr // already 8-bit
}

// highByte returns the high-byte name of a 16-bit register.
func highByte(rr string) string {
	if isSpill(rr) {
		return rr + "+1"
	}
	switch rr {
	case "HL":
		return "H"
	case "DE":
		return "D"
	case "BC":
		return "B"
	case "IX":
		return "IXH"
	case "IY":
		return "IYH"
	}
	return rr
}

// isPairReg reports whether r is a 16-bit register pair name.
func isPairReg(r string) bool {
	switch r {
	case "HL", "DE", "BC", "IX", "IY":
		return true
	}
	return false
}

// parentPair returns the 16-bit register pair containing an 8-bit register.
func parentPair(r string) string {
	switch r {
	case "A":
		return "AF" // rarely useful; callers should special-case A
	case "B", "C":
		return "BC"
	case "D", "E":
		return "DE"
	case "H", "L":
		return "HL"
	case "IXH", "IXL":
		return "IX"
	case "IYH", "IYL":
		return "IY"
	}
	return r // already a pair or unknown
}

// promote8toPair zero-extends an 8-bit register into a 16-bit pair suitable
// for ADD HL,rr or SBC HL,rr. For A, routes through BC (since AF is not a
// valid arithmetic pair). For others, uses parentPair with high byte zeroed.
// Returns the pair name to use.
func (g *z80cg) promote8toPair(r string) string {
	if r == "A" {
		g.emit("    LD C, A")
		g.emit("    LD B, 0")
		return "BC"
	}
	pair := parentPair(r)
	g.emitf("    LD %s, 0", highByte(pair))
	return pair
}

// ── PUSH/POP pair helpers ─────────────────────────────────────────────────────

// toPair returns the 16-bit register pair containing the given register.
func toPair(r string) string {
	switch r {
	case "A", "F", "AF":
		return "AF"
	case "B", "C", "BC":
		return "BC"
	case "D", "E", "DE":
		return "DE"
	case "H", "L", "HL":
		return "HL"
	}
	return r // IX, IY already pairs
}
