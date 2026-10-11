// outparam_promote.go — Detect write-only output parameters and promote to tuple returns.
//
// C pattern:
//   void get_minmax(uint8_t *buf, uint8_t n, uint8_t *out_min, uint8_t *out_max);
//
// If *out_min and *out_max are only written to (never read), the true contract is:
//   (min: u8, max: u8) = get_minmax(buf: ^u8, n: u8)
//
// This eliminates pointer indirection entirely — PFCCO assigns min/max to registers
// and the caller reads them directly, no memory access needed.
//
// Detection: at MIR2 level, scan function body for pointer parameter usage:
//   - If param P is only used as OpStore dst (written through) and never OpLoad src
//     (never read through), it's a write-only output parameter.
//   - Promote: remove pointer param, add return value.
//
// This is ADR-0022 extended: struct-return promotion handles the return side,
// outparam promotion handles the parameter side. Together they cover all C patterns
// for multi-value returns.
package lir

import (
	"github.com/minz/minzc/pkg/mir2"
)

// OutparamInfo describes a detected write-only output parameter.
type OutparamInfo struct {
	ParamIdx int      // index in function params
	ParamReg mir2.Reg // virtual register of the pointer param
	StoredVRegs []mir2.Reg // vregs stored through this pointer (the output values)
}

// DetectOutparams analyzes a MIR2 function for write-only pointer parameters.
// Returns list of outparam candidates suitable for tuple promotion.
func DetectOutparams(f *mir2.Func) []OutparamInfo {
	if f == nil || len(f.Contract.Params) == 0 {
		return nil
	}

	var results []OutparamInfo

	for i, param := range f.Contract.Params {
		// Only consider pointer-class params (HL, DE, IX, IY).
		if param.Class != mir2.ClassPointer && param.Class != mir2.ClassIndex {
			continue
		}
		reg := param.Reg

		// Scan all blocks for usage of this param.
		readThrough := false   // does any instruction load through this pointer?
		writeThrough := false  // does any instruction store through this pointer?
		usedDirectly := false  // is the pointer value itself used (not dereferenced)?
		var storedVRegs []mir2.Reg

		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				// OpStore: src[0] is pointer, src[1] is value
				if inst.Op == mir2.OpStore && inst.Src[0] == reg {
					writeThrough = true
					if inst.Src[1] != mir2.NoReg {
						storedVRegs = append(storedVRegs, inst.Src[1])
					}
					continue
				}

				// OpLoad: src[0] is pointer
				if inst.Op == mir2.OpLoad && inst.Src[0] == reg {
					readThrough = true
					continue
				}

				// Any other use of the pointer value itself
				for s := 0; s < 2; s++ {
					if inst.Src[s] == reg && inst.Op != mir2.OpStore {
						usedDirectly = true
					}
				}
			}
		}

		// Write-only: written through, never read through, never used directly
		if writeThrough && !readThrough && !usedDirectly && len(storedVRegs) > 0 {
			results = append(results, OutparamInfo{
				ParamIdx:    i,
				ParamReg:    reg,
				StoredVRegs: storedVRegs,
			})
		}
	}

	return results
}

// OutparamReport generates a human-readable report of detected outparams.
func OutparamReport(f *mir2.Func, outparams []OutparamInfo) string {
	if len(outparams) == 0 {
		return ""
	}
	report := f.Name + ": detected write-only output params:\n"
	for _, op := range outparams {
		report += "  param " + string(rune('0'+op.ParamIdx)) + " (vreg " +
			string(rune('0'+int(op.ParamReg))) + "): write-only → tuple return candidate\n"
	}
	return report
}
