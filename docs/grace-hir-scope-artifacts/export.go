// Run from minzc: go run ../docs/grace-hir-scope-artifacts/export.go
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
)

func main() {
	_, file, _, _ := runtime.Caller(0)
	out := filepath.Dir(file)
	repo := filepath.Clean(filepath.Join(out, "../.."))
	test := filepath.Join(repo, "minzc/pkg/hir/lower_scope_pipeline_test.go")
	tree, e := parser.ParseFile(token.NewFileSet(), test, nil, 0)
	if e != nil {
		panic(e)
	}
	source := ""
	ast.Inspect(tree, func(n ast.Node) bool {
		v, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range v.Names {
			if name.Name == "src" {
				lit, ok := v.Values[i].(*ast.BasicLit)
				if !ok {
					panic("nonliteral source")
				}
				source, e = strconv.Unquote(lit.Value)
				if e != nil {
					panic(e)
				}
			}
		}
		return true
	})
	if source == "" {
		panic("missing src constant")
	}
	write := func(name string, v any) {
		var b []byte
		if x, ok := v.([]byte); ok {
			b = x
		} else {
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				panic(e)
			}
		}
		path := filepath.Join(out, name)
		if filepath.Ext(path) == ".gz" {
			f, e := os.Create(path)
			if e != nil {
				panic(e)
			}
			z := gzip.NewWriter(f)
			if _, e = z.Write(b); e != nil {
				panic(e)
			}
			if e = z.Close(); e != nil {
				panic(e)
			}
			f.Close()
		} else if e = os.WriteFile(path, b, 0644); e != nil {
			panic(e)
		}
	}
	write("source.nanz", []byte(source))
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"buf": {1, 2, 3}}, Globals: map[string][]byte{}}
	wants := []int64{8, 10, 10, 12, 16}
	for _, n := range []int64{0, 1, 2, 3, 5} {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "scoped", Arguments: []hir.ProfileArgument{literal(7), literal(n), {Resource: "buf"}}})
	}
	write("workload.json", w)
	factory := func() *hir.Module {
		m, e := nanz.Parse(source, "lexical-source.nanz")
		if e != nil {
			panic(e)
		}
		return m
	}
	original := factory()
	raw, e := hir.RunLoopProfile(original, w)
	if e != nil {
		panic(e)
	}
	for i, want := range wants {
		if raw.Results[i][0].I != want || !reflect.DeepEqual(raw.Snapshots[i].Resources["buf"], []byte{1, 2, 3}) || !reflect.DeepEqual(raw.Accesses[i].Resources["buf"].Reads, []uint64{1, 1, 1}) || !reflect.DeepEqual(raw.Accesses[i].Resources["buf"].Writes, []uint64{0, 0, 0}) {
			panic("independent raw scope oracle")
		}
	}
	write("raw.json.gz", raw)
	write("oracle.json", map[string]any{"results": wants, "resource": "buf", "snapshot": []byte{1, 2, 3}, "reads_per_call": []uint64{1, 1, 1}, "writes_per_call": []uint64{0, 0, 0}, "globals": "no user globals; compiler generated read-only storage follows raw snapshots"})
	rows := []map[string]any{}
	for _, grace := range []bool{false, true} {
		m := factory()
		steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "none", UseHIRGrace: grace})
		if e != nil {
			panic(e)
		}
		prefix := fmt.Sprintf("grace-%t", grace)
		write(prefix+".a80", []byte(steps.Assembly))
		write(prefix+".hir.gz", []byte(steps.HIR))
		write(prefix+"-raw.mir.gz", []byte(steps.MIR2Raw))
		write(prefix+"-optimized.mir.gz", []byte(steps.MIR2Opt))
		assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || assembled == nil {
			panic(e)
		}
		if len(assembled.Errors) > 0 {
			panic(assembled.Errors)
		}
		write(prefix+".bin", assembled.Binary)
		run, e := pipeline.RunZ80ProfileWorkload(steps, m, w, *raw)
		if e != nil {
			panic(e)
		}
		for i, want := range wants {
			inv := run.Invocations[i]
			if inv.Results[0].I != want || !reflect.DeepEqual(inv.Snapshot, raw.Snapshots[i]) || !reflect.DeepEqual(*inv.Access, raw.Accesses[i]) {
				panic("independent target scope oracle")
			}
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))
		if hash != run.BinarySHA256 {
			panic("binary digest")
		}
		write(prefix+"-target.json.gz", run)
		rows = append(rows, map[string]any{"grace": grace, "t_states": run.TotalTStates, "bytes": len(assembled.Binary), "binary_sha256": hash, "results": wants})
	}
	write("costs.json", rows)
	fmt.Println(string(mustJSON(rows)))
}
func mustJSON(v any) []byte {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	return b
}
