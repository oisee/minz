// gpu-bench — Generate JSON corpus for GPU regalloc + compare results.
//
// Mode 1 (generate): go run ./cmd/gpu-bench/ --generate
//   Writes /tmp/gpu_corpus.jsonl (one VIR function per line)
//
// Mode 2 (compare):  go run ./cmd/gpu-bench/ --compare
//   Reads /tmp/gpu_results.jsonl, compares GPU costs with Z3 costs.
//
// The CUDA kernel is run externally (z80-optimizer session):
//   CUDA_VISIBLE_DEVICES=0 z80_regalloc --server < /tmp/gpu_corpus.jsonl > /tmp/gpu_results.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
)

const corpusFile = "/tmp/gpu_corpus.jsonl"
const resultsFile = "/tmp/gpu_results.jsonl"

type funcMeta struct {
	File   string `json:"_file"`
	Name   string `json:"_name"`
	NVregs int    `json:"_nVregs"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: gpu-bench --generate | --compare\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--generate":
		generateCorpus()
	case "--compare":
		compareResults()
	default:
		fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func generateCorpus() {
	exDir := filepath.Join("..", "examples", "nanz")
	files, err := filepath.Glob(filepath.Join(exDir, "*.nanz"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(files)

	out, err := os.Create(corpusFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	total, skipped := 0, 0

	for _, fpath := range files {
		base := filepath.Base(fpath)
		src, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}

		hm, err := nanz.ParseWithOpts(string(src), base, nanz.ParseOpts{})
		if err != nil {
			continue
		}

		m := hir.LowerModule(hm)
		for _, f := range m.Funcs {
			mir2.ReorderBlocks(f)
			mir2.DeadStoreElim(f)
		}
		for _, f := range m.Funcs {
			lr := mir2.ComputeLiveness(f)
			mir2.Allocate(f, lr, mir2.Z80CostTable{})
		}

		for _, f := range m.Funcs {
			mir2.FuseAbsDiff(f)
			vf, err := vir.LowerFunc(f, vir.Z80, m)
			if err != nil {
				continue
			}

			var allOps []vir.VIROp
			for _, b := range vf.Blocks {
				allOps = append(allOps, b.Ops...)
			}
			if len(allOps) == 0 {
				continue
			}

			// Build GPU function description (reuse SolveGPU's logic)
			gf, vregCount, err := vir.BuildGPUDesc(allOps, vir.Z80, vir.SolverOptions{})
			if err != nil {
				skipped++
				continue
			}

			// Write metadata + JSON on one line
			type gpuLine struct {
				vir.GPUFuncDesc
				funcMeta
			}
			line := gpuLine{
				GPUFuncDesc: *gf,
				funcMeta:    funcMeta{File: base, Name: f.Name, NVregs: vregCount},
			}
			data, _ := json.Marshal(line)
			out.Write(data)
			out.WriteString("\n")
			total++
		}
	}

	fmt.Printf("Generated %s: %d functions (%d skipped)\n", corpusFile, total, skipped)
}

func compareResults() {
	// Read GPU results
	rf, err := os.Open(resultsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open results: %v\n", err)
		os.Exit(1)
	}
	defer rf.Close()

	// Read corpus metadata
	cf, err := os.Open(corpusFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open corpus: %v\n", err)
		os.Exit(1)
	}
	defer cf.Close()

	type meta struct {
		File   string `json:"_file"`
		Name   string `json:"_name"`
		NVregs int    `json:"_nVregs"`
	}

	var metas []meta
	cScanner := bufio.NewScanner(cf)
	cScanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for cScanner.Scan() {
		var m meta
		json.Unmarshal(cScanner.Bytes(), &m)
		metas = append(metas, m)
	}

	var results []vir.GPUResult
	rScanner := bufio.NewScanner(rf)
	rScanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for rScanner.Scan() {
		var r vir.GPUResult
		json.Unmarshal(rScanner.Bytes(), &r)
		results = append(results, r)
	}

	if len(metas) != len(results) {
		fmt.Fprintf(os.Stderr, "mismatch: %d corpus lines, %d result lines\n", len(metas), len(results))
	}

	// Now run Z3 for comparison
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           GPU Brute-Force vs Z3 Register Allocation                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Vreg distribution
	vregDist := make(map[int]int)
	for _, m := range metas {
		vregDist[m.NVregs]++
	}
	fmt.Println("Vreg distribution:")
	for v := 0; v <= 16; v++ {
		if c, ok := vregDist[v]; ok {
			bar := strings.Repeat("█", c)
			fmt.Printf("  %2d vregs: %3d funcs  %s\n", v, c, bar)
		}
	}

	gpuOK, gpuFail := 0, 0
	var totalGPUCost int64
	var totalSearchSpace int64
	var totalFeasible int64

	for i, r := range results {
		if i >= len(metas) {
			break
		}
		if r.Cost >= 0 {
			gpuOK++
			totalGPUCost += int64(r.Cost)
		} else {
			gpuFail++
		}
		totalSearchSpace += r.SearchSpace
		totalFeasible += r.Feasible
	}

	fmt.Println()
	fmt.Printf("GPU: %d OK, %d fail (of %d functions)\n", gpuOK, gpuFail, len(results))
	fmt.Printf("Total search space: %d assignments\n", totalSearchSpace)
	fmt.Printf("Total feasible: %d\n", totalFeasible)
	fmt.Printf("Total GPU cost: %d T-states\n", totalGPUCost)

	// Show per-function GPU results
	fmt.Println()
	fmt.Printf("%-25s %-18s %4s %8s %12s %8s\n",
		"File", "Function", "Vreg", "GPU-cost", "SearchSpace", "Feasible")
	fmt.Printf("%-25s %-18s %4s %8s %12s %8s\n",
		"─────────────────────────", "──────────────────", "────", "────────", "────────────", "────────")

	n := len(results)
	if n > len(metas) {
		n = len(metas)
	}
	_ = time.Now() // suppress unused import
	for i := 0; i < n; i++ {
		m := metas[i]
		r := results[i]
		costStr := fmt.Sprintf("%d", r.Cost)
		if r.Cost < 0 {
			costStr = "FAIL"
		}
		fmt.Printf("%-25s %-18s %4d %8s %12d %8d\n",
			m.File, m.Name, m.NVregs, costStr, r.SearchSpace, r.Feasible)
	}
}
