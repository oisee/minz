package mir2

import (
	"fmt"
	"math/bits"
)

// lutLoadPat describes a page-aligned LUT access merged at codegen time.
type lutLoadPat struct {
	sym     string // page-aligned global symbol
	src8Reg Reg    // virtual reg holding the 8-bit index (after lo-subtract if any)
}

type bitStorePat struct {
	bit        int
	byteOffset int
	op         string // "SET" or "RES"
}

type bitCmpPat struct {
	bit        int
	byteOffset int
	ptrReg     Reg
	srcReg     Reg
	useMem     bool
}

// globalFieldInfo describes a global struct field access that can be lowered
// to direct Z80 addressing: LD A,(sym__field) or LD (sym__field),A.
//
// The EQU label is: {sanitizeIdent(sym)}__{fieldName}  (or just sanitizeIdent(sym) for offset 0).
// Emission: LD A,(sym__field)  for loads; LD A,val + LD (sym__field),A  for stores.
type globalFieldInfo struct {
	sym       string // global symbol name (raw, before sanitize)
	offset    int64  // byte offset within the global
	fieldName string // field name for the EQU label (may be empty → use offset)
	// HL-chain: when true, adjacent stores share one HL address load.
	hlChain bool // part of a consecutive-field HL-chain group
	hlHead  bool // first in group → load its field address into HL
	hlLast  bool // last in group  → no INC HL after
}

// ── Page-aligned LUT pre-scan ──────────────────────────────────────────────────

// scanLUTPatterns detects the pattern emitted by LUTGen for page-aligned tables:
//
//	idx16 = Ext(src8, u8→u16, ClassIndex)
//	base  = AddrOf("sym")                      ← sym has Align==256
//	ptr   = PtrAdd(base, idx16)
//	dst   = Load(ptr, u8, ClassAcc)
//
// When found:
//   - g.lutLoadPat[dst] = {sym, src8Reg}
//   - g.lutSkip[idx16, base, ptr] = true
//
// The pattern is cleared (g.lutLoadPat, g.lutSkip rebuilt) at each new block.
// Shared intermediates are admitted only when every use is consumed by
// the selected fused accesses; all other chains keep their original definitions.
func (g *z80cg) scanLUTPatterns(b *Block) {
	// Reset per-block maps (keep same map objects; just clear them).
	clear(g.lutLoadPat)
	clear(g.lutSkip)

	// Build: reg → defining instruction (within this block only).
	defInst := make(map[Reg]*Inst, len(b.Insts))
	for _, inst := range b.Insts {
		if inst.Dst != NoReg {
			defInst[inst.Dst] = inst
		}
	}

	chains := make(map[Reg][3]Reg)
	for _, inst := range b.Insts {
		// Looking for: dst = Load(ptr, u8)
		if inst.Volatile || inst.Op != OpLoad || inst.Ty.Width() != 8 || inst.Dst == NoReg {
			continue
		}
		ptrReg := inst.Src[0]

		// ptr = PtrAdd(base, idx16)
		ptrInst, ok := defInst[ptrReg]
		if !ok || ptrInst.Op != OpPtrAdd {
			continue
		}
		baseReg := ptrInst.Src[0]
		idx16Reg := ptrInst.Src[1]

		// base = AddrOf(sym) where sym is page-aligned
		baseInst, ok := defInst[baseReg]
		if !ok || baseInst.Op != OpAddrOf {
			continue
		}
		sym := baseInst.Sym
		if !g.isGlobalPageAligned(sym) {
			continue
		}

		// idx16 = Ext(src8, u8→u16)
		extInst, ok := defInst[idx16Reg]
		if !ok || extInst.Op != OpExt {
			continue
		}
		if extInst.SrcTy == nil || extInst.SrcTy.Width() != 8 || extInst.Ty.Width() != 16 {
			continue
		}
		src8Reg := extInst.Src[0]

		// Pattern matched: record and mark intermediates for skipping.
		g.lutLoadPat[inst.Dst] = lutLoadPat{sym: sym, src8Reg: src8Reg}
		g.lutSkip[idx16Reg] = true
		g.lutSkip[baseReg] = true
		g.lutSkip[ptrReg] = true
		chains[inst.Dst] = [3]Reg{idx16Reg, baseReg, ptrReg}
	}

	if len(chains) == 0 {
		return
	}

	// A pointer can also feed a later store, a call or a CFG edge. Its
	// definition must survive in that case. Fall back for the entire chain:
	// rebuilding only part of it could overwrite the original byte index
	// before the fused load reads it. Shared indices of split word LUTs
	// remain eligible when all users belong to the fused accesses.
	type user struct {
		block *Block
		inst  *Inst
	}
	users := make(map[Reg][]user)
	for _, block := range g.fn.Blocks {
		for _, inst := range block.Insts {
			for _, r := range inst.Uses() {
				users[r] = append(users[r], user{block, inst})
			}
		}
		if block.Term != nil {
			for _, r := range block.Term.termUses() {
				users[r] = append(users[r], user{block, nil})
			}
		}
	}
	for {
		clear(g.lutSkip)
		for dst, chain := range chains {
			if _, ok := g.lutLoadPat[dst]; ok {
				for _, r := range chain {
					g.lutSkip[r] = true
				}
			}
		}
		var rejected []Reg
		for dst, chain := range chains {
			if _, ok := g.lutLoadPat[dst]; !ok {
				continue
			}
			unsafe := false
			for _, r := range chain {
				for _, use := range users[r] {
					if use.block == b && use.inst != nil {
						_, fusedLoad := g.lutLoadPat[use.inst.Dst]
						if g.lutSkip[use.inst.Dst] || (use.inst.Op == OpLoad && fusedLoad) {
							continue
						}
					}
					unsafe = true
				}
			}
			if unsafe {
				rejected = append(rejected, dst)
			}
		}
		if len(rejected) == 0 {
			break
		}
		for _, dst := range rejected {
			delete(g.lutLoadPat, dst)
		}
	}
}

