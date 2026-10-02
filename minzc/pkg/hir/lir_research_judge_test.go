package hir_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/minz/minzc/pkg/lir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// Reproduce the old native path's PBQP guidance without invoking production
// LIR emission. Research tests must never silently judge PBQP assembly.
func researchHints(s pipeline.Steps) lir.AllocHints {
	h := lir.AllocHints{}
	for reg, loc := range s.Allocation.Locs {
		if i := lir.Z80.LocByName(loc.Name); i >= 0 {
			h[int(reg)] = i
		}
	}
	return h
}

func TestLIRResearchByteArithmetic(t *testing.T) {
	for _, op := range []string{"+", "-", "<<", ">>"} {
		t.Run(op, func(t *testing.T) {
			expr, count := "a "+op+" b", 65536
			params := "a: u8, b: u8"
			inputs := func(i int) []int64 { return []int64{int64(i >> 8), int64(i & 255)} }
			if op == "<<" || op == ">>" {
				expr, params, count = "a "+op+" 1", "a: u8", 256
				inputs = func(i int) []int64 { return []int64{int64(i)} }
			}
			hm, err := nanz.Parse("fun leaf("+params+") -> u8 { return "+expr+" }", "research.nanz")
			if err != nil {
				t.Fatal(err)
			}
			model := func(a []int64) int64 {
				switch op {
				case "+":
					return (a[0] + a[1]) & 255
				case "-":
					return (a[0] - a[1]) & 255
				case "<<":
					return (a[0] << 1) & 255
				default:
					return a[0] >> 1
				}
			}
			bad, first := sweepCodegenJudge(t, hm.Funcs, "leaf", count, inputs, model, false, true)
			if bad != 0 {
				t.Fatalf("%d/%d mismatches: %s", bad, count, first)
			}
		})
	}
}

// Comparisons returned as values are still rejected by the research allowlist;
// check this explicitly so a PBQP fallback cannot make the native judge green.
func TestLIRResearchComparisonValues(t *testing.T) {
	for _, op := range []string{"==", "<"} {
		t.Run(op, func(t *testing.T) {
			hm, err := nanz.Parse("fun cmp(a: u8, b: u8) -> bool { return a "+op+" b }", "research.nanz")
			if err != nil {
				t.Fatal(err)
			}
			s, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = lir.LIRCodegenFunc(s.MIR2Module.FuncByName("cmp"), s.MIR2Module, researchHints(s)); err == nil {
				t.Fatal("native comparison now compiles: replace rejection check with exhaustive execution judge")
			}
		})
	}
}

// These exact critic fixtures retain the complete Cartesian u8 domains. The
// opt-in run can be expensive (a2 has 2^32 inputs). MINZ_RESEARCH_LIMIT is only
// for recording an explicitly labelled prefix measurement, never a green gate.
type lirCompositionFixture struct {
	name, src string
	arity     int
	model     func([]int64) int64
	measured  string
}

func lirCompositionFixtures() []lirCompositionFixture {
	return []lirCompositionFixture{
		{"a1", `fun a1(a: u8, b: u8, c: u8) -> u8 { let p: u8 = a + b
 return p - (c + a) }`, 3, func(a []int64) int64 { return (a[1] - a[2]) & 255 }, "16646144/16777216 mismatches (full domain)"},
		{"a2", `fun a2(a: u8, b: u8, c: u8, d: u8) -> u8 { let p: u8 = a - b
 return p + (c - d) }`, 4, func(a []int64) int64 { return (a[0] - a[1] + a[2] - a[3]) & 255 }, "65280/65536 mismatches (prefix with c=d=0; full domain has 4294967296 inputs)"},
		{"a4", `fun a4(a: u8, b: u8, c: u8) -> u8 { return c - ((a >> 1) - (b << 1)) }`, 3, func(a []int64) int64 { return (a[2] - (a[0] >> 1) + (a[1] << 1)) & 255 }, "16711680/16777216 mismatches (full domain)"},
		{"a5", `fun a5(a: u8) -> u8 { return 5 - a + 3 }`, 1, func(a []int64) int64 { return (8 - a[0]) & 255 }, "256/256 mismatches (full domain)"},
		{"a8", `fun a8(a: u8, b: u8, c: u8) -> u8 { return b - a + (c << 1) }`, 3, func(a []int64) int64 { return (a[1] - a[0] + (a[2] << 1)) & 255 }, "16711680/16777216 mismatches (full domain)"},
		{"a9", `fun a9(a: u8, b: u8) -> u8 { let p: u8 = a >> 1
 return p + (a << 1) + b }`, 2, func(a []int64) int64 { return ((a[0] >> 1) + (a[0] << 1) + a[1]) & 255 }, "65024/65536 mismatches (full domain)"},
	}
}

func TestLIRResearchCompositions(t *testing.T) {
	for _, tc := range lirCompositionFixtures() {
		t.Run(tc.name, func(t *testing.T) {
			if os.Getenv("MINZ_RUN_KNOWN_RED") != "1" {
				t.Skipf("2026-10-02: native LIR %s: %s; MINZ_RUN_KNOWN_RED=1 runs full exhaustive u8 domain", tc.name, tc.measured)
			}
			hm, err := nanz.Parse(tc.src, "critic.nanz")
			if err != nil {
				t.Fatal(err)
			}
			count := int(uint64(1) << (8 * tc.arity))
			if limit := os.Getenv("MINZ_RESEARCH_LIMIT"); limit != "" {
				n, err := strconv.Atoi(limit)
				if err != nil || n <= 0 {
					t.Fatal("invalid research prefix limit")
				}
				if n < count {
					count = n
				}
				t.Logf("PREFIX ONLY: %d inputs of %d", count, uint64(1)<<(8*tc.arity))
			}
			inputs := func(i int) []int64 {
				a := make([]int64, tc.arity)
				for k := range a {
					a[k] = int64((i >> (8 * k)) & 255)
				}
				return a
			}
			bad, first := sweepCodegenJudge(t, hm.Funcs, tc.name, count, inputs, tc.model, false, true)
			if bad != 0 {
				t.Fatalf("%s: %d/%d mismatches: %s", tc.name, bad, count, first)
			}
			if os.Getenv("MINZ_RESEARCH_LIMIT") != "" {
				t.Fatalf("prefix measurement only: %d/%d mismatches; rerun full domain", bad, count)
			}
		})
	}
}

// Production checks use the same source fixtures and independent models. Two
// byte arguments are exhausted while remaining arguments take the critic's
// values; the research opt-in above retains the full Cartesian domains.
func TestLIRDisabledCompositionExecution(t *testing.T) {
	for _, tc := range lirCompositionFixtures() {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := nanz.Parse(tc.src, "critic.nanz")
			if err != nil {
				t.Fatal(err)
			}
			count := 65536
			if tc.arity == 1 {
				count = 256
			}
			inputs := func(i int) []int64 {
				a := []int64{int64(i & 255), int64((i >> 8) & 255), 3, 4}
				return a[:tc.arity]
			}
			bad, first := sweepCodegenJudge(t, hm.Funcs, tc.name, count, inputs, tc.model, true)
			plainBad, plainFirst := sweepCodegenJudge(t, hm.Funcs, tc.name, count, inputs, tc.model, false)
			if bad != 0 || plainBad != 0 {
				t.Fatalf("--lir %d/%d mismatches (%s); plain %d/%d (%s)", bad, count, first, plainBad, count, plainFirst)
			}
		})
	}
}
