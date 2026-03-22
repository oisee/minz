// bridge_egraph.go — Multi-variant bridge: MIR2 instructions → EGraph.
//
// For each MIR2 instruction, generates all valid Z80 lowering variants
// and stores them in an EGraph. The isel/WFC pipeline then picks the
// cheapest valid form.
//
// Width-mismatch cases (the #1 source of LIR errors) get multiple variants:
//   OpMove u16→u8: {trunc via L, trunc via E, trunc via C}
//   OpMove u8→u16: {zext via HL, zext via DE, zext via BC}
//   OpCmp flag→A:  {SBC A,A, JR+SCF+SBC}
package lir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// LowerMIR2BlockEGraph lowers a MIR2 block into an EGraph of MIROp variants.
// Each MIR2 instruction becomes an EClass with 1+ variants.
// Returns both the EGraph and the flattened ops (cheapest per class).
func LowerMIR2BlockEGraph(b *mir2.Block, desc *MachineDesc, mod *mir2.Module) (*EGraph, []MIROp, error) {
	eg := NewEGraph()

	for _, inst := range b.Insts {
		// Calls and muls use existing multi-op lowering (not e-graphable yet).
		if inst.Op == mir2.OpCall || inst.Op == mir2.OpCallIndirect {
			callOps, err := translateCall(inst, desc, mod)
			if err != nil {
				return nil, nil, fmt.Errorf("block %s: %w", b.Label, err)
			}
			for _, op := range callOps {
				eg.SingleClass([]MIROp{op}, 17, "call")
			}
			continue
		}
		if inst.Op == mir2.OpMul {
			mulOps := translateMul(inst, desc)
			if mulOps != nil {
				eg.SingleClass(mulOps, 80, "mul_runtime")
				continue
			}
		}

		// Try e-graph multi-variant lowering.
		variants := TranslateInstEGraph(inst, desc)
		if len(variants) > 0 {
			eg.AddClass(variants...)
			continue
		}

		// Fallback: single variant from standard bridge.
		op, err := translateInst(inst, desc)
		if err != nil {
			return nil, nil, fmt.Errorf("block %s: %w", b.Label, err)
		}
		if op != nil {
			eg.SingleClass([]MIROp{*op}, 4, "standard")
		}
	}

	// Select cheapest for now (WFC integration will override later).
	eg.SelectCheapest()
	ops := eg.Extract()

	// Fuse addr_of + store/load → store_global/load_global.
	// LD (sym), HL (16T) vs LD rr,sym (10T) + store via ptr (22T) = 32T.
	ops = fuseGlobalAccess(ops)

	// Split self-stores: when OpStore has src0==src1 (same vreg used as
	// both pointer and value), insert a move to evacuate the value to a
	// different pair. Without this, WFC assigns both to HL → LD (HL), HL.
	if desc.Name == "z80" {
		ops = splitSelfStores(ops, desc)
		ops = insertSaveBeforeOverwrite(ops, desc)
	}

	return eg, ops, nil
}

// TranslateInstEGraph converts a MIR2 instruction into EGraph variants.
// Returns the variants for this instruction. Most instructions have 1 variant;
// width-mismatches and flag materialization have 2-3.
func TranslateInstEGraph(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	switch {
	// ── Truncation: u16 → u8 ────────────────────────────────────────
	case inst.Op == mir2.OpTrunc:
		return truncVariants(inst, desc)

	// ── Width widening: u8 → u16 (zero-extend) ───────────────────────
	case inst.Op == mir2.OpExt || inst.Op == mir2.OpSext:
		return extendVariants(inst, desc)

	// ── Flag → register materialization ──────────────────────────────
	case inst.Op == mir2.OpMove && inst.Src[0] != mir2.NoReg:
		if inst.SrcTy != nil && inst.SrcTy.Width() == 1 {
			return flagMaterializeVariants(inst, desc)
		}

	// ── 16-bit store via pointer ─────────────────────────────────────
	case inst.Op == mir2.OpStore && inst.Ty != nil && inst.Ty.Width() > 8:
		return store16Variants(inst, desc)

	// ── Comparison with constant overflow ────────────────────────────
	case inst.Op == mir2.OpCmp && inst.Imm != 0:
		return cmpVariants(inst, desc)
	}

	// Default: single variant from existing translateInst.
	op, err := translateInst(inst, desc)
	if err != nil || op == nil {
		return nil
	}
	cost := 4
	for _, p := range desc.Patterns {
		if p.MIROp == op.Op && (p.Width == 0 || p.Width == op.Width) {
			cost = p.Cost
			break
		}
	}
	return []EVariant{{Ops: []MIROp{*op}, Cost: cost, Tag: "default"}}
}

