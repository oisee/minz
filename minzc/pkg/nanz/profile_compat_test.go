package nanz_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func profileHash(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

func TestLoopProfileCompatibilityAndRefusals(t *testing.T) {
	src := `global seed:u8=3
fun test(n:u8)->u8 {while n>0 {n=n-1}
 return seed}`
	parse := func() *hir.Module {
		h, e := nanz.Parse(src, "compat.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	h := parse()
	context := hir.ProfileContext{CompilerID: "test-compiler-1", SemanticsID: "mir2-vm-1", TargetID: "z80-cpm-1", WorkloadSHA256: profileHash("test(3)"), HostsSHA256: profileHash("no hosts"), Dependencies: map[string]string{"compat.nanz": profileHash(src), "external.bin": profileHash("external data")}}
	p, e := hir.NewLoopProfilerWithContext(h, context)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Call("test", []mir2.Value{{I: 3}}); e != nil {
		t.Fatal(e)
	}
	snapshot := p.Snapshot()
	encoded, e := json.Marshal(snapshot)
	if e != nil {
		t.Fatal(e)
	}
	load := func(data []byte, m *hir.Module, c hir.ProfileContext) error {
		_, e := hir.LoadLoopProfile(bytes.NewReader(data), m, c)
		return e
	}
	if e := load(encoded, parse(), context); e != nil {
		t.Fatal(e)
	}
	key, e := hir.BuildProfileCompatibility(parse(), context)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(key)
	b, _ := json.Marshal(snapshot.Compatibility)
	if !bytes.Equal(a, b) {
		t.Fatal("fingerprint nondeterministic")
	}
	t.Log(string(encoded))
	for _, field := range []string{"compiler", "semantics", "target", "workload", "hosts", "dependency"} {
		t.Run(field, func(t *testing.T) {
			c := context
			c.Dependencies = map[string]string{}
			for n, h := range context.Dependencies {
				c.Dependencies[n] = h
			}
			switch field {
			case "compiler":
				c.CompilerID += "changed"
			case "semantics":
				c.SemanticsID += "changed"
			case "target":
				c.TargetID += "changed"
			case "workload":
				c.WorkloadSHA256 = profileHash("other")
			case "hosts":
				c.HostsSHA256 = profileHash("other")
			case "dependency":
				c.Dependencies["external.bin"] = profileHash("other")
			}
			if e := load(encoded, h, c); e == nil {
				t.Fatal("stale context accepted")
			}
		})
	}
	// Same source origin, but changed typed HIR, initializer or ABI: refuse.
	for _, edit := range []func(*hir.Module){
		func(m *hir.Module) {
			m.FuncByName("test").Body.Body[0].(*hir.WhileStmt).Cond = &hir.BoolLitExpr{Val: false}
		},
		func(m *hir.Module) { m.Globals[0].Init = []byte{9} },
		func(m *hir.Module) { m.FuncByName("test").Params[0].SMC = true },
		func(m *hir.Module) { m.Target = hir.TargetZXSpectrum },
	} {
		m := parse()
		edit(m)
		if e := load(encoded, m, context); e == nil {
			t.Fatal("changed semantic HIR accepted")
		}
	}
	for _, name := range []string{"old-schema", "wrong-stage", "no-key", "failed", "empty", "missing-count", "duplicate-count", "unknown-site", "impossible-count", "unmapped", "source-hash"} {
		t.Run(name, func(t *testing.T) {
			var q hir.LoopProfile
			if e := json.Unmarshal(encoded, &q); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "old-schema":
				q.Schema = "minz.input-hir-loop-observations/v1"
			case "wrong-stage":
				q.Stage = "optimised-mir"
			case "no-key":
				q.Compatibility = nil
			case "failed":
				q.FailedCalls = 1
			case "empty":
				q.CompletedCalls = 0
			case "missing-count":
				q.Counts = nil
			case "duplicate-count":
				q.Counts = append(q.Counts, q.Counts[0])
			case "unknown-site":
				q.Counts[0].SiteID = "unknown"
			case "impossible-count":
				q.Counts[0].Entries = 0
			case "unmapped":
				q.Sites.UnmappedLoops = 1
			case "source-hash":
				q.Sites.Sites[0].Origin.SourceSHA256 = profileHash("stale")
			}
			data, _ := json.Marshal(q)
			if e := load(data, h, context); e == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
	for _, data := range []string{
		string(encoded) + ` {}`,
		strings.Replace(string(encoded), `"entries":1`, `"entries":1,"entries":2`, 1),
		strings.Replace(string(encoded), `"entries":1`, `"entries":-1`, 1),
		strings.Replace(string(encoded), `"entries":1`, `"entries":18446744073709551616`, 1),
		strings.Replace(string(encoded), `"entries":1,`, ``, 1),
		strings.Replace(string(encoded), `"failed_calls":0`, `"failed_calls":null`, 1),
		strings.Replace(string(encoded), `"failed_calls":0`, `"failed_calls":0,"unknown":1`, 1),
		strings.Repeat(" ", 8<<20) + string(encoded),
	} {
		if e := load([]byte(data), h, context); e == nil {
			t.Fatal("malformed JSON accepted", data[:min(100, len(data))])
		}
	}
	// Snapshots and supplied context maps must not alias the stored key.
	context.Dependencies["external.bin"] = profileHash("changed")
	snapshot.Compatibility.Context.Dependencies["compat.nanz"] = profileHash("changed")
	if p.Snapshot().Compatibility.Context.Dependencies["compat.nanz"] != profileHash(src) || p.Snapshot().Compatibility.Context.Dependencies["external.bin"] != profileHash("external data") {
		t.Fatal("compatibility key aliased")
	}
}

func TestLoopFreeDependencyProfileInvalidation(t *testing.T) {
	// A dependency with no loops must still be part of the compatibility key.
	src := `fun helper()->u8 {return 3}
fun test(n:u8)->u8 {while n>0 {n=n-1}
 return helper()}`
	m, e := nanz.Parse(src, "deps.nanz")
	if e != nil {
		t.Fatal(e)
	}
	c := hir.ProfileContext{CompilerID: "compiler", SemanticsID: "vm", TargetID: "cpm", WorkloadSHA256: profileHash("test(1)"), HostsSHA256: profileHash("none"), Dependencies: map[string]string{"deps.nanz": profileHash(src)}}
	p, e := hir.NewLoopProfilerWithContext(m, c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Call("test", []mir2.Value{{I: 1}}); e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(p.Snapshot())
	m.FuncByName("helper").Body.Body[0].(*hir.ReturnStmt).Val = &hir.IntLitExpr{Val: 4, Ty: mir2.TyU8}
	if _, e := hir.LoadLoopProfile(bytes.NewReader(data), m, c); e == nil {
		t.Fatal("loop-free dependency body edit accepted")
	}
	delete(c.Dependencies, "deps.nanz")
	if _, e := hir.BuildProfileCompatibility(m, c); e == nil {
		t.Fatal("missing source dependency accepted")
	}
}

func TestImportedProfileOrigins(t *testing.T) {
	dir := t.TempDir()
	dep := `fun helper(n:u8)->u8 {while n>0 {n=n-1}
 return n}`
	if e := os.WriteFile(filepath.Join(dir, "helper.nanz"), []byte(dep), 0600); e != nil {
		t.Fatal(e)
	}
	src := `import helper
fun test(n:u8)->u8 {return helper.helper(n)}`
	m, e := nanz.ParseWithOpts(src, "main.nanz", nanz.ParseOpts{BaseDir: dir})
	if e != nil {
		t.Fatal(e)
	}
	c := hir.ProfileContext{CompilerID: "compiler", SemanticsID: "vm", TargetID: "cpm", WorkloadSHA256: profileHash("test(3)"), HostsSHA256: profileHash("none"), Dependencies: map[string]string{"main.nanz": profileHash(src), "helper": profileHash(dep)}}
	p, e := hir.NewLoopProfilerWithContext(m, c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Call("helper__helper", []mir2.Value{{I: 3}}); e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(p.Snapshot())
	if _, e := hir.LoadLoopProfile(bytes.NewReader(data), m, c); e != nil {
		t.Fatal(e)
	}
	c.Dependencies["helper"] = profileHash(dep + "\n")
	if _, e := hir.LoadLoopProfile(bytes.NewReader(data), m, c); e == nil {
		t.Fatal("stale imported source accepted")
	}
}
