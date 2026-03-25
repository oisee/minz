// vir-enumerate — Generate all possible small regalloc constraint patterns.
//
// Enumerates GPUFuncDesc JSON for CUDA brute-force regalloc.
// Each output line is a JSON object that can be piped to z80_regalloc --server.
//
// Strategy: enumerate combinations of "op classes" (distinct pattern sets)
// with interference graphs, deduplicate by signature.
//
// Usage:
//   vir-enumerate --max-vregs=3 --max-ops=5 | z80_regalloc --server > results.jsonl
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/minz/minzc/pkg/vir"
)

// opClass represents a distinct set of pattern constraints for one instruction.
// Two instructions with the same opClass are interchangeable for regalloc purposes.
type opClass struct {
	// dense vreg refs: dst, src0, src1 as offsets (0-based into vreg list)
	// -1 means unused
	nDst  bool // has destination
	nSrc0 bool // has src0
	nSrc1 bool // has src1
	// pattern fingerprint (hash of all pattern dst/src locs, cost, tied)
	patFP string
}

// extractOpClasses returns all distinct op classes from the Z80 pattern table.
// An op class = unique set of patterns that an instruction can match.
func extractOpClasses(desc *vir.MachineDesc) [][]vir.GPUPatternDesc {
	// Group patterns by (Op, Width) — each group is one op class
	type opKey struct {
		op    vir.Op
		width int
	}
	groups := make(map[opKey][]int) // opKey → pattern indices

	for i, pat := range desc.Patterns {
		key := opKey{pat.Op, pat.Width}
		groups[key] = append(groups[key], i)
	}

	// Convert to GPU pattern sets, dedup
	seen := make(map[string]bool)
	var classes [][]vir.GPUPatternDesc

	for _, indices := range groups {
		var pats []vir.GPUPatternDesc
		for _, pi := range indices {
			pat := &desc.Patterns[pi]
			pats = append(pats, vir.GPUPatternDesc{
				DstLocs:    locSetToSlice(pat.DstLocs),
				SrcLocs0:   locSetToSlice(pat.SrcLocs[0]),
				SrcLocs1:   locSetToSlice(pat.SrcLocs[1]),
				Cost:       pat.Cost,
				TiedDstSrc: pat.TiedDstSrc,
			})
		}
		// Fingerprint this pattern set
		fp := patternSetFP(pats)
		if !seen[fp] {
			seen[fp] = true
			classes = append(classes, pats)
		}
	}

	// Sort for deterministic enumeration
	sort.Slice(classes, func(i, j int) bool {
		return patternSetFP(classes[i]) < patternSetFP(classes[j])
	})

	return classes
}

func locSetToSlice(ls vir.LocSet) []int {
	var result []int
	for i := 0; i < 64; i++ {
		if ls.Has(i) {
			result = append(result, i)
		}
	}
	return result
}