// scanBitStorePatterns detects the common u8/u16 load-modify-store forms:
//
//	cur  = Load(ptr, u8)
//	next = Or(cur, const(1<<n))
//	Store(ptr, next, u8)
//
//	cur  = Load(ptr, u8)
//	next = And(cur, const(^ (1<<n) & 0xFF))
//	Store(ptr, next, u8)
//
// and explicit MIR2 bit intent:
//
//	cur  = Load(ptr, ty)
//	next = BitSet(cur, .n) / BitReset(cur, .n)
//	Store(ptr, next, ty)
//
// When matched:
//   - g.bitStorePat[nextReg] = {bit:n, op:"SET"/"RES"}
//   - g.bitStoreSkip[cur,next] = true
//
// The optimisation only skips single-use intermediates so it is safe to merge.
func (g *z80cg) scanBitStorePatterns(b *Block) {
	clear(g.bitStorePat)
	clear(g.bitStoreSkip)

	defInst := make(map[Reg]*Inst, len(b.Insts))
	useCount := make(map[Reg]int, len(b.Insts))
	for _, inst := range b.Insts {
		if inst.Dst != NoReg {
			defInst[inst.Dst] = inst
		}
		for _, src := range inst.Src {
			if src != NoReg {
				useCount[src]++
			}
		}
		for _, arg := range inst.Args {
			if arg != NoReg {
				useCount[arg]++
			}
		}
	}
	if b.Term != nil {
		for _, src := range b.Term.termUses() {
			if src != NoReg {
				useCount[src]++
			}
		}
	}

	for _, inst := range b.Insts {
		if inst.Volatile || inst.Op != OpStore || (inst.Ty.Width() != 8 && inst.Ty.Width() != 16) {
			continue
		}
		valReg := inst.Src[1]
		valInst, ok := defInst[valReg]
		if !ok || useCount[valReg] != 1 {
			continue
		}
		if valInst.Op != OpOr && valInst.Op != OpAnd && valInst.Op != OpBitSet && valInst.Op != OpBitReset {
			continue
		}

		var loadReg Reg = NoReg
		var mask byte
		width := inst.Ty.Width()
		for _, src := range valInst.Src {
			if def, ok := defInst[src]; ok && !def.Volatile && def.Op == OpLoad && def.Ty.Width() == width && def.Src[0] == inst.Src[0] {
				loadReg = src
			} else if def, ok := defInst[src]; ok && def.Op == OpConst {
				mask = byte(def.Imm)
			}
		}
		if loadReg == NoReg || useCount[loadReg] != 1 {
			if valInst.Op != OpBitSet && valInst.Op != OpBitReset {
				continue
			}
			loadReg = valInst.Src[0]
			if def, ok := defInst[loadReg]; !ok || def.Volatile || def.Op != OpLoad || def.Ty.Width() != width || def.Src[0] != inst.Src[0] || useCount[loadReg] != 1 {
				continue
			}
		}

		switch width {
		case 8:
			switch valInst.Op {
			case OpOr:
				if bits.OnesCount8(mask) != 1 {
					continue
				}
				bit := bits.TrailingZeros8(mask)
				g.bitStorePat[valReg] = bitStorePat{bit: bit, byteOffset: 0, op: "SET"}
			case OpAnd:
				if bits.OnesCount8(^mask) != 1 {
					continue
				}
				bit := bits.TrailingZeros8(^mask)
				g.bitStorePat[valReg] = bitStorePat{bit: bit, byteOffset: 0, op: "RES"}
			case OpBitSet:
				g.bitStorePat[valReg] = bitStorePat{bit: int(valInst.Imm), byteOffset: 0, op: "SET"}
			case OpBitReset:
				g.bitStorePat[valReg] = bitStorePat{bit: int(valInst.Imm), byteOffset: 0, op: "RES"}
			}
		case 16:
			mask16 := uint16(0)
			for _, src := range valInst.Src {
				if def, ok := defInst[src]; ok && def.Op == OpConst {
					mask16 = uint16(def.Imm)
				}
			}
			switch valInst.Op {
			case OpOr:
				if bits.OnesCount16(mask16) != 1 {
					continue
				}
				totalBit := bits.TrailingZeros16(mask16)
				g.bitStorePat[valReg] = bitStorePat{bit: totalBit % 8, byteOffset: totalBit / 8, op: "SET"}
			case OpAnd:
				if bits.OnesCount16(^mask16) != 1 {
					continue
				}
				totalBit := bits.TrailingZeros16(^mask16)
				g.bitStorePat[valReg] = bitStorePat{bit: totalBit % 8, byteOffset: totalBit / 8, op: "RES"}
			case OpBitSet:
				totalBit := int(valInst.Imm)
				g.bitStorePat[valReg] = bitStorePat{bit: totalBit % 8, byteOffset: totalBit / 8, op: "SET"}
			case OpBitReset:
				totalBit := int(valInst.Imm)
				g.bitStorePat[valReg] = bitStorePat{bit: totalBit % 8, byteOffset: totalBit / 8, op: "RES"}
			}
		}
		if _, ok := g.bitStorePat[valReg]; ok {
			g.bitStoreSkip[loadReg] = true
			g.bitStoreSkip[valReg] = true
		}
	}
}

