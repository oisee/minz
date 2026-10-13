package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestProfilePinnedCheckedTableMeasuredReplay(t *testing.T) {
	_, path := localTableFixture(t)
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	source := checkedTableSource + `fun sum_array(ptr:^u8,n:u8)->u8 { var s:u8=0 var i:u8=0 while i<n { s=s+ptr[i] i=i+1 } return s }`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "profile-table.nanz") }
	opts := Options{OptSize: true, AssertMode: "none", UseHIRGrace: true, CheckedLocalTablePath: path, CheckedLocalTableSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	id, e := ProfileCompilationID(opts, "default")
	if e != nil {
		t.Fatal(e)
	}
	moved := filepath.Join(t.TempDir(), "relocated.json")
	if e := os.WriteFile(moved, data, 0644); e != nil {
		t.Fatal(e)
	}
	other := opts
	other.CheckedLocalTablePath = moved
	id2, e := ProfileCompilationID(other, "default")
	if e != nil || id2 != id {
		t.Fatal("path-dependent identity", id, id2, e)
	}
	n := int64(40)
	input := make([]byte, 40)
	for i := range input {
		input[i] = byte(i + 1)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-pbqp-default-jrs/v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"input": input}, Calls: []hir.ProfileInvocation{{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &n}}}}}
	model, evidence, e := MeasureLoopCostModel(factory, "sum_array", w, opts)
	if e != nil {
		t.Fatal(e)
	}
	if len(evidence.Rows) != 1022 || model.CompilationID != id {
		t.Fatal("incomplete bound measurement")
	}
	h, e := factory()
	if e != nil {
		t.Fatal(e)
	}
	run, e := hir.RunLoopProfile(h, w)
	if e != nil {
		t.Fatal(e)
	}
	observations, e := json.Marshal(run.Profile)
	if e != nil {
		t.Fatal(e)
	}
	other.ProfileSelection = &ProfileSelection{Observations: observations, Workload: w, Model: model, AssemblerMode: "default"}
	h, e = factory()
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, other)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.ProfileDecisions) != 1 || s.ProfileDecisions[0].Variant != "original" || s.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("Size/table replay", s.ProfileDecisions, s.Traces["add5"])
	}
	for x := 0; x < 256; x++ {
		executeLocalTableFunction(t, s.Assembly, byte(x))
	}
	memory := map[uint16]byte{}
	for i := 0; i < 255; i++ {
		memory[0x4000+uint16(i)] = byte(i + 1)
	}
	for _, length := range []int64{0, 1, 17, 40, 255} {
		h, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		measureProfileZ80(t, h, "sum_array", other, []int64{0x4000, length}, memory, length*(length+1)/2&255, "")
	}
	if e := os.WriteFile(moved, []byte("[]"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := ProfileCompilationID(other, "default"); e == nil {
		t.Fatal("mutated file retained identity")
	}
	h, _ = factory()
	if _, e := CompileHIRSteps(h, other); e == nil {
		t.Fatal("mutated replay accepted")
	}
	other.CheckedLocalTableSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("[]")))
	changedID, e := ProfileCompilationID(other, "default")
	if e != nil || changedID == id {
		t.Fatal("new content identity", changedID, e)
	}
	h, _ = factory()
	if _, e := CompileHIRSteps(h, other); e == nil {
		t.Fatal("old measured model accepted with new table pin")
	}
	if e := os.WriteFile(moved, data, 0644); e != nil {
		t.Fatal(e)
	}
	other.CheckedLocalTableSHA256 = opts.CheckedLocalTableSHA256
	h, _ = factory()
	if asm, e := CompileHIRWithOptions(h, other); e != nil || asm != s.Assembly {
		t.Fatal("alternate profile entry", e)
	}
	other.CheckedLocalTableSHA256 = ""
	if _, e := ProfileCompilationID(other, "default"); e == nil {
		t.Fatal("unpinned profile accepted")
	}
	image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil || len(image.Errors) > 0 {
		t.Fatal(e, image.Errors)
	}
	if dir := os.Getenv("MINZ_PROFILE_TABLE_EXPORT"); dir != "" {
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		for name, v := range map[string]any{"model.json": model, "evidence.json": evidence, "workload.json": w, "profile.json": run.Profile, "decisions.json": s.ProfileDecisions} {
			b, e := json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
		for name, b := range map[string][]byte{"table.json": data, "source.nanz": []byte(source), "selected.a80": []byte("ORG 0x8000\n" + s.Assembly), "selected.bin": image.Binary} {
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Log("1022 measured cases; Size replay retains original loop and checked add5; relocation/mutation controls and native inputs PASS")
}
