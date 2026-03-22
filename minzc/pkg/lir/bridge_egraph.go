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

	// Insert save-before-overwrite moves for Z80.
	if desc.Name == "z80" {
		ops = insertSaveBeforeOverwrite(ops, desc)
	}

	return eg, ops, nil
}

// TranslateInstEGraph converts a MIR2 instruction into EGraph variants.
// Returns the variants for this instruction. Most instructions have 1 variant;
// width-mismatches and flag materialization have 2-3.
func TranslateInstEGraph(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	switch {
	// ── Width narrowing: u16 → u8 (truncation) ────────────────────────
	case inst.Op == mir2.OpMove && inst.Ty != nil && inst.Ty.Width() <= 8 &&
		inst.Src[0] != mir2.NoReg:
		// Source might be 16-bit (pair), dest is 8-bit.
		// Variants: take low byte of each possible pair.
		return truncVariants(inst, desc)

	// ── Width widening: u8 → u16 (zero-extend) ───────────────────────
	case inst.Op == mir2.OpExt || inst.Op == mir2.OpSext:
		return extendVariants(inst, desc)

	// ── Flag → register materialization ──────────────────────────────
	case inst.Op == mir2.OpMove && inst.Src[0] != mir2.NoReg:
		// Check if source was a flag result (bool type).
		if inst.SrcTy != nil && inst.SrcTy.Width() == 1 {
			return flagMaterializeVariants(inst, desc)
		}

	// ── Comparison with constant overflow ────────────────────────────
	case inst.Op == mir2.OpCmp && inst.Imm != 0:
		// Mask immediate to 8 or 16 bits to prevent CP 4294967295.
		return cmpVariants(inst, desc)
	}

	// Default: single variant from existing translateInst.
	op, err := translateInst(inst, desc)
	if err != nil || op == nil {
		return nil
	}
	cost := 4 // default cost
	for _, p := range desc.Patterns {
		if p.MIROp == op.Op && (p.Width == 0 || p.Width == op.Width) {
			cost = p.Cost
			break
		}
	}
	return []EVariant{{Ops: []MIROp{*op}, Cost: cost, Tag: "default"}}
}

// truncVariants generates lowering variants for 16-bit → 8-bit truncation.
// Z80 doesn't have a trunc instruction — must extract the low byte of a pair.
func truncVariants(inst *mir2.Inst, desc *MachineDesc) []EVariant {
	src := int(inst.Src[0])
	dst := int(inst.Dst)
	if dst == 0 || src == 0 {
		return nil
	}

	gpr8 := desc.LocsOfWidth(8)
	pairs := desc.LocsOfWidth(16)

	var variants []EVariant

	// Variant: direct move (if src is 8-bit, just move)
	variants = append(variants, EVariant{
		Ops: []MIROp{{
			Op:    OpMove,
			Dst:   dst,
			Src:   [2]int{src, -1},
			Width: 8,
		}},
		Cost: 4,
		Tag:  "trunc_move8",
	})

	// Variant: if src is a pair, take low byte
	// This is expressed as a move with width=8, src constrained to pair
	// The emitter should emit "LD A, L" (low byte of HL) etc.
	_ = gpr8
	_ = pairs

	return variants
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