// scanBitCmpPatterns detects u8/u16 bit-test compares against zero:
//
//	masked = And(x, const(1<<n))
//	cmp    = CmpEq/CmpNe(masked, 0)
//
// and the canonical lowered bitread shape:
//
//	shifted = Shr(x, const(n))
//	masked  = And(shifted, 1)
//	cmp     = CmpEq/CmpNe(masked, 0)
//
// x may be a Load(ptr,u8/u16) or an already-materialized register/pair value.
func (g *z80cg) scanBitCmpPatterns(b *Block) {
	clear(g.bitCmpPat)
	clear(g.bitCmpSkip)

	defInst := make(map[Reg]*Inst, len(b.Insts))
	// Definitions may be used in successor blocks; skipping a load requires
	// a function-wide single-use proof, not merely a count in this block.
	useCount := countRegUses(g.fn)
	for _, inst := range b.Insts {
		if inst.Dst != NoReg {
			defInst[inst.Dst] = inst
		}
	}

	for _, inst := range b.Insts {
		if inst.Op != OpCmp || (inst.Cond != CmpEq && inst.Cond != CmpNe) {
			continue
		}
		testReg, zeroReg := NoReg, NoReg
		if rhsInst, ok := defInst[inst.Src[1]]; ok && rhsInst.Op == OpConst && rhsInst.Imm == 0 {
			testReg = inst.Src[0]
			zeroReg = inst.Src[1]
		} else if lhsInst, ok := defInst[inst.Src[0]]; ok && lhsInst.Op == OpConst && lhsInst.Imm == 0 {
			testReg = inst.Src[1]
			zeroReg = inst.Src[0]
		}
		if testReg == NoReg || useCount[zeroReg] != 1 {
			continue
		}

		testInst, ok := defInst[testReg]
		if !ok || useCount[testReg] != 1 {
			continue
		}

		var (
			pat       bitCmpPat
			skipRegs  []Reg
			matched   bool
			maskConst uint16
			otherReg  Reg = NoReg
		)

		if testInst.Op == OpBitGet {
			totalBit := int(testInst.Imm)
			pat.bit = totalBit % 8
			pat.byteOffset = totalBit / 8
			baseReg := testInst.Src[0]
			if loadInst, ok := defInst[baseReg]; ok && !loadInst.Volatile && loadInst.Op == OpLoad && (loadInst.Ty.Width() == 8 || loadInst.Ty.Width() == 16) && useCount[baseReg] == 1 {
				pat.ptrReg = loadInst.Src[0]
				pat.useMem = true
				skipRegs = []Reg{baseReg, testReg}
				matched = true
			} else {
				pat.srcReg = baseReg
				skipRegs = []Reg{testReg}
				matched = true
			}
		}
		if matched && g.canEmitBitCmpPattern(pat) && bitCmpMemoryAdjacent(b, inst, pat, skipRegs, zeroReg) {
			g.bitCmpPat[inst.Dst] = pat
			g.bitCmpSkip[zeroReg] = true
			for _, r := range skipRegs {
				g.bitCmpSkip[r] = true
			}
			continue
		}

		if testInst.Op != OpAnd {
			continue
		}
		for _, src := range testInst.Src {
			if def, ok := defInst[src]; ok && def.Op == OpConst {
				maskConst = uint16(def.Imm)
			} else {
				otherReg = src
			}
		}

		if maskConst == 1 && otherReg != NoReg {
			shrInst, ok := defInst[otherReg]
			if ok && shrInst.Op == OpShr && useCount[otherReg] == 1 {
				shiftConstInst, ok := defInst[shrInst.Src[1]]
				if ok && shiftConstInst.Op == OpConst {
					totalBit := int(shiftConstInst.Imm)
					pat.bit = totalBit % 8
					pat.byteOffset = totalBit / 8
					baseReg := shrInst.Src[0]
					if loadInst, ok := defInst[baseReg]; ok && !loadInst.Volatile && loadInst.Op == OpLoad && (loadInst.Ty.Width() == 8 || loadInst.Ty.Width() == 16) && useCount[baseReg] == 1 {
						pat.ptrReg = loadInst.Src[0]
						pat.useMem = true
						skipRegs = []Reg{baseReg, otherReg, testReg}
						matched = true
					} else {
						pat.srcReg = baseReg
						skipRegs = []Reg{otherReg, testReg}
						matched = true
					}
				}
			}
		}

		if !matched && bits.OnesCount16(maskConst) == 1 && otherReg != NoReg {
			totalBit := bits.TrailingZeros16(maskConst)
			pat.bit = totalBit % 8
			pat.byteOffset = totalBit / 8
			if loadInst, ok := defInst[otherReg]; ok && !loadInst.Volatile && loadInst.Op == OpLoad && (loadInst.Ty.Width() == 8 || loadInst.Ty.Width() == 16) && useCount[otherReg] == 1 {
				pat.ptrReg = loadInst.Src[0]
				pat.useMem = true
				skipRegs = []Reg{otherReg, testReg}
				matched = true
			} else {
				pat.srcReg = otherReg
				skipRegs = []Reg{testReg}
				matched = true
			}
		}

		if matched && g.canEmitBitCmpPattern(pat) && bitCmpMemoryAdjacent(b, inst, pat, skipRegs, zeroReg) {
			g.bitCmpPat[inst.Dst] = pat
			g.bitCmpSkip[zeroReg] = true
			for _, r := range skipRegs {
				g.bitCmpSkip[r] = true
			}
		}
	}
}

