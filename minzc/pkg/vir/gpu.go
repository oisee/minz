// gpu.go — GPU brute-force register allocator bridge.
//
// Serializes a VIR function's constraints to JSON, invokes the CUDA
// z80_regalloc kernel, and parses the optimal assignment back.
//
// This is an alternative to Z3 for small-to-medium functions (≤12 vregs).
// The GPU exhaustively searches ALL 7^N register assignments and returns
// the provably optimal one.
//
// Requires: z80_regalloc binary in PATH or at $Z80_REGALLOC_PATH
package vir

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
)

// GPUFuncDesc is the JSON schema for the CUDA regalloc kernel.
type GPUFuncDesc struct {
	NVregs           int              `json:"nVregs"`
	Ops              []GPUOpDesc      `json:"ops"`
	Interference     [][2]int         `json:"interference"`
	ParamConstraints []GPUParamConst  `json:"paramConstraints"`
}

type GPUOpDesc struct {
	Dst      int             `json:"dst"`
	Src0     int             `json:"src0"`
	Src1     int             `json:"src1"`
	Patterns []GPUPatternDesc `json:"patterns"`
}

type GPUPatternDesc struct {
	DstLocs    []int `json:"dstLocs"`
	SrcLocs0   []int `json:"srcLocs0"`
	SrcLocs1   []int `json:"srcLocs1"`
	Cost       int   `json:"cost"`
	TiedDstSrc bool  `json:"tiedDstSrc,omitempty"`
}

type GPUParamConst struct {
	Vreg int `json:"vreg"`
	Loc  int `json:"loc"`
}

// GPUResult is the JSON output from the CUDA kernel.
type GPUResult struct {
	Cost        int   `json:"cost"`
	Assignment  []int `json:"assignment"`
	SearchSpace int64 `json:"searchSpace"`
	Feasible    int64 `json:"feasible"`
}

// gpuRegAllocPath returns the path to the z80_regalloc binary.
func gpuRegAllocPath() string {
	if p := os.Getenv("Z80_REGALLOC_PATH"); p != "" {
		return p
	}
	// Try common locations
	for _, p := range []string{
		"z80_regalloc",
		os.ExpandEnv("$HOME/dev/z80-optimizer/cuda/z80_regalloc"),
	} {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
		// Check absolute path
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// SolveGPU converts a VIR problem to JSON, invokes the GPU kernel, and
// returns the optimal register assignment (vreg → physical loc index).
// Returns nil, error if GPU is unavailable or function is too large.
func SolveGPU(ops []VIROp, desc *MachineDesc, opts SolverOptions) (map[int]int, int, error) {
	binPath := gpuRegAllocPath()
	if binPath == "" {
		return nil, 0, fmt.Errorf("z80_regalloc not found (set Z80_REGALLOC_PATH)")
	}

	prob := buildProblem(ops, desc)

	// Map vregs to dense 0..N-1 indices for the GPU
	vregList := make([]int, 0, len(prob.vregs))
	for v := range prob.vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)

	if len(vregList) > 14 {
		return nil, 0, fmt.Errorf("too many vregs for GPU: %d (max 14)", len(vregList))
	}

	vregIdx := make(map[int]int) // vreg → dense index
	for i, v := range vregList {
		vregIdx[v] = i
	}

	// Build GPU function description
	gf := GPUFuncDesc{
		NVregs: len(vregList),
	}

	// Convert ops
	for i, op := range ops {
		gop := GPUOpDesc{Dst: -1, Src0: -1, Src1: -1}
		if op.Dst > 0 {
			if idx, ok := vregIdx[op.Dst]; ok {
				gop.Dst = idx
			}
		}
		if op.Src[0] > 0 {
			if idx, ok := vregIdx[op.Src[0]]; ok {
				gop.Src0 = idx
			}
		}
		if op.Src[1] > 0 {
			if idx, ok := vregIdx[op.Src[1]]; ok {
				gop.Src1 = idx
			}
		}

		// Convert matching patterns (only 8-bit GPR locs: indices 0-6)
		for _, pi := range prob.patterns[i] {
			pat := &desc.Patterns[pi]
			gpat := GPUPatternDesc{
				Cost:       pat.Cost,
				TiedDstSrc: pat.TiedDstSrc,
				DstLocs:    locSetToGPU(pat.DstLocs),
				SrcLocs0:   locSetToGPU(pat.SrcLocs[0]),
				SrcLocs1:   locSetToGPU(pat.SrcLocs[1]),
			}
			gop.Patterns = append(gop.Patterns, gpat)
		}

		if len(gop.Patterns) > 0 {
			gf.Ops = append(gf.Ops, gop)
		}
	}

	// Build interference from liveness
	emitted := make(map[[2]int]bool)
	for _, l := range prob.liveness {
		ids := make([]int, 0, len(l.live))
		for v := range l.live {
			if _, ok := vregIdx[v]; ok {
				ids = append(ids, vregIdx[v])
			}
		}
		sort.Ints(ids)
		for a := 0; a < len(ids); a++ {
			for b := a + 1; b < len(ids); b++ {
				pair := [2]int{ids[a], ids[b]}
				if !emitted[pair] {
					gf.Interference = append(gf.Interference, pair)
					emitted[pair] = true
				}
			}
		}
	}

	// Param constraints
	if opts.FuncParamLocs != nil || opts.ParamLocs != nil {
		paramLocs := opts.ParamLocs
		if paramLocs == nil {
			paramLocs = make(map[int]int)
		}
		for vreg, phys := range paramLocs {
			if idx, ok := vregIdx[vreg]; ok && phys < 7 { // only 8-bit GPR
				gf.ParamConstraints = append(gf.ParamConstraints, GPUParamConst{Vreg: idx, Loc: phys})
			}
		}
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(gf)
	if err != nil {
		return nil, 0, fmt.Errorf("GPU json: %w", err)
	}

	// Write JSON to temp file (avoids stdin pipe issues with CUDA process)
	tmpFile, err := os.CreateTemp("", "gpu_regalloc_*.json")
	if err != nil {
		return nil, 0, fmt.Errorf("GPU tmpfile: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Write(jsonData)
	tmpFile.Close()

	// Invoke kernel via shell: z80_regalloc --json < tmpfile
	out, err := exec.Command("sh", "-c", fmt.Sprintf("cat %s | %s --json", tmpPath, binPath)).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, 0, fmt.Errorf("GPU exec: %w\nstderr: %s", err, string(exitErr.Stderr))
		}
		return nil, 0, fmt.Errorf("GPU exec: %w", err)
	}

	// Parse result
	var result GPUResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, 0, fmt.Errorf("GPU parse result: %w\nraw: %s", err, string(out))
	}

	if result.Cost < 0 {
		return nil, 0, fmt.Errorf("GPU: no feasible assignment (space=%d)", result.SearchSpace)
	}

	// Map back from dense indices to original vregs
	assignment := make(map[int]int)
	for i, loc := range result.Assignment {
		if i < len(vregList) {
			assignment[vregList[i]] = loc
		}
	}

	return assignment, result.Cost, nil
}

// locSetToGPU converts a LocSet to a slice of GPU-compatible loc indices (0-6 only).
func locSetToGPU(ls LocSet) []int {
	var result []int
	ls.ForEach(func(loc int) bool {
		if loc < 7 { // only 8-bit GPR: A=0..L=6
			result = append(result, loc)
		}
		return true
	})
	return result
}
