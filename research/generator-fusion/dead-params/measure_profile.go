package main

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
)

func main() {
	source, e := os.ReadFile("../examples/nanz/01_sum_array.nanz")
	if e != nil {
		panic(e)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), "examples/nanz/01_sum_array.nanz") }
	n := int64(40)
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-pbqp-default-jrs/v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"input": make([]byte, 40)}, Calls: []hir.ProfileInvocation{{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &n}}}}}
	options := pipeline.Options{AssertMode: "none"}
	if len(os.Args) > 2 && os.Args[2] == "--hir-grace" {
		options.UseHIRGrace = true
	}
	model, evidence, e := pipeline.MeasureLoopCostModel(factory, "sum_array", workload, options)
	if e != nil {
		panic(e)
	}
	out := os.Args[1]
	if e = os.MkdirAll(out, 0755); e != nil {
		panic(e)
	}
	write := func(name string, v any) {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(out, name), b, 0644); e != nil {
			panic(e)
		}
	}
	write("model.json", model)
	write("evidence.json", evidence)
	m, e := factory()
	if e != nil {
		panic(e)
	}
	if e = hir.SeedLoopLineage(m); e != nil {
		panic(e)
	}
	if _, e = hir.RunGrace(m, 0); e != nil {
		panic(e)
	}
	steps, e := pipeline.CompileHIRSteps(m, options)
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(filepath.Join(out, "pointer.a80"), []byte(steps.Assembly), 0644); e != nil {
		panic(e)
	}
	b, _ := json.MarshalIndent(model, "", "  ")
	fmt.Println(string(b))
	fmt.Printf("Rows: %d\n", len(evidence.Rows))
}
