package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"time"
)

func main() {
	sourcePath := "pkg/pipeline/testdata/grace_rule_budget.nanz"
	src, e := os.ReadFile(sourcePath)
	if e != nil {
		panic(e)
	}
	out := os.Args[1]
	os.MkdirAll(out, 0755)
	for _, size := range []bool{false} {
		for mask := 0; mask < 4; mask++ {
			m, e := nanz.Parse(string(src), sourcePath)
			if e != nil {
				panic(e)
			}
			m.GraceRulePolicy = map[string]bool{"hir-constant-if": mask&1 != 0, "hir-invariant-base": mask&2 != 0}
			if e = hir.SeedLoopLineage(m); e != nil {
				panic(e)
			}
			m.PointerWalkPolicy = map[string]bool{}
			start := time.Now()
			if _, e = hir.RunGraceWithOptions(m, hir.GraceOptions{PointerWalkSites: m.PointerWalkPolicy}); e != nil {
				panic(e)
			}
			steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "none", UseHIRGrace: true, OptSize: size})
			if e != nil {
				panic(e)
			}
			name := fmt.Sprintf("size-%v-mask-%d.a80", size, mask)
			if e = os.WriteFile(filepath.Join(out, name), []byte(steps.Assembly), 0644); e != nil {
				panic(e)
			}
			fmt.Printf("%s SHA256 %x bytes %d\n", name, sha256.Sum256([]byte(steps.Assembly)), len(steps.Assembly))
			fmt.Printf("elapsed %s\n", time.Since(start))
		}
	}
}
