// Run from the integrated repository's minzc directory:
// go run /absolute/path/to/export.go -out /absolute/output/path
// The helper refuses unknown fixture functions. Extend the independent oracle
// explicitly if TestGotoC89SourceVMZ80 adds new calls.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

type oracleCase struct {
	Want   int64  `json:"want"`
	Reason string `json:"reason"`
}

var oracle = map[string]oracleCase{
	"skip":                   {1, "The forward jump skips x=99, leaving initialized x=1."},
	"outward":                {7, "The backward jump restores the outer x=7; both inner x=99 bindings leave scope."},
	"forward":                {9, "The assignment before the inner declaration updates outer x to 9; the jump exits the shadow and skips x=88."},
	"static_shadow":          {7, "The inner automatic x increments 99 to 100; outer static x remains 7."},
	"dead_both":              {11, "The jump enters the else label directly, writes outer x=11 and jumps to out; dead condition and then arm do not execute."},
	"loop_entry":             {7, "The jump enters the for body at label a, skipping induction initialization and immediately returning 7."},
	"dead_switch":            {9, "The jump directly enters case label a, writes outer x=9 and exits the switch; skipped dispatch does not execute."},
	"branch_assign":          {7, "The initializer is skipped; the true branch initializes x to 7 before its first read."},
	"return_assign":          {2, "The initializer is skipped; the false branch initializes x to 2 before its first read."},
	"assigned_before_shadow": {2, "The true branch first initializes outer x to 2, then declares and changes inner x; outer x returns 2."},
}

func sourceFromTest(path string) (string, []string) {
	tree, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	must(err)
	var function *ast.FuncDecl
	for _, decl := range tree.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "TestGotoC89SourceVMZ80" {
			if function != nil {
				panic("duplicate fixture function")
			}
			function = f
		}
	}
	if function == nil {
		panic("missing TestGotoC89SourceVMZ80")
	}
	var source string
	count := 0
	ast.Inspect(function.Body, func(n ast.Node) bool {
		d, ok := n.(*ast.DeclStmt)
		if !ok {
			return true
		}
		g, ok := d.Decl.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			return true
		}
		for _, spec := range g.Specs {
			v, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range v.Names {
				if name.Name != "src" {
					continue
				}
				if i >= len(v.Values) {
					panic("missing src initializer")
				}
				lit, ok := v.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					panic("nonliteral src")
				}
				source, err = strconv.Unquote(lit.Value)
				must(err)
				count++
			}
		}
		return true
	})
	if count != 1 || source == "" {
		panic(fmt.Sprintf("expected one fixture src constant, got %d", count))
	}
	var calls []string
	ast.Inspect(function.Body, func(n ast.Node) bool {
		literal, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		selector, ok := literal.Type.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "ProfileWorkload" {
			return true
		}
		for _, elt := range literal.Elts {
			key, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			name, ok := key.Key.(*ast.Ident)
			if !ok || name.Name != "Calls" {
				continue
			}
			sequence, ok := key.Value.(*ast.CompositeLit)
			if !ok {
				panic("nonliteral fixture calls")
			}
			for _, call := range sequence.Elts {
				inv, ok := call.(*ast.CompositeLit)
				if !ok {
					panic("nonliteral invocation")
				}
				functionName := ""
				for _, field := range inv.Elts {
					pair, ok := field.(*ast.KeyValueExpr)
					if !ok {
						panic("nonkeyed invocation")
					}
					key, ok := pair.Key.(*ast.Ident)
					if !ok || key.Name != "Function" {
						panic("new fixture call arguments need explicit oracle")
					}
					value, ok := pair.Value.(*ast.BasicLit)
					if !ok || value.Kind != token.STRING {
						panic("nonliteral function name")
					}
					functionName, err = strconv.Unquote(value.Value)
					must(err)
				}
				if functionName == "" {
					panic("missing invocation function")
				}
				calls = append(calls, functionName)
			}
		}
		return false
	})
	if len(calls) == 0 {
		panic("no exact fixture calls")
	}
	return source, calls
}