// A memory BIT reads at the compare, so moving a prior load across unrelated
// work can change memory or lose the allocated pointer. Fuse only the exact
// contiguous expression (constants and the definitions being eliminated).
func bitCmpMemoryAdjacent(b *Block, cmp *Inst, pat bitCmpPat, skipped []Reg, zero Reg) bool {
	if !pat.useMem {
		// Deferring the AND/shift to BIT also extends the source register's
		// lifetime past allocation. An intervening instruction may reuse that
		// physical register, even though the original test value was live.
		// Fuse only contiguous expressions for register operands too.
		allowed := map[Reg]bool{zero: true}
		for _, r := range skipped {
			allowed[r] = true
		}
		started := false
		for _, i := range b.Insts {
			if i == cmp {
				return started
			}
			if allowed[i.Dst] {
				started = true
				continue
			}
			if started && i.Op != OpConst {
				return false
			}
		}
		return false
	}
	allowed := map[Reg]bool{zero: true}
	for _, r := range skipped {
		allowed[r] = true
	}
	started := false
	for _, i := range b.Insts {
		if i == cmp {
			return started
		}
		if i.Op == OpLoad && i.Src[0] == pat.ptrReg && allowed[i.Dst] {
			started = true
			continue
		}
		if started && !allowed[i.Dst] && i.Op != OpConst {
			return false
		}
	}
	return false
}

// isGlobalPageAligned reports whether the named global in the current module
// has Align == 256 (i.e. is a page-aligned LUT candidate).
func (g *z80cg) isGlobalPageAligned(sym string) bool {
	for _, gl := range g.mod.Globals {
		if gl.Name == sym {
			if at, ok := gl.Ty.(*ArrayTy); ok {
				return at.Align == 256
			}
		}
	}
	return false
}