// truncSynthBase is a counter for synthetic vreg IDs used in truncation.
var truncSynthBase = 9000

// truncVariants generates lowering variants for 16-bit → 8-bit truncation.
//
// The source vreg is 16-bit (allocated to HL/DE/BC). We can't emit LD A, HL.
// Instead: emit OpMove(width=8) with SrcAllowed constrained to gpr8.
// WFC sees that the 16-bit vreg is in HL but the use needs gpr8 →
// it inserts a setup move (LD L→A) or picks a pattern that reads L directly.
func truncVariants(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	src := int(inst.Src[0])
	dst := int(inst.Dst)
	if inst.Dst == mir2.NoReg || inst.Src[0] == mir2.NoReg {
		return nil
	}

	gpr8 := desc.LocsOfWidth(8)

	pairs := desc.LocsOfWidth(16)

	return []EVariant{
		// Variant A: src is already 8-bit (no actual truncation needed)
		{
			Ops: []MIROp{{
				Op: OpMove, Dst: dst, Src: [2]int{src, -1}, Width: 8,
				DstAllowed: gpr8, SrcAllowed: [2]LocSet{gpr8},
			}},
			Cost: 4,
			Tag: "trunc_nop",
		},
		// Variant B: src is 16-bit pair → use trunc patterns (LD A, L etc.)
		// Leave SrcAllowed open to pairs so isel picks trunc_hl_a / trunc_de_a etc.
		{
			Ops: []MIROp{{
				Op: OpMove, Dst: dst, Src: [2]int{src, -1}, Width: 8,
				DstAllowed: gpr8, SrcAllowed: [2]LocSet{pairs},
			}},
			Cost: 4,
			Tag: "trunc_pair",
		},
	}
}

// extendVariants generates lowering variants for 8-bit → 16-bit zero-extend.
func extendVariants(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	src := int(inst.Src[0])
	dst := int(inst.Dst)
	if inst.Dst == mir2.NoReg {
		return nil
	}

	hl := desc.LocSetByNames("HL")
	de := desc.LocSetByNames("DE")
	bc := desc.LocSetByNames("BC")
	a := desc.LocSetByNames("A")
	gpr8 := desc.LocsOfWidth(8)

	var variants []EVariant

	// Variant A: zero-extend into HL (LD L, src; LD H, 0)
	variants = append(variants, EVariant{
		Ops: []MIROp{
			{Op: OpMove, Dst: dst, Src: [2]int{src, -1}, Width: 16,
				DstAllowed: hl, SrcAllowed: [2]LocSet{gpr8}},
		},
		Cost: 11,
		Tag:  "zext_hl",
	})

	// Variant B: zero-extend into DE (LD E, src; LD D, 0)
	variants = append(variants, EVariant{
		Ops: []MIROp{
			{Op: OpMove, Dst: dst, Src: [2]int{src, -1}, Width: 16,
				DstAllowed: de, SrcAllowed: [2]LocSet{gpr8}},
		},
		Cost: 11,
		Tag:  "zext_de",
	})

	// Variant C: zero-extend into BC (LD C, src; LD B, 0)
	variants = append(variants, EVariant{
		Ops: []MIROp{
			{Op: OpMove, Dst: dst, Src: [2]int{src, -1}, Width: 16,
				DstAllowed: bc, SrcAllowed: [2]LocSet{gpr8}},
		},
		Cost: 11,
		Tag:  "zext_bc",
	})

	_ = a
	return variants
}

