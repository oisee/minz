// z3validate.go — Z3-based validation and repair of WFC allocations.
//
// After WFC Collapse, some assignments may violate constraints:
//   - Same vreg assigned to different physical locations in different uses
//   - Pattern requires specific register but WFC picked something else
//   - validate-reject loop produces worse code than necessary
//
// Z3Validate checks the WFC result and, if invalid, re-solves with Z3
// to find a valid allocation. This replaces the hacky validate-reject loop
// with a provably correct constraint solver.
package lir

import (
	"fmt"
	"os/exec"
)

// hasZ3 checks if Z3 binary is available.
func hasZ3() bool {
	_, err := exec.LookPath(Z3Path)
	return err == nil
}

// Z3ValidateAndRepair checks WFC allocation for consistency and fixes
// any violations using Z3. Returns true if repair was needed.
func Z3ValidateAndRepair(wfc *WFCState, desc *MachineDesc, hints map[int]int) bool {
	// Check 1: vreg consistency — same vreg must have same phys everywhere.
	vregPhys := make(map[int]int) // vreg → first seen phys
	inconsistent := false

	for _, c := range wfc.Cells {
		if c.VRegDst >= 0 {
			phys := PhysOf(c.DstLocs)
			if phys >= 0 {
				if prev, ok := vregPhys[c.VRegDst]; ok && prev != phys {
					inconsistent = true
					break
				}
				vregPhys[c.VRegDst] = phys
			}
		}
		for s := 0; s < 2; s++ {
			v := c.VRegSrc[s]
			if v < 0 {
				continue
			}
			phys := PhysOf(c.SrcLocs[s])
			if phys >= 0 {
				if prev, ok := vregPhys[v]; ok && prev != phys {
					inconsistent = true
					break
				}
				vregPhys[v] = phys
			}
		}
		if inconsistent {
			break
		}
	}

	if !inconsistent {
		return false // WFC allocation is consistent, no repair needed
	}

	// Build LIR Prog from WFC state for Z3.
	prog := &Prog{
		Name: "z3repair",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: wfc.ToInsts(),
		}},
	}

	z3Result, err := SolveOptimalWithHints(prog, hints)
	if err != nil {
		// Z3 failed — can't repair, WFC result stands.
		fmt.Printf("[z3-validate] repair failed: %v\n", err)
		return false
	}

	// Apply Z3 assignments to WFC cells.
	for ci := range wfc.Cells {
		c := &wfc.Cells[ci]
		if c.VRegDst >= 0 {
			if phys, ok := z3Result.VRegPhys[c.VRegDst]; ok {
				c.DstLocs = Singleton(phys)
			}
		}
		for s := 0; s < 2; s++ {
			v := c.VRegSrc[s]
			if v >= 0 {
				if phys, ok := z3Result.VRegPhys[v]; ok {
					c.SrcLocs[s] = Singleton(phys)
				}
			}
		}
	}

	return true
}