// Never skip the original definitions until the selected BIT form can be
// encoded. genCmp's ordinary fallback requires those AND/shift/zero values.
func (g *z80cg) canEmitBitCmpPattern(pat bitCmpPat) bool {
	if pat.useMem {
		ptr := g.loc(pat.ptrReg)
		if isSpill(ptr) {
			ptr = "HL"
		}
		return ptr == "HL" && (pat.byteOffset == 0 || pat.byteOffset == 1) || isIXY(ptr) && pat.byteOffset >= 0 && pat.byteOffset <= 127
	}
	src := selectBitReg(g.loc(pat.srcReg), pat.byteOffset)
	return isSimpleReg(src) && !isSpill(src) && !isIXYReg(src)
}

func (g *z80cg) emitBitMemOp(op string, bit int, ptr string, byteOffset int) bool {
	if ptr == "HL" {
		switch byteOffset {
		case 0:
			g.emitf("    %s %d, (HL)", op, bit)
			return true
		case 1:
			g.emit("    INC HL")
			g.emitf("    %s %d, (HL)", op, bit)
			g.emit("    DEC HL")
			return true
		default:
			return false
		}
	}
	if isIXY(ptr) && byteOffset >= 0 && byteOffset <= 127 {
		g.emitf("    %s %d, %s", op, bit, ptrIndirect(ptr, byteOffset))
		return true
	}
	return false
}

func selectBitReg(src string, byteOffset int) string {
	switch byteOffset {
	case 0:
		return lowByte(src)
	case 1:
		return highByte(src)
	default:
		return ""
	}
}

func (g *z80cg) emitBitRegOp(op string, bit int, reg string) bool {
	if reg == "" || reg == "A" || isSpill(reg) || reg == "F" {
		return false
	}
	if !isSimpleReg(reg) {
		return false
	}
	// BIT/SET/RES use CB prefix encoding — IXH/IXL/IYH/IYL are NOT valid
	// operands (no DD/FD CB r form exists). Return false to fall through
	// to the AND/OR mask path in every caller.
	if isIXYReg(reg) {
		return false
	}
	g.emitf("    %s %d, %s", op, bit, reg)
	return true
}

// globalFieldLabel returns the EQU label for a global struct field access.
// Format: sanitizeIdent(sym)__fieldName  (or just sanitizeIdent(sym) for offset 0 / no name).
func globalFieldLabel(sym string, offset int64, fieldName string) string {
	base := sanitizeIdent(sym)
	if fieldName != "" {
		return base + "__" + fieldName
	}
	if offset == 0 {
		return base
	}
	// No named field at this offset (e.g. nested struct access with folded offsets).
	// Use "sym + N" arithmetic expression — the assembler resolves it inline.
	return fmt.Sprintf("%s + %d", base, offset)
}

// globalStructField looks up a global in the module by symbol name and returns its
// StructTy if it has one, plus the field name for a given byte offset.
// Returns ("", -1, false) when not found or not a struct global.
func (g *z80cg) globalStructField(sym string, offset int64) (fieldName string, found bool) {
	for _, gl := range g.mod.Globals {
		if gl.Name != sym {
			continue
		}
		st, ok := gl.Ty.(*StructTy)
		if !ok {
			return "", false
		}
		// Find field by byte offset.
		for index, f := range st.Fields {
			if int64(st.ByteOffset(index)) == offset {
				return f.Name, true
			}
		}
		// Offset is valid but doesn't align with a named field boundary — still use it.
		return "", false
	}
	return "", false
}

