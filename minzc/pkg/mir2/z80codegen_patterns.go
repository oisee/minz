package mir2

import (
	"fmt"
	"math/bits"
	"sort"
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
	// HL-chain: when true, emit LD HL,sym chain instead of LD (sym__field),A.
	hlChain bool // part of a consecutive-field HL-chain group
	hlHead  bool // first in group → emit LD HL,sym before
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
// This is safe because LUTGen only ever creates single-use intermediate regs.
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

	for _, inst := range b.Insts {
		// Looking for: dst = Load(ptr, u8)
		if inst.Op != OpLoad || inst.Ty.Width() != 8 || inst.Dst == NoReg {
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
		if inst.Op != OpStore || (inst.Ty.Width() != 8 && inst.Ty.Width() != 16) {
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
			if def, ok := defInst[src]; ok && def.Op == OpLoad && def.Ty.Width() == width && def.Src[0] == inst.Src[0] {
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
			if def, ok := defInst[loadReg]; !ok || def.Op != OpLoad || def.Ty.Width() != width || def.Src[0] != inst.Src[0] || useCount[loadReg] != 1 {
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
			if loadInst, ok := defInst[baseReg]; ok && loadInst.Op == OpLoad && (loadInst.Ty.Width() == 8 || loadInst.Ty.Width() == 16) && useCount[baseReg] == 1 {
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
		if matched {
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
					if loadInst, ok := defInst[baseReg]; ok && loadInst.Op == OpLoad && (loadInst.Ty.Width() == 8 || loadInst.Ty.Width() == 16) && useCount[baseReg] == 1 {
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
			if loadInst, ok := defInst[otherReg]; ok && loadInst.Op == OpLoad && (loadInst.Ty.Width() == 8 || loadInst.Ty.Width() == 16) && useCount[otherReg] == 1 {
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

		if matched {
			g.bitCmpPat[inst.Dst] = pat
			g.bitCmpSkip[zeroReg] = true
			for _, r := range skipRegs {
				g.bitCmpSkip[r] = true
			}
		}
	}
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
		off := int64(0)
		for _, f := range st.Fields {
			if off == offset {
				return f.Name, true
			}
			off += int64(ByteWidth(f.Ty))
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

	// ── HL-chain detection ──────────────────────────────────────────────────────
	// When 2+ stores to the same global struct cover consecutive byte offsets and
	// all value registers are safe (not H or L, which alias with the HL pointer),
	// replace independent LD (sym__field),A sequences with:
	//   LD HL,sym  ;  LD (HL),r  ;  INC HL  ;  LD (HL),r  ;  ...
	// Saves 8T and 4 bytes for a 3-field struct (61T→53T, 12B→8B).
	type hlEntry struct {
		addrReg Reg
		valReg  Reg
		offset  int64
	}
	symStores := make(map[string][]hlEntry)
	for addrReg, info := range g.globalFieldStore {
		for _, inst := range b.Insts {
			if inst.Op == OpStore && inst.Src[0] == addrReg {
				symStores[info.sym] = append(symStores[info.sym], hlEntry{
					addrReg: addrReg,
					valReg:  inst.Src[1],
					offset:  info.offset,
				})
				break
			}
		}
	}
	for _, entries := range symStores {
		if len(entries) < 2 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].offset < entries[j].offset })
		// Check consecutive offsets.
		consecutive := true
		for i := 1; i < len(entries); i++ {
			if entries[i].offset != entries[i-1].offset+1 {
				consecutive = false
				break
			}
		}
		if !consecutive {
			continue
		}
		// Check all value regs safe (not H/L which aliases with HL pointer).
		safe := true
		for _, e := range entries {
			if e.valReg != NoReg {
				loc := g.ar.Loc(e.valReg).Name
				if loc == "H" || loc == "L" {
					safe = false
					break
				}
			}
		}
		if !safe {
			continue
		}
		// Mark as HL-chain.
		for i, e := range entries {
			entry := g.globalFieldStore[e.addrReg]
			entry.hlChain = true
			entry.hlHead = (i == 0)
			entry.hlLast = (i == len(entries)-1)
			g.globalFieldStore[e.addrReg] = entry
		}
	}

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
	// Pattern: OpLoad(w=16, addrReg=AddrOf(sym)) → dst (u16, must be HL)
	// Enables LD HL, (sym) (20T, no A clobber) instead of INC/DEC trick via A.
	// Only fires when addrReg is used solely by this load (useCount==1) so we
	// can safely skip the AddrOf instruction (suppress LD HL,sym).
	for _, inst := range b.Insts {
		if inst.Op != OpLoad || inst.Ty.Width() != 16 || inst.Dst == NoReg {
			continue
		}
		if g.ar.Loc(inst.Dst).Name != "HL" {
			continue // only LD HL,(nn) exists for direct u16 load
		}
		addrReg := inst.Src[0]
		def, ok := defInst[addrReg]
		if !ok || def.Op != OpAddrOf || !isGlobal(def.Sym) {
			continue
		}
		if useCount[addrReg] != 1 {
			continue // addr reg used by other instructions too
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
// OpLoad codegen after it emits the POP HL.
func (g *z80cg) scanPtrAddBase(b *Block) {
	g.ptrAddPushHL = NoReg

	for idx, inst := range b.Insts {
		if inst.Op != OpPtrAdd {
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
