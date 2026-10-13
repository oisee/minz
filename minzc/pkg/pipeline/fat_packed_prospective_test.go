package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fatPackedCheckIndependent(t *testing.T, w hir.ProfileWorkload, results [][]mir2.Value, snapshots []hir.ProfileMemorySnapshot) {
	t.Helper()
	if len(results) != len(w.Calls) || len(snapshots) != len(w.Calls) {
		t.Fatal("completeinvocationsrequired")
	}
	resources := map[string][]byte{}
	for name, b := range w.Resources {
		resources[name] = append([]byte(nil), b...)
	}
	for i, call := range w.Calls {
		name := call.Arguments[0].Resource
		clst := int(*call.Arguments[1].Value)
		ofs := clst * 2
		fat12 := strings.HasSuffix(call.Function, "12")
		if fat12 {
			ofs = clst + clst/2
		}
		b := resources[name]
		raw := uint16(b[ofs]) | uint16(b[ofs+1])<<8
		writing := strings.HasPrefix(call.Function, "write")
		if writing {
			val := uint16(*call.Arguments[2].Value)
			next := val
			if fat12 {
				if clst&1 != 0 {
					next = raw&15 | val<<4
				} else {
					next = raw&0xf000 | val&0xfff
				}
			}
			b[ofs], b[ofs+1] = byte(next), byte(next>>8)
			if len(results[i]) != 0 {
				t.Fatal("voidwrite")
			}
		} else {
			want := raw
			if fat12 {
				if clst&1 != 0 {
					want = raw >> 4
				} else {
					want = raw & 0xfff
				}
			}
			if len(results[i]) != 1 || results[i][0].I != int64(want) {
				t.Fatal("independentpackedword", i, call, results[i], want)
			}
		}
		for resource, want := range resources {
			if !bytes.Equal(snapshots[i].Resources[resource], want) {
				t.Fatal("independentfullresource/nibbleoracle", i, resource)
			}
		}
	}
}
func TestFATPackedProspectiveMeasuredModes(t *testing.T) {
	const srcpath = "../../../stdlib/fs/fat12.nanz"
	const frozen = "../../../reports/fat-packed-prospective"
	src, e := os.ReadFile(srcpath)
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SourceSHA256 string `json:"source_sha256"`
	}
	b, e := os.ReadFile(frozen + "/workload.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &freeze); e != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.SourceSHA256 {
		t.Fatal("frozen source changed", e)
	}
	load := func(name string) hir.ProfileWorkload {
		b, e := os.ReadFile(frozen + "/" + name + "-controlled.json")
		if e != nil {
			t.Fatal(e)
		}
		var w hir.ProfileWorkload
		if e = json.Unmarshal(b, &w); e != nil {
			t.Fatal(e)
		}
		return w
	}
	train, heldout := load("training"), load("heldout")
	factory := func() (*hir.Module, error) { return nanz.Parse(string(src), srcpath) }
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_FAT_PACKED_EXPORT")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var data []byte
		switch x := v.(type) {
		case string:
			data = []byte(x)
		case []byte:
			data = x
		default:
			var e error
			data, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e := os.WriteFile(p, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", src)
	save("training.json", train)
	save("heldout.json", heldout)
	for _, goal := range []string{"speed", "size"} {
		result, e := MeasureCompilerModeVariants(factory, train, goal, false)
		if e != nil {
			t.Fatal("full-source measured mode admission/execution", goal, e)
		}
		fatPackedCheckIndependent(t, train, result.Evidence.Oracle.Results, result.Evidence.Oracle.Snapshots)
		for i, c := range result.Evidence.Candidates {
			rows := [][]mir2.Value{}
			snap := []hir.ProfileMemorySnapshot{}
			for _, v := range c.Run.Invocations {
				rows = append(rows, v.Results)
				snap = append(snap, v.Snapshot)
			}
			fatPackedCheckIndependent(t, train, rows, snap)
			save(fmt.Sprintf("%s/candidate%d.a80", goal, i), c.Assembly)
			save(fmt.Sprintf("%s/candidate%d.bin", goal, i), c.Binary)
			save(fmt.Sprintf("%s/candidate%d.mir", goal, i), c.FinalMIR)
			t.Logf("%s candidate%d contract%v grace%v %dT %dB", goal, i, c.Options.ContractOpt, c.Options.UseGrace, c.Run.TotalTStates, len(c.Binary))
		}
		fresh, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		oracle, e := hir.RunLoopProfile(fresh, heldout)
		if e != nil {
			t.Fatal(e)
		}
		fatPackedCheckIndependent(t, heldout, oracle.Results, oracle.Snapshots)
		run, e := RunZ80ProfileWorkload(result.Steps, fresh, heldout, *oracle)
		if e != nil {
			t.Fatal(e)
		}
		rows := [][]mir2.Value{}
		snap := []hir.ProfileMemorySnapshot{}
		for _, v := range run.Invocations {
			rows = append(rows, v.Results)
			snap = append(snap, v.Snapshot)
		}
		fatPackedCheckIndependent(t, heldout, rows, snap)
		if run.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(result.Binary)) {
			t.Fatal("heldout changedselectedexecutable")
		}
		save(goal+"/evidence.json", result.Evidence)
		save(goal+"/heldout-run.json", run)
		save(goal+"/selected.bin", result.Binary)
		save(goal+"/selected.a80", result.Steps.Assembly)
		t.Logf("%s selected%d sameBIN heldout%d calls %dT", goal, result.Evidence.Selected, len(run.Invocations), run.TotalTStates)
	}
}