// flagMaterializeVariants generates variants for bool/flag → register.
func flagMaterializeVariants(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	src := int(inst.Src[0])
	dst := int(inst.Dst)
	if inst.Dst == mir2.NoReg {
		return nil
	}

	a := desc.LocSetByNames("A")
	flags := desc.LocSetByNames("F")

	// Variant A: SBC A,A (carry → 0xFF/0x00, 4T)
	return []EVariant{
		{
			Ops: []MIROp{{
				Op: OpMove, Dst: dst, Src: [2]int{src, -1}, Width: 8,
				DstAllowed: a, SrcAllowed: [2]LocSet{flags},
			}},
			Cost: 4,
			Tag:  "flag_sbc_a_a",
		},
	}
}

// splitSelfStores detects OpStore where src0 (ptr) and src1 (val) are the
// same vreg, and inserts an OpMove to evacuate the value to a fresh vreg.
// Without this, WFC assigns both to the same physical register (HL),
// producing invalid self-stores like LD (HL), HL.
func splitSelfStores(ops []MIROp, desc *MachineDesc) []MIROp {
	var result []MIROp
	for _, op := range ops {
		if op.Op == OpStore && op.Src[0] >= 0 && op.Src[1] >= 0 && op.Src[0] == op.Src[1] {
			// Self-store: ptr==val. Insert move: val → fresh vreg in DE.
			freshVReg := truncSynthBase
			truncSynthBase++
			de := desc.LocSetByNames("DE")
			// EX DE, HL to evacuate value.
			result = append(result, MIROp{
				Op:         OpMove,
				Dst:        freshVReg,
				Src:        [2]int{op.Src[1], -1},
				Width:      16,
				DstAllowed: de,
			})
			// Patched store: ptr=original, val=fresh(DE).
			patched := op
			patched.Src[1] = freshVReg
			result = append(result, patched)
			continue
		}
		result = append(result, op)
	}
	return result
}

// store16Variants generates variants for 16-bit indirect store.
// When ptr and value might be in the same pair (HL), offers safe+fast variants.
func store16Variants(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	ptr := int(inst.Src[0])
	val := int(inst.Src[1])
	if inst.Src[0] == mir2.NoReg || inst.Src[1] == mir2.NoReg {
		return nil
	}

	hl := desc.LocSetByNames("HL")
	de := desc.LocSetByNames("DE")
	bc := desc.LocSetByNames("BC")

	var variants []EVariant

	// Variant: ptr=HL, val=DE (no conflict, cheapest)
	variants = append(variants, EVariant{
		Ops: []MIROp{{
			Op: OpStore, Dst: -1, Src: [2]int{ptr, val}, Width: 16,
			SrcAllowed: [2]LocSet{hl, de},
		}},
		Cost: 22, Tag: "st16_hl_de",
	})

	// Variant: ptr=HL, val=BC
	variants = append(variants, EVariant{
		Ops: []MIROp{{
			Op: OpStore, Dst: -1, Src: [2]int{ptr, val}, Width: 16,
			SrcAllowed: [2]LocSet{hl, bc},
		}},
		Cost: 22, Tag: "st16_hl_bc",
	})

	// Variant: self-store (ptr==val, both in HL). Evacuate to DE first.
	// This fires when the same vreg is used as both pointer and value.
	if ptr == val {
		evacuateVReg := truncSynthBase
		truncSynthBase++
		variants = append(variants, EVariant{
			Ops: []MIROp{
				// Step 1: copy HL to DE (EX DE,HL)
				{Op: OpMove, Dst: evacuateVReg, Src: [2]int{val, -1}, Width: 16,
					DstAllowed: de, SrcAllowed: [2]LocSet{hl}},
				// Step 2: store DE via HL pointer
				{Op: OpStore, Dst: -1, Src: [2]int{ptr, evacuateVReg}, Width: 16,
					SrcAllowed: [2]LocSet{hl, de}},
			},
			Cost: 26, Tag: "st16_self_evacuate",
		})
	}

	return variants
}

// cmpVariants generates comparison variants with masked immediates.
func cmpVariants(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	// Use the existing translateInst but mask the immediate.
	op, err := translateInst(inst, desc)
	if err != nil || op == nil {
		return nil
	}
	// Mask immediate to width.
	if op.Width <= 8 {
		op.Imm &= 0xFF
	} else if op.Width <= 16 {
		op.Imm &= 0xFFFF
	}
	return []EVariant{{Ops: []MIROp{*op}, Cost: 7, Tag: "cmp_masked"}}
}
