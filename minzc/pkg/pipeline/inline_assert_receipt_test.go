package pipeline

import (
	"fmt"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
)

func TestLizpInlineAssertReceipts(t *testing.T) {
	const path = "../../../examples/lizp/self_tokenizer.lizp"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := lizp.Compile(string(source), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Asserts) != 7 {
		t.Fatal("tokenizer assertion inventory changed", len(inventory.Asserts))
	}
	// Exercise the ordinary size gate directly as well as A1's outer search;
	// search trials disable assertions and would otherwise mask a gate replay.
	for _, disableSearch := range []bool{true, false} {
		for _, backend := range []string{"mir2", "z80"} {
			for index, assertion := range inventory.Asserts {
				t.Run(fmt.Sprintf("search-disabled-%t/%s/line%d", disableSearch, backend, assertion.Line), func(t *testing.T) {
					h, err := lizp.Compile(string(source), path)
					if err != nil {
						t.Fatal(err)
					}
					h.Asserts = h.Asserts[index : index+1]
					h.Asserts[0].Via = backend
					h.AssertStats = &hir.AssertStats{}
					opt := DefaultOptions()
					opt.AssertMode = backend
					opt.DisableVariantSearch = disableSearch
					if _, err := CompileHIRSteps(h, opt); err != nil {
						t.Fatal(err)
					}
					if *h.AssertStats != (hir.AssertStats{Executed: 1, Passed: 1}) {
						t.Fatalf("want exactly one execution per assert, got %+v", *h.AssertStats)
					}
				})
			}
		}
	}
}