// mapFixtureLoop supplies explicit harness provenance, not frontend support.
// It validates one exact source token and one structured loop across this
// ten-function fixture; future extra loops require an explicit mapping update.
func mapFixtureLoop(m *hir.Module, source, hash string) hir.SourceLocation {
	const spelling = "for(int i=0;i<3;i++)"
	if strings.Count(source, spelling) != 1 {
		panic("expected unique exact fixture for token")
	}
	offset := strings.Index(source, spelling)
	line, column := 1, 1
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	location := hir.SourceLocation{Unit: "goto-source.c", ByteOffset: offset, Line: line, Column: column, SourceSHA256: hash}
	m.SourceLocations = map[hir.Stmt]hir.SourceLocation{}
	count := 0
	var walk func(*hir.Block, string)
	walk = func(b *hir.Block, fn string) {
		if b == nil {
			return
		}
		for _, stmt := range b.Body {
			switch x := stmt.(type) {
			case *hir.WhileStmt:
				count++
				if fn != "loop_entry" {
					panic("unexpected structured loop function")
				}
				m.SourceLocations[stmt] = location
				walk(x.Body, fn)
			case *hir.ForRangeStmt, *hir.ForEachStmt:
				panic("unexpected structured loop kind")
			case *hir.Block:
				walk(x, fn)
			case *hir.IfStmt:
				walk(x.Then, fn)
				walk(x.Else, fn)
			case *hir.SwitchStmt:
				for _, c := range x.Cases {
					walk(c.Body, fn)
				}
				walk(x.Default, fn)
			}
		}
	}
	for _, f := range m.Funcs {
		walk(f.Body, f.Name)
	}
	if count != 1 {
		panic(fmt.Sprintf("expected one mapped fixture loop, got %d", count))
	}
	return location
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	out := flag.String("out", "", "artifact directory (required)")
	test := flag.String("test", "pkg/hir/goto_c89_test.go", "fixture path, relative to minzc")
	flag.Parse()
	if *out == "" {
		panic("-out required")
	}
	must(os.MkdirAll(*out, 0755))
	write := func(name string, value any) {
		var b []byte
		if bytes, ok := value.([]byte); ok {
			b = bytes
		} else {
			var err error
			b, err = json.MarshalIndent(value, "", "  ")
			must(err)
			b = append(b, '\n')
		}
		path := filepath.Join(*out, name)
		if filepath.Ext(path) == ".gz" {
			f, err := os.Create(path)
			must(err)
			z := gzip.NewWriter(f)
			_, err = z.Write(b)
			must(err)
			must(z.Close())
			must(f.Close())
		} else {
			must(os.WriteFile(path, b, 0644))
		}
	}
	source, functions := sourceFromTest(*test)
	write("source.c", []byte(source))
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	write("source-identity.json", map[string]any{"filename": "goto-source.c", "sha256": hash, "extraction": "Go AST: sole const src within TestGotoC89SourceVMZ80 only", "loop_locations": "C frontend has no automatic collector provenance. Explicit fixture wrapper maps the unique for(int i=0;i<3;i++) token to the sole structured loop in loop_entry, with exact SHA, byte offset, line and column."})
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}}
	wants := []int64{}
	descriptions := []map[string]any{}
	seen := map[string]bool{}
	// The exact fixture calls are parsed from its ProfileWorkload literal.
	for _, name := range functions {
		expected, ok := oracle[name]
		if !ok {
			panic("independent oracle missing for " + name)
		}
		if seen[name] {
			panic("duplicate fixture call " + name)
		}
		seen[name] = true
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name})
		wants = append(wants, expected.Want)
		descriptions = append(descriptions, map[string]any{"function": name, "want": expected.Want, "reason": expected.Reason})
	}
	write("workload.json", w)
	factory := func() *hir.Module {
		m, err := c89.Compile(source, "goto-source.c")
		must(err)
		m.SourceUnitsComplete = true
		m.SourceUnits = map[string]string{"goto-source.c": hash}
		mapFixtureLoop(m, source, hash)
		must(hir.Verify(m))
		return m
	}
	original := factory()
	write("fixture-loop-origin.json", mapFixtureLoop(original, source, hash))
	sites, siteErr := hir.BuildLoopSiteMap(original)
	must(siteErr)
	write("loop-sites.json", sites)
	// Only the explicitly modeled static is permitted. Compiler-generated strings
	// are in the separate string pool; they are not silently accepted as globals.
	if len(original.Globals) != 1 || original.Globals[0].Name != "static_shadow__x" || mir2.ByteWidth(original.Globals[0].Ty) != 2 {
		panic("unexpected global layout: extend independent state oracle")
	}
	expectedSnapshot := hir.ProfileMemorySnapshot{Globals: map[string][]byte{"static_shadow__x": {7, 0}}, Resources: map[string][]byte{}}
	raw, err := hir.RunLoopProfile(original, w)
	must(err)
	if len(raw.Results) != len(wants) || len(raw.Snapshots) != len(wants) || len(raw.Accesses) != len(wants) {
		panic("missing raw observations")
	}
	for i, want := range wants {
		if len(raw.Results[i]) != 1 || raw.Results[i][0].I != want || !reflect.DeepEqual(raw.Snapshots[i], expectedSnapshot) {
			panic(fmt.Sprintf("raw mathematical/state oracle call %d", i))
		}
		reads := []uint64{0, 0}
		if w.Calls[i].Function == "static_shadow" {
			reads = []uint64{1, 1}
		}
		expectedAccess := hir.ProfileAccessSnapshot{Globals: map[string]hir.ProfileByteAccess{"static_shadow__x": {Reads: reads, Writes: []uint64{0, 0}}}, Resources: map[string]hir.ProfileByteAccess{}}
		if !reflect.DeepEqual(raw.Accesses[i], expectedAccess) {
			panic(fmt.Sprintf("raw independent traffic oracle call %d: %#v", i, raw.Accesses[i]))
		}
	}
	write("raw.json.gz", raw)
	write("oracle.json", map[string]any{"calls": descriptions, "snapshot_each_call": expectedSnapshot, "traffic": "Static x: two byte reads only in static_shadow; zero reads in other calls, zero writes in every call; no input resources."})
	rows := []map[string]any{}
	for _, grace := range []bool{false, true} {
		m := factory()
		steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "all", UseHIRGrace: grace})
		must(err)
		prefix := fmt.Sprintf("grace-%t", grace)
		write(prefix+".a80.gz", []byte(steps.Assembly))
		write(prefix+".hir.gz", []byte(steps.HIR))
		write(prefix+"-raw.mir.gz", []byte(steps.MIR2Raw))
		write(prefix+"-optimized.mir.gz", []byte(steps.MIR2Opt))
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		must(err)
		if assembled == nil || len(assembled.Errors) > 0 {
			panic(assembled)
		}
		write(prefix+".bin", assembled.Binary)
		run, err := pipeline.RunZ80ProfileWorkload(steps, m, w, *raw)
		must(err)
		if len(run.Invocations) != len(wants) {
			panic("missing target observations")
		}
		for i, want := range wants {
			inv := run.Invocations[i]
			if len(inv.Results) != 1 || inv.Results[0].I != want || !reflect.DeepEqual(inv.Snapshot, expectedSnapshot) || inv.Access == nil || !reflect.DeepEqual(*inv.Access, raw.Accesses[i]) {
				panic(fmt.Sprintf("target mathematical/state/traffic oracle call %d", i))
			}
		}
		binaryHash := fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))
		if binaryHash != run.BinarySHA256 {
			panic("binary digest disagrees with target runner")
		}
		write(prefix+"-target.json.gz", run)
		rows = append(rows, map[string]any{"grace": grace, "t_states": run.TotalTStates, "bytes": len(assembled.Binary), "binary_sha256": binaryHash, "source_sha256": hash, "results": wants, "calls": w.Calls})
	}
	write("costs.json", rows)
	b, err := json.MarshalIndent(rows, "", "  ")
	must(err)
	fmt.Println(string(b))
}