func patternSetFP(pats []vir.GPUPatternDesc) string {
	h := sha256.New()
	for _, p := range pats {
		fmt.Fprintf(h, "%v,%v,%v,%d,%v;", p.DstLocs, p.SrcLocs0, p.SrcLocs1, p.Cost, p.TiedDstSrc)
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}

// opTemplate describes how an op uses vregs
type opTemplate struct {
	hasDst  bool
	hasSrc0 bool
	hasSrc1 bool
	pats    []vir.GPUPatternDesc
}

func classifyOp(pats []vir.GPUPatternDesc) opTemplate {
	t := opTemplate{pats: pats}
	for _, p := range pats {
		if len(p.DstLocs) > 0 {
			t.hasDst = true
		}
		if len(p.SrcLocs0) > 0 {
			t.hasSrc0 = true
		}
		if len(p.SrcLocs1) > 0 {
			t.hasSrc1 = true
		}
	}
	return t
}

func main() {
	maxVregs := flag.Int("max-vregs", 3, "Maximum number of vregs to enumerate")
	maxOps := flag.Int("max-ops", 5, "Maximum number of ops per function")
	minOps := flag.Int("min-ops", 2, "Minimum number of ops per function")
	maxTpl := flag.Int("max-templates", 6, "Maximum op class templates to use (reduces combinatorial explosion)")
	flag.Parse()

	desc := vir.Z80

	classes := extractOpClasses(desc)
	fmt.Fprintf(os.Stderr, "Found %d distinct op classes\n", len(classes))

	templates := make([]opTemplate, len(classes))
	for i, c := range classes {
		templates[i] = classifyOp(c)
	}

	// For small enumerations, generate all combinations:
	// - Pick nOps ops from the template set
	// - Assign vreg references (dst, src0, src1) from [0, nVregs)
	// - Generate all interference graphs (subset of all vreg pairs)
	// - Deduplicate by signature hash

	type enumKey string
	seen := make(map[enumKey]bool)
	total := 0
	dupes := 0

	enc := json.NewEncoder(os.Stdout)

	for nVregs := 2; nVregs <= *maxVregs; nVregs++ {
		for nOps := *minOps; nOps <= *maxOps; nOps++ {
			if nOps < nVregs {
				continue // need at least nVregs ops to define all vregs
			}

			fmt.Fprintf(os.Stderr, "Enumerating: %d vregs, %d ops (%d templates)...\n",
				nVregs, nOps, len(templates))

			// Enumerate op class combinations (with replacement)
			enumOps(templates, nOps, nVregs, *maxTpl, func(ops []vir.GPUOpDesc) {
				// Enumerate interference graphs
				// All pairs of vregs (n*(n-1)/2 possible edges)
				nPairs := nVregs * (nVregs - 1) / 2
				allPairs := make([][2]int, 0, nPairs)
				for a := 0; a < nVregs; a++ {
					for b := a + 1; b < nVregs; b++ {
						allPairs = append(allPairs, [2]int{a, b})
					}
				}

				// Enumerate subsets of interference pairs (2^nPairs)
				for mask := 0; mask < (1 << nPairs); mask++ {
					var interf [][2]int
					for bit := 0; bit < nPairs; bit++ {
						if mask&(1<<bit) != 0 {
							interf = append(interf, allPairs[bit])
						}
					}

					gf := vir.GPUFuncDesc{
						NVregs:       nVregs,
						Ops:          ops,
						Interference: interf,
					}

					// Dedup by content hash
					key := enumKey(funcDescFP(&gf))
					if seen[key] {
						dupes++
						continue
					}
					seen[key] = true

					enc.Encode(gf)
					total++

					if total%10000 == 0 {
						fmt.Fprintf(os.Stderr, "  generated %d unique (%d dupes)\n", total, dupes)
					}
				}
			})
		}
	}

	fmt.Fprintf(os.Stderr, "Done: %d unique patterns (%d duplicates skipped)\n", total, dupes)
}

func funcDescFP(gf *vir.GPUFuncDesc) string {
	h := sha256.New()
	fmt.Fprintf(h, "V%d:", gf.NVregs)
	for _, op := range gf.Ops {
		fmt.Fprintf(h, "O%d,%d,%d:", op.Dst, op.Src0, op.Src1)
		for _, p := range op.Patterns {
			fmt.Fprintf(h, "P%v,%v,%v,%d,%v;", p.DstLocs, p.SrcLocs0, p.SrcLocs1, p.Cost, p.TiedDstSrc)
		}
	}
	for _, i := range gf.Interference {
		fmt.Fprintf(h, "I%d,%d;", i[0], i[1])
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}

// enumOps enumerates all ways to assign op classes and vreg references.
// For each combination, calls fn with the generated ops.
func enumOps(templates []opTemplate, nOps, nVregs, maxTplCount int, fn func([]vir.GPUOpDesc)) {
	maxTemplates := len(templates)
	if maxTemplates > maxTplCount {
		maxTemplates = maxTplCount
	}

	ops := make([]vir.GPUOpDesc, nOps)
	var recurse func(depth int, definedVregs int)
	recurse = func(depth int, definedVregs int) {
		if depth == nOps {
			// Must have defined all nVregs
			if definedVregs >= nVregs {
				cpy := make([]vir.GPUOpDesc, nOps)
				copy(cpy, ops)
				fn(cpy)
			}
			return
		}

		remaining := nOps - depth
		needToDefine := nVregs - definedVregs

		for ti := 0; ti < maxTemplates; ti++ {
			t := templates[ti]

			// Assign dst vreg
			dstMin, dstMax := -1, -1
			if t.hasDst {
				// Can define a new vreg or redefine existing
				dstMin = 0
				dstMax = min(definedVregs, nVregs-1) // can define next vreg
				if definedVregs < nVregs && remaining <= needToDefine {
					// Must define a new vreg
					dstMin = definedVregs
				}
			}

			for dst := dstMin; dst <= dstMax; dst++ {
				newDefined := definedVregs
				if dst == definedVregs && dst < nVregs {
					newDefined = definedVregs + 1
				}

				// Assign src0 (must be already defined)
				src0Min, src0Max := -1, -1
				if t.hasSrc0 {
					if newDefined > 0 {
						src0Min = 0
						src0Max = newDefined - 1
					} else {
						continue // no defined vregs to use as source
					}
				}

				for src0 := src0Min; src0 <= src0Max; src0++ {
					// Assign src1
					src1Min, src1Max := -1, -1
					if t.hasSrc1 {
						if newDefined > 0 {
							src1Min = 0
							src1Max = newDefined - 1
						} else {
							continue
						}
					}

					for src1 := src1Min; src1 <= src1Max; src1++ {
						ops[depth] = vir.GPUOpDesc{
							Dst:      dst,
							Src0:     src0,
							Src1:     src1,
							Patterns: t.pats,
						}
						recurse(depth+1, newDefined)
					}
				}
			}
		}
	}

	recurse(0, 0)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