// scanGlobalFieldPatterns detects patterns where a global struct field is loaded
// or stored via AddrOf + optional Field + Load/Store.  When found, the load/store
// is rewritten to use Z80 direct addressing (LD A,(sym__field) or LD (sym__field),A).
//
// Patterns detected (u8 only — all addressable via single LD A,(nn)):
//
//	Load pattern (with field):
//	  base  = AddrOf(sym)            [OpAddrOf]
//	  fptr  = Field(base, imm)       [OpField]
//	  dst   = Load(fptr, u8)         [OpLoad]
//
//	Load pattern (field 0, no OpField):
//	  base  = AddrOf(sym)            [OpAddrOf]
//	  dst   = Load(base, u8)         [OpLoad]
//
//	Store pattern (with field):
//	  base  = AddrOf(sym)
//	  fptr  = Field(base, imm)
//	          Store(fptr, val, u8)   [OpStore, Dst==NoReg]
//
//	Store pattern (field 0):
//	  base  = AddrOf(sym)
//	          Store(base, val, u8)
//
// Only fires when the AddrOf refers to a global in m.Globals (not a func symbol).
// Only u8 loads and stores are handled (single-byte LD A,(nn) / LD (nn),A).
func (g *z80cg) scanGlobalFieldPatterns(b *Block) {
	// Reset per-block maps.
	clear(g.globalFieldLoad)
	clear(g.globalFieldStore)
	clear(g.globalFieldAddr)
	clear(g.globalFieldSkip)
	clear(g.globalU16StoreSym)
	clear(g.globalU16LoadSym)

	// Build: reg → defining instruction within this block.
	defInst := make(map[Reg]*Inst, len(b.Insts))
	for _, inst := range b.Insts {
		if inst.Dst != NoReg {
			defInst[inst.Dst] = inst
		}
	}

	// Helper: check if sym names a module-level global (not a string or function symbol).
	isGlobal := func(sym string) bool {
		for _, gl := range g.mod.Globals {
			if gl.Name == sym {
				return true
			}
		}
		return false
	}

	// Count uses of each virtual reg in this block (instructions + block terminators).
	// Used to guard against skipping intermediates that are shared with other users.
	useCount := make(map[Reg]int)
	for _, inst := range b.Insts {
		for _, s := range inst.Src {
			if s != NoReg {
				useCount[s]++
			}
		}
		for _, s := range inst.Args {
			if s != NoReg {
				useCount[s]++
			}
		}
	}
	if b.Term != nil {
		for _, s := range b.Term.termUses() {
			if s != NoReg {
				useCount[s]++
			}
		}
	}

	// Scan Load instructions.
	// Only mark intermediates as skip when:
	//   1. The load's allocated destination IS A (LD A,(nn) is the only direct read on Z80).
	//   2. Each intermediate register (AddrOf/Field results) is used exactly once —
	//      by this Load (and the Field uses the AddrOf, counted once).
	//      If an intermediate is shared (e.g., base ptr reused for multiple fields),
	//      we must not skip it or the other consumer will see a dead register.
	for _, inst := range b.Insts {
		if inst.Op == OpLoad && inst.Ty.Width() == 8 && inst.Dst != NoReg {
			if g.bitCmpSkip[inst.Dst] {
				continue
			} // BIT (HL) still needs its pointer materialized
			// Only apply when the allocated destination is the A register.
			if g.ar.Loc(inst.Dst).Name != "A" {
				continue
			}
			ptrReg := inst.Src[0]
			ptrInst, hasDef := defInst[ptrReg]
			if !hasDef {
				continue
			}

			switch ptrInst.Op {
			case OpAddrOf:
				// Pattern: Load(AddrOf(sym), u8)  — offset 0
				if !isGlobal(ptrInst.Sym) {
					continue
				}
				// Only skip AddrOf if its result is used only by this Load.
				if useCount[ptrReg] != 1 {
					continue
				}
				fieldName, _ := g.globalStructField(ptrInst.Sym, 0)
				g.globalFieldLoad[inst.Dst] = globalFieldInfo{
					sym:       ptrInst.Sym,
					offset:    0,
					fieldName: fieldName,
				}
				g.globalFieldSkip[ptrReg] = true

			case OpField:
				// Pattern: Load(Field(AddrOf(sym), imm), u8)
				// Guard: Field result used only by this Load, AddrOf result used only by Field.
				if useCount[ptrReg] != 1 {
					continue
				}
				baseReg := ptrInst.Src[0]
				baseDef, ok := defInst[baseReg]
				if !ok || baseDef.Op != OpAddrOf {
					continue
				}
				if !isGlobal(baseDef.Sym) {
					continue
				}
				if useCount[baseReg] != 1 {
					continue
				}
				offset := ptrInst.Imm
				fieldName, _ := g.globalStructField(baseDef.Sym, offset)
				g.globalFieldLoad[inst.Dst] = globalFieldInfo{
					sym:       baseDef.Sym,
					offset:    offset,
					fieldName: fieldName,
				}
				g.globalFieldSkip[ptrReg] = true  // Field result
				g.globalFieldSkip[baseReg] = true // AddrOf result
			}
		}
	}

	// Scan Store instructions (Dst==NoReg, Src[0]=addr, Src[1]=val).
	for _, inst := range b.Insts {
		if inst.Op != OpStore || inst.Ty.Width() != 8 {
			continue
		}
		addrReg := inst.Src[0]
		addrInst, hasDef := defInst[addrReg]
		if !hasDef {
			continue
		}

		switch addrInst.Op {
		case OpAddrOf:
			if !isGlobal(addrInst.Sym) {
				continue
			}
			// Only skip if the AddrOf result is used solely by this Store.
			if useCount[addrReg] != 1 {
				continue
			}
			fieldName, _ := g.globalStructField(addrInst.Sym, 0)
			g.globalFieldStore[addrReg] = globalFieldInfo{
				sym:       addrInst.Sym,
				offset:    0,
				fieldName: fieldName,
			}
			g.globalFieldSkip[addrReg] = true

		case OpField:
			if useCount[addrReg] != 1 {
				continue
			}
			baseReg2 := addrInst.Src[0]
			baseDef, ok := defInst[baseReg2]
			if !ok || baseDef.Op != OpAddrOf {
				continue
			}
			if !isGlobal(baseDef.Sym) {
				continue
			}
			if useCount[baseReg2] != 1 {
				continue
			}
			offset := addrInst.Imm
			fieldName, _ := g.globalStructField(baseDef.Sym, offset)
			g.globalFieldStore[addrReg] = globalFieldInfo{
				sym:       baseDef.Sym,
				offset:    offset,
				fieldName: fieldName,
			}
			g.globalFieldSkip[addrReg] = true  // Field result
			g.globalFieldSkip[baseReg2] = true // AddrOf result
		}
	}

	// A remaining literal global field address can be loaded directly into its
	// allocated pair, even when the later byte load targets C/D/etc rather than
	// A. Do not materialize a base plus scratch offset for LD HL,sym+offset.
	// Whole-function use counts also protect an SSA base used by another block.
	allUses := countRegUses(g.fn)
	for _, inst := range b.Insts {
		if inst.Op != OpField || inst.Imm < 0 || inst.Imm > 65535 || g.globalFieldSkip[inst.Dst] || !isPairReg(g.ar.Loc(inst.Dst).Name) {
			continue
		}
		base := inst.Src[0]
		definition := defInst[base]
		if definition == nil || definition.Op != OpAddrOf || !isGlobal(definition.Sym) || allUses[base] != 1 {
			continue
		}
		g.globalFieldAddr[inst.Dst] = globalFieldInfo{sym: definition.Sym, offset: inst.Imm}
		g.globalFieldSkip[base] = true
	}

	// Literal global displacements may feed a direct word load as well.
	// Both the base and derived address must have one use across the entire
	// function; otherwise skipping their materialization would lose a live pointer.
	for _, inst := range b.Insts {
		if inst.Op != OpLoad || inst.Ty.Width() != 16 || inst.Dst == NoReg || g.bitCmpSkip[inst.Dst] {
			continue
		}
		dst := g.ar.Loc(inst.Dst).Name
		if dst != "HL" && dst != "DE" && dst != "BC" {
			continue
		}
		addr := inst.Src[0]
		if _, literal := g.globalFieldAddr[addr]; literal && allUses[addr] == 1 {
			g.globalFieldSkip[addr] = true
		}
	}

	// ── HL-chain detection ──────────────────────────────────────────────────────
	// When 2+ adjacent stores cover consecutive byte offsets in source order and
	// HL is free, with all values outside H/L,
	// replace independent LD (sym__field),A sequences with:
	//   LD HL,sym  ;  LD (HL),r  ;  INC HL  ;  LD (HL),r  ;  ...
	// Saves 8T and 4 bytes for a 3-field struct (61T→53T, 12B→8B).
	// The shared pointer is implicit, so MIR liveness cannot keep it safe
	// across an intervening ALU scratch save, call, load, or other store.
	// Only the address definitions already skipped above may separate stores.
	var chain []*Inst
	flushChain := func() {
		defer func() { chain = nil }()
		if len(chain) < 2 {
			return
		}
		// Loading the head must not destroy an unrelated live HL value.
		// Skipped address vregs never occupy their allocated register.
		for r := range g.regsLiveAfterInst(chain[0]) {
			if g.globalFieldSkip[r] {
				continue
			}
			loc := g.ar.Loc(r).Name
			if loc == "HL" || loc == "H" || loc == "L" {
				return
			}
		}
		for i, store := range chain {
			entry := g.globalFieldStore[store.Src[0]]
			entry.hlChain = true
			entry.hlHead = i == 0
			entry.hlLast = i == len(chain)-1
			g.globalFieldStore[store.Src[0]] = entry
		}
	}
	for _, inst := range b.Insts {
		if inst.Dst != NoReg && g.globalFieldSkip[inst.Dst] && (inst.Op == OpAddrOf || inst.Op == OpField) {
			continue
		}
		info, eligible := g.globalFieldStore[inst.Src[0]]
		value := g.ar.Loc(inst.Src[1]).Name
		if inst.Op != OpStore || inst.Ty.Width() != 8 || !eligible || value == "H" || value == "L" || value == "HL" {
			flushChain()
			continue
		}
		if len(chain) > 0 {
			previous := g.globalFieldStore[chain[len(chain)-1].Src[0]]
			if previous.sym != info.sym || info.offset != previous.offset+1 {
				flushChain()
			}
		}
		chain = append(chain, inst)
	}
	flushChain()

	// ── u16 direct-address store detection ──────────────────────────────────────
	// Pattern: OpStore(w=16, addrReg=AddrOf(sym), valReg)
	// Enables LD (sym), HL (16T, no A clobber) when valReg is in HL.
	for _, inst := range b.Insts {
		if inst.Op != OpStore || inst.Ty.Width() != 16 {
			continue
		}
		addrReg := inst.Src[0]
		def, ok := defInst[addrReg]
		if !ok || def.Op != OpAddrOf || !isGlobal(def.Sym) {
			continue
		}
		g.globalU16StoreSym[addrReg] = def.Sym
	}

	// ── u16 direct-address load detection ────────────────────────────────────
	// Pattern: OpLoad(w=16, addrReg=AddrOf(sym)) → dst (u16, must be HL/DE/BC)
	// Enables direct LD rr, (sym) without scratch-register clobbers.
	// Only fires when addrReg is used solely by this load (useCount==1) so we
	// can safely skip the AddrOf instruction (suppress LD HL,sym).
	for _, inst := range b.Insts {
		if inst.Op != OpLoad || inst.Ty.Width() != 16 || inst.Dst == NoReg || g.bitCmpSkip[inst.Dst] {
			continue
		}
		dst := g.ar.Loc(inst.Dst).Name
		if dst != "HL" && dst != "DE" && dst != "BC" {
			continue
		}
		addrReg := inst.Src[0]
		def, ok := defInst[addrReg]
		if !ok || def.Op != OpAddrOf || !isGlobal(def.Sym) {
			continue
		}
		if useCount[addrReg] != 1 {
			continue // addr reg used by other instructions too
		}
		// The DE/BC extension requires the address to have one use across
		// the whole function, including direct successor and terminator uses.
		if dst != "HL" {
			if g.fn == nil || countRegUses(g.fn)[addrReg] != 1 {
				continue
			}
			storage := false
			for _, global := range g.mod.Globals {
				if global.Name == def.Sym && ByteWidth(global.Ty) >= 2 {
					storage = true
				}
			}
			if !storage {
				continue
			}
		}
		g.globalU16LoadSym[addrReg] = def.Sym
		g.globalFieldSkip[addrReg] = true // suppress LD HL, sym from OpAddrOf
	}
}

