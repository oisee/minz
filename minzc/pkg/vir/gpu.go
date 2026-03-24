// gpu.go — GPU brute-force register allocator bridge.
//
// Serializes VIR function constraints to JSON, sends to the CUDA
// z80_regalloc --server process, and parses the optimal assignment back.
//
// Server mode: CUDA initializes once, GPU buffers reused across solves.
// Protocol: write JSON-per-line to stdin, read JSON-per-line from stdout.
// "regalloc-server: ready" on stderr signals startup complete.
//
// Requires: z80_regalloc binary in PATH or at $Z80_REGALLOC_PATH
package vir

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
)

// GPUFuncDesc is the JSON schema for the CUDA regalloc kernel.
type GPUFuncDesc struct {
	NVregs           int              `json:"nVregs"`
	Ops              []GPUOpDesc      `json:"ops"`
	Interference     [][2]int         `json:"interference"`
	ParamConstraints []GPUParamConst  `json:"paramConstraints"`
}

type GPUOpDesc struct {
	Dst      int              `json:"dst"`
	Src0     int              `json:"src0"`
	Src1     int              `json:"src1"`
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
	Cost        int    `json:"cost"`
	Assignment  []int  `json:"assignment"`
	SearchSpace int64  `json:"searchSpace"`
	Feasible    int64  `json:"feasible"`
	Error       string `json:"error,omitempty"`
}

// ── GPU Server (persistent process) ─────────────────────────────────────────

// gpuServer manages a long-running z80_regalloc --server process.
type gpuServer struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
}

var (
	globalGPU     *gpuServer
	globalGPUOnce sync.Once
	globalGPUErr  error
)

// getGPUServer returns a shared GPU server instance, starting it if needed.
func getGPUServer() (*gpuServer, error) {
	globalGPUOnce.Do(func() {
		globalGPU, globalGPUErr = startGPUServer()
	})
	return globalGPU, globalGPUErr
}

func startGPUServer() (*gpuServer, error) {
	binPath := gpuRegAllocPath()
	if binPath == "" {
		return nil, fmt.Errorf("z80_regalloc not found (set Z80_REGALLOC_PATH)")
	}

	cmd := exec.Command(binPath, "--server")
	cmd.Env = append(os.Environ(), "CUDA_VISIBLE_DEVICES=0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // isolate from Go test signal group

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("GPU stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("GPU stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("GPU stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("GPU start: %w", err)
	}

	// Wait for "regalloc-server: ready" on stderr
	stderrScanner := bufio.NewScanner(stderr)
	ready := make(chan bool, 1)
	go func() {
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			if line == "regalloc-server: ready" {
				ready <- true
				return
			}
		}
		ready <- false
	}()

	if !<-ready {
		cmd.Process.Kill()
		return nil, fmt.Errorf("GPU server did not become ready")
	}

	// Drain remaining stderr in background
	go func() {
		for stderrScanner.Scan() {
			// discard
		}
	}()

	return &gpuServer{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

// solve sends a function description to the GPU server and reads the result.
func (s *gpuServer) solve(gf *GPUFuncDesc) (*GPUResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	jsonData, err := json.Marshal(gf)
	if err != nil {
		return nil, fmt.Errorf("GPU json: %w", err)
	}

	// Write JSON + newline
	if _, err := s.stdin.Write(append(jsonData, '\n')); err != nil {
		return nil, fmt.Errorf("GPU write: %w", err)
	}

	// Read result line
	if !s.stdout.Scan() {
		return nil, fmt.Errorf("GPU read: %v", s.stdout.Err())
	}

	var result GPUResult
	if err := json.Unmarshal(s.stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("GPU parse: %w\nraw: %s", err, s.stdout.Text())
	}

	if result.Error != "" {
		return nil, fmt.Errorf("GPU error: %s", result.Error)
	}

	return &result, nil
}

// ── Public API ──────────────────────────────────────────────────────────────

// gpuRegAllocPath returns the path to the z80_regalloc binary.
func gpuRegAllocPath() string {
	if p := os.Getenv("Z80_REGALLOC_PATH"); p != "" {
		return p
	}
	for _, p := range []string{
		"z80_regalloc",
		os.ExpandEnv("$HOME/dev/z80-optimizer/cuda/z80_regalloc"),
	} {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// BuildGPUDesc converts VIR ops into a GPUFuncDesc for the CUDA kernel.
// Returns the desc, vreg count, and error. Exported for use by gpu-bench.
func BuildGPUDesc(ops []VIROp, desc *MachineDesc, opts SolverOptions) (*GPUFuncDesc, int, error) {
	prob := buildProblem(ops, desc)

	vregList := make([]int, 0, len(prob.vregs))
	for v := range prob.vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)

	if len(vregList) > 14 {
		return nil, len(vregList), fmt.Errorf("too many vregs for GPU: %d (max 14)", len(vregList))
	}

	vregIdx := make(map[int]int)
	for i, v := range vregList {
		vregIdx[v] = i
	}

	gf := &GPUFuncDesc{NVregs: len(vregList)}

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

	// Interference from liveness
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
	if opts.ParamLocs != nil {
		for vreg, phys := range opts.ParamLocs {
			if idx, ok := vregIdx[vreg]; ok && phys < 7 {
				gf.ParamConstraints = append(gf.ParamConstraints, GPUParamConst{Vreg: idx, Loc: phys})
			}
		}
	}

	return gf, len(vregList), nil
}

// SolveGPU converts a VIR problem to JSON, sends to the GPU server, and
// returns the optimal register assignment (vreg → physical loc index).
func SolveGPU(ops []VIROp, desc *MachineDesc, opts SolverOptions) (map[int]int, int, error) {
	srv, err := getGPUServer()
	if err != nil {
		return nil, 0, err
	}

	gf, _, err := BuildGPUDesc(ops, desc, opts)
	if err != nil {
		return nil, 0, err
	}

	result, err := srv.solve(gf)
	if err != nil {
		return nil, 0, err
	}

	if result.Cost < 0 {
		return nil, 0, fmt.Errorf("GPU: no feasible assignment (space=%d)", result.SearchSpace)
	}

	// Map back: need vregList to reverse the dense indices
	prob := buildProblem(ops, desc)
	vregList := make([]int, 0, len(prob.vregs))
	for v := range prob.vregs {
		vregList = append(vregList, v)
	}
	sort.Ints(vregList)

	assignment := make(map[int]int)
	for i, loc := range result.Assignment {
		if i < len(vregList) {
			assignment[vregList[i]] = loc
		}
	}

	return assignment, result.Cost, nil
}

// locSetToGPU converts a LocSet to GPU-compatible loc indices (0-6 only).
// Empty LocSet → all 7 GPR (unconstrained).
func locSetToGPU(ls LocSet) []int {
	var result []int
	ls.ForEach(func(loc int) bool {
		if loc < 7 {
			result = append(result, loc)
		}
		return true
	})
	if len(result) == 0 {
		// Unconstrained — any GPR is valid
		return []int{0, 1, 2, 3, 4, 5, 6}
	}
	return result
}
