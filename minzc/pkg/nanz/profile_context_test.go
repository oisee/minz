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

func TestAutomaticProfileContext(t *testing.T) {
	dir := t.TempDir()
	dep := `fun constant()->u8 {return 7}`
	if err := os.WriteFile(filepath.Join(dir, "helper.nanz"), []byte(dep), 0600); err != nil {
		t.Fatal(err)
	}
	src := `import helper
fun test(n:u8)->u8 {while n>0 {n=n-1}
 return n}`
	parse := func() *hir.Module {
		m, err := nanz.ParseWithOpts(src, "main.nanz", nanz.ParseOpts{BaseDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := parse()
	in := hir.ProfileInputs{Workload: []byte("calls=test(3);memory=64KiB;gas=1000"), Hosts: []byte("hosts=none;ports=none"), Resources: map[string][]byte{"story.bin": {1, 2, 3}}, TargetModel: "cpm-z80-v1"}
	c, err := hir.BuildAutomaticProfileContext(m, in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Dependencies["main.nanz"] != profileHash(src) || c.Dependencies["helper"] != profileHash(dep) || c.Dependencies["story.bin"] != profileHash(string([]byte{1, 2, 3})) {
		t.Fatalf("dependencies=%+v", c)
	}
	// Independently hash the actual executable; no build-info/Git-label fallback.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if c.CompilerID != fmt.Sprintf("executable-sha256:%x", sha256.Sum256(data)) {
		t.Fatal("wrong executable identity")
	}
	again, err := hir.BuildAutomaticProfileContext(parse(), in)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(c)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("context nondeterministic")
	}
	p, err := hir.NewLoopProfilerWithContext(m, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Call("test", []mir2.Value{{I: 3}}); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(p.Snapshot())
	if _, err := hir.LoadLoopProfile(bytes.NewReader(snapshot), parse(), again); err != nil {
		t.Fatal(err)
	}
	t.Log(string(a))
	// Comment-only edit in a loop-free imported unit expires observations even
	// though the semantic HIR and every loop-site identity stay unchanged.
	if err := os.WriteFile(filepath.Join(dir, "helper.nanz"), []byte(dep+"\n// changed"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := parse()
	changedContext, err := hir.BuildAutomaticProfileContext(changed, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hir.LoadLoopProfile(bytes.NewReader(snapshot), changed, changedContext); err == nil {
		t.Fatal("loop-free source edit accepted")
	}
	for _, field := range []string{"workload", "hosts", "resource", "target"} {
		altered := in
		altered.Resources = map[string][]byte{"story.bin": {1, 2, 3}}
		switch field {
		case "workload":
			altered.Workload = []byte("different")
		case "hosts":
			altered.Hosts = []byte("different")
		case "resource":
			altered.Resources["story.bin"] = []byte{9}
		case "target":
			altered.TargetModel = "different"
		}
		x, err := hir.BuildAutomaticProfileContext(m, altered)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := hir.LoadLoopProfile(bytes.NewReader(snapshot), m, x); err == nil {
			t.Fatalf("%s edit accepted", field)
		}
	}
	in.Resources["main.nanz"] = []byte(src)
	if _, err := hir.BuildAutomaticProfileContext(m, in); err == nil {
		t.Fatal("resource/source collision accepted")
	}
	m.SourceUnitsComplete = false
	if _, err := hir.BuildAutomaticProfileContext(m, in); err == nil {
		t.Fatal("incomplete manifest accepted")
	}
	// Host bytes are explicitly descriptors; no claim of hashing a Go closure.
	if !strings.HasPrefix(c.CompilerID, "executable-sha256:") {
		t.Fatal(c.CompilerID)
	}
}

func TestManifestForDeclarationOnlyAndMixedInputs(t *testing.T) {
	dir := t.TempDir()
	for _, file := range []struct {
		name, source string
		complete     bool
	}{
		{"constants.nanz", "const answer:u8=7", true},
		{"constants.lanz", "(fun answer () -> u8 (return 7))", false},
	} {
		t.Run(file.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.source), 0600); err != nil {
				t.Fatal(err)
			}
			// Fresh directory prevents selecting the other extension first.
			isolated := t.TempDir()
			if err := os.WriteFile(filepath.Join(isolated, file.name), []byte(file.source), 0600); err != nil {
				t.Fatal(err)
			}
			src := `import constants
fun test()->u8 {return 0}`
			m, err := nanz.ParseWithOpts(src, "manifest.nanz", nanz.ParseOpts{BaseDir: isolated})
			if err != nil {
				t.Fatal(err)
			}
			if m.SourceUnits["constants"] != profileHash(file.source) || m.SourceUnitsComplete != file.complete {
				t.Fatalf("manifest=%v complete=%v", m.SourceUnits, m.SourceUnitsComplete)
			}
			_, err = hir.BuildAutomaticProfileContext(m, hir.ProfileInputs{TargetModel: "cpm"})
			if file.complete && err != nil {
				t.Fatal(err)
			}
			if !file.complete && err == nil {
				t.Fatal("mixed incomplete frontend accepted")
			}
		})
	}
}