// scanPtrAddBase detects OpPtrAdd instructions where the base pointer is in HL
// AND the base virtual is live after the ptr_add (e.g. needed on a loop
// back-edge).  In that case ADD HL,rr would clobber HL (the base), so we
// arrange for PUSH HL before the ADD and POP HL after the subsequent load.
//
// Sets g.ptrAddPushHL = ptr_add.Dst  when the pattern fires; cleared by the
// OpLoad/OpStore codegen after it emits the POP HL.
func (g *z80cg) scanPtrAddBase(b *Block) {
	g.ptrAddPushHL = NoReg

	uses := countRegUses(g.fn)
	for idx, inst := range b.Insts {
		if inst.Op != OpPtrAdd || idx+1 >= len(b.Insts) {
			continue
		}
		consumer := b.Insts[idx+1]
		// A delayed save must have an immediate, single-use consumer that
		// restores the stack before any subsequent pointer calculation.
		if (consumer.Op != OpLoad && consumer.Op != OpStore) || consumer.Src[0] != inst.Dst || uses[inst.Dst] != 1 {
			continue
		}
		baseReg := inst.Src[0]
		if g.ar.Loc(baseReg).Name != "HL" {
			continue // base not in HL — no clobber risk
		}

		// Check if baseReg is live after this ptr_add.
		// A base is live-after if it appears in any later instruction's sources
		// OR in the block terminator's argument list.
		liveAfter := false
		for _, later := range b.Insts[idx+1:] {
			for _, s := range later.Src {
				if s == baseReg {
					liveAfter = true
					break
				}
			}
			for _, a := range later.Args {
				if a == baseReg {
					liveAfter = true
					break
				}
			}
			if liveAfter {
				break
			}
		}
		if !liveAfter {
			if t := b.Term; t != nil {
				for _, r := range t.termUses() {
					if r == baseReg {
						liveAfter = true
						break
					}
				}
			}
		}

		if liveAfter {
			g.ptrAddPushHL = inst.Dst
			return // one pattern per block is enough
		}
	}
}
