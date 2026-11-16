package pipeline

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

// Parameterless callees of different shapes (several returns, a loop, a
// store, a bool result) are called while the caller keeps values in BC, DE,
// HL and the index registers. Their emitted bodies prove which registers they
// write, so the caller saves only those; every argument byte is checked on
// the Z80 against the MIR2 VM.
func TestExactClobbersMultiBlockLeavesNative(t *testing.T) {
	src, e := os.ReadFile("testdata/exact_clobbers_leaves.nanz")
	if e != nil {
		t.Fatal(e)
	}
	runFlagReturnNative(t, string(src), "mix")

	saves := func(opts Options) map[string]string {
		h, e := nanz.Parse(string(src), "exact_clobbers_leaves.nanz")
		if e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(h, opts)
		if e != nil {
			t.Fatal(e)
		}
		// Caller saves sit between the genCall marker and the CALL.
		out := map[string]string{}
		site := regexp.MustCompile(`(?s); genCall: (leaf_\w+) dst=\d+\n(.*?)\n\s*CALL leaf_\w+\n`)
		for _, m := range site.FindAllStringSubmatch(steps.Assembly, -1) {
			for _, line := range strings.Split(m[2], "\n") {
				if f := strings.Fields(line); len(f) == 2 && f[0] == "PUSH" {
					out[m[1]] += f[1] + " "
				}
			}
		}
		if len(out) == 0 && !strings.Contains(steps.Assembly, "CALL leaf_") {
			t.Fatalf("no leaf calls:\n%s", steps.Assembly)
		}
		return out
	}
	exact := saves(DefaultOptions())
	legacy := DefaultOptions()
	legacy.NoExactClobbers = true
	old := saves(legacy)
	for _, pair := range []string{"BC", "IX", "IY"} {
		if !strings.Contains(old["leaf_branch"]+old["leaf_store"]+old["leaf_loop"]+old["leaf_pred"], pair) {
			t.Errorf("legacy ABI saved no live %s around the leaves; the test no longer exercises it: %v", pair, old)
		}
	}
	for leaf, pairs := range exact {
		for _, p := range strings.Fields(pairs) {
			if p == "IX" || p == "IY" {
				t.Errorf("%s writes no index register but the caller saves %s (%v)", leaf, p, exact)
			}
		}
		if len(strings.Fields(pairs)) >= len(strings.Fields(old[leaf])) && old[leaf] != "" {
			t.Errorf("%s: saves %q not narrower than the full ABI %q", leaf, pairs, old[leaf])
		}
	}
}

// Callees with parameters (two byte arguments, a crossed pair of arguments,
// a word argument, a store, a bool result): each site saves only what the
// callee body and its own measured argument copies write. Every argument
// byte is checked on the Z80 against the MIR2 VM.
func TestExactClobbersParamCalleesNative(t *testing.T) {
	src, e := os.ReadFile("testdata/exact_clobbers_params.nanz")
	if e != nil {
		t.Fatal(e)
	}
	runFlagReturnNative(t, string(src), "mix2")
	pushes := func(opts Options) (int, map[string]int) {
		h, e := nanz.Parse(string(src), "exact_clobbers_params.nanz")
		if e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(h, opts)
		if e != nil {
			t.Fatal(e)
		}
		site := regexp.MustCompile(`(?s); genCall: (p_\w+) dst=\d+\n(.*?)\n\s*CALL p_\w+\n`)
		n, pairs := 0, map[string]int{}
		for _, m := range site.FindAllStringSubmatch(steps.Assembly, -1) {
			for _, line := range strings.Split(m[2], "\n") {
				if f := strings.Fields(line); len(f) == 2 && f[0] == "PUSH" {
					n++
					pairs[f[1]]++
				}
			}
		}
		if opts.NoExactClobbers == strings.Contains(steps.Assembly, "; exact args:") {
			t.Fatalf("exact-args markers do not follow the switch:\n%s", steps.Assembly)
		}
		return n, pairs
	}
	exact, exactPairs := pushes(DefaultOptions())
	legacyOpts := DefaultOptions()
	legacyOpts.NoExactClobbers = true
	legacy, legacyPairs := pushes(legacyOpts)
	for _, pair := range []string{"DE", "IX", "IY"} {
		if legacyPairs[pair] == 0 {
			t.Errorf("full ABI saved no live %s; the test no longer exercises it: %v", pair, legacyPairs)
		}
		if exactPairs[pair] >= legacyPairs[pair] {
			t.Errorf("%s saved %d times, full ABI %d", pair, exactPairs[pair], legacyPairs[pair])
		}
	}
	if exact >= legacy {
		t.Errorf("exact saves %d, full ABI %d", exact, legacy)
	}
}
