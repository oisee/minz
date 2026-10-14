package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func layoutModeTestState(t *testing.T) {
	hirProfileTestState(t)
	f, r, l, d, w, cm, cr := hirProfileLayoutFunction, hirProfileLayoutReplay, hirProfileLayoutLimit, hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileCompilerModes, hirProfileCompilerModesReplay
	t.Cleanup(func() {
		hirProfileLayoutFunction, hirProfileLayoutReplay, hirProfileLayoutLimit, hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileCompilerModes, hirProfileCompilerModesReplay = f, r, l, d, w, cm, cr
	})
	hirProfileLayoutFunction, hirProfileLayoutReplay, hirProfileLayoutLimit = "classify", "", 4
	hirProfileCompilerModes, hirProfileCompilerModesReplay = false, ""
	hirProfileAutoDirectory = filepath.Join(t.TempDir(), "artifacts")
	hirProfileAutoWorkload = "workload.json"
}

const layoutCLISource = `global seen:u8=0
fun classify(c:u8)->u8 {
 if c==32{return 1}
 if c==9{return 1}
 if c==10{return 1}
 if c==13{return 1}
 return 0
}
fun caller(c:u8)->u8 {
 let result:u8=classify(c)
 seen=result
 return c+result
}`

func layoutCLIWorkload(values []int64) hir.ProfileWorkload {
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Globals: map[string][]byte{"seen": {255}}, Resources: map[string][]byte{}}
	for _, value := range values {
		v := value
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "caller", Arguments: []hir.ProfileArgument{{Value: &v}}})
	}
	return w
}
func TestLayoutModeCLICollectReplayExactOutput(t *testing.T) {
	layoutModeTestState(t)
	root := t.TempDir()
	source := filepath.Join(root, "source.nanz")
	train := filepath.Join(root, "train.json")
	held := filepath.Join(root, "heldout.json")
	if e := os.WriteFile(source, []byte(layoutCLISource), 0644); e != nil {
		t.Fatal(e)
	}
	for path, w := range map[string]hir.ProfileWorkload{train: layoutCLIWorkload([]int64{32, 65, 9, 0, 255}), held: layoutCLIWorkload([]int64{10, 13, 66, 48, 9})} {
		data, e := json.Marshal(w)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, size := range []bool{false, true} {
		for _, ext := range []string{".a80", ".BiN"} {
			optSize = size
			hirProfileLayoutFunction = "classify"
			hirProfileLayoutReplay = ""
			hirProfileAutoWorkload = train
			name := ext
			if size {
				name += "-size"
			}
			dir := filepath.Join(root, name)
			if e := os.Mkdir(dir, 0755); e != nil {
				t.Fatal(e)
			}
			hirProfileAutoDirectory = filepath.Join(dir, "collect")
			outputFile = filepath.Join(dir, "output"+ext)
			if e := compile(source); e != nil {
				t.Fatal(e)
			}
			saved, e := readLayoutModeArtifacts(hirProfileAutoDirectory)
			if e != nil {
				t.Fatal(e)
			}
			var receipt layoutModeReceipt
			if e = json.Unmarshal(saved["selection.json"], &receipt); e != nil {
				t.Fatal(e)
			}
			goal := "speed"
			if size {
				goal = "size"
			}
			if receipt.Evidence.Goal != goal || receipt.SizePolicy != size {
				t.Fatal("wrong objective")
			}
			expected := saved["selected.a80"]
			if strings.EqualFold(ext, ".bin") {
				expected = saved["selected.bin"]
			}
			out, e := os.ReadFile(outputFile)
			if e != nil || !bytes.Equal(out, expected) {
				t.Fatal("collect -o not selected artifact", e)
			}
			training := hirProfileAutoDirectory
			hirProfileLayoutFunction = ""
			hirProfileLayoutReplay = training
			optSize = false
			hirProfileAutoWorkload = held
			hirProfileAutoDirectory = filepath.Join(dir, "replay")
			outputFile = filepath.Join(dir, "replayed"+ext)
			if e = compile(source); e != nil {
				t.Fatal(e)
			}
			out, e = os.ReadFile(outputFile)
			if e != nil || !bytes.Equal(out, expected) {
				t.Fatal("replay -o changed saved winner", e)
			}
			data, e := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "run.json"))
			if e != nil {
				t.Fatal(e)
			}
			var run pipeline.Z80ProfileWorkloadEvidence
			if e = json.Unmarshal(data, &run); e != nil {
				t.Fatal(e)
			}
			for i, value := range []byte{10, 13, 66, 48, 9} {
				flag := byte(0)
				if value == 10 || value == 13 || value == 9 {
					flag = 1
				}
				inv := run.Invocations[i]
				if byte(inv.Results[0].I) != value+flag || !bytes.Equal(inv.Snapshot.Globals["seen"], []byte{flag}) {
					t.Fatal("independent heldout result/state", i)
				}
			}
		}
	}
}
func TestLayoutModeCLIRejectsFlagsAndAliases(t *testing.T) {
	layoutModeTestState(t)
	source := filepath.Join(t.TempDir(), "source.nanz")
	if e := os.WriteFile(source, []byte(layoutCLISource), 0644); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"both", "limit", "compiler-modes", "runtime", "output-alias", "goal-replay"} {
		t.Run(kind, func(t *testing.T) {
			r, l, cm, rt, out, size := hirProfileLayoutReplay, hirProfileLayoutLimit, hirProfileCompilerModes, runtimeMode, outputFile, optSize
			t.Cleanup(func() {
				hirProfileLayoutReplay, hirProfileLayoutLimit, hirProfileCompilerModes, runtimeMode, outputFile, optSize = r, l, cm, rt, out, size
			})
			switch kind {
			case "both":
				hirProfileLayoutReplay = "saved"
			case "limit":
				hirProfileLayoutLimit = 5
			case "compiler-modes":
				hirProfileCompilerModes = true
			case "runtime":
				runtimeMode = "cpm"
			case "output-alias":
				outputFile = source
			case "goal-replay":
				hirProfileLayoutFunction = ""
				hirProfileLayoutReplay = "saved"
				optSize = true
				t.Cleanup(func() { hirProfileLayoutFunction = "classify" })
			}
			if e := validateHIRProfileAuto(source); e == nil {
				t.Fatal("incompatible flags accepted", kind)
			}
		})
	}
}

// A manifest authenticates bytes, not truth. Consistently resealed evidence must
// still reproduce from the current compiler and unchanged source dependencies.
func TestLayoutModeCLIReplayRejectsResealedEvidenceAndImports(t *testing.T) {
	layoutModeTestState(t)
	root := t.TempDir()
	source := filepath.Join(root, "source.nanz")
	library := filepath.Join(root, "library.nanz")
	write := func(path string, data []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	write(source, []byte("import library { advance }\n"+strings.Replace(layoutCLISource, "return c+result", "return advance(c)+result", 1)))
	originalLibrary := []byte("fun advance(n:u8)->u8{return n+7}")
	write(library, originalLibrary)
	workload := filepath.Join(root, "workload.json")
	data, e := json.Marshal(layoutCLIWorkload([]int64{32, 65, 9}))
	if e != nil {
		t.Fatal(e)
	}
	write(workload, data)
	hirProfileAutoWorkload = workload
	hirProfileAutoDirectory = filepath.Join(root, "training")
	// Imported files are inputs too: failed admission must not overwrite one.
	outputFile = library
	if e = compile(source); e == nil {
		t.Fatal("import output alias accepted")
	}
	unchanged, e := os.ReadFile(library)
	if e != nil || !bytes.Equal(unchanged, originalLibrary) {
		t.Fatal("import overwritten", e)
	}
	if _, e = os.Lstat(hirProfileAutoDirectory); !os.IsNotExist(e) {
		t.Fatal("failed admission published directory", e)
	}
	outputFile = ""
	if e = compile(source); e != nil {
		t.Fatal(e)
	}
	saved, e := readLayoutModeArtifacts(hirProfileAutoDirectory)
	if e != nil {
		t.Fatal(e)
	}
	var mapping map[string]string
	if e = json.Unmarshal(saved["source-paths.json"], &mapping); e != nil || len(mapping) != 2 {
		t.Fatal("missing imported source", e, mapping)
	}
	for _, kind := range []string{"forged-timing", "forged-assembly", "duplicate-json", "changed-import"} {
		t.Run(kind, func(t *testing.T) {
			files := map[string][]byte{}
			for name, data := range saved {
				files[name] = append([]byte(nil), data...)
			}
			var receipt layoutModeReceipt
			if e := json.Unmarshal(files["selection.json"], &receipt); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "forged-timing":
				// Forge an unselected candidate as well as its internally consistent sum.
				row := receipt.Evidence.Runs[len(receipt.Evidence.Runs)-1]
				row.Candidates[0].Run.TotalTStates++
				row.Candidates[0].Run.Invocations[0].TStates++
			case "forged-assembly":
				winner := receipt.Evidence.Runs[receipt.Evidence.SelectedMode]
				winner.Candidates[winner.Selected].Assembly += "\n; forged\n"
				files["selected.a80"] = []byte(winner.Candidates[winner.Selected].Assembly)
			case "changed-import":
				write(library, []byte("fun advance(n:u8)->u8{return n+8}"))
				t.Cleanup(func() { write(library, originalLibrary) })
			}
			if kind != "duplicate-json" {
				// Reseal all nested evidence digests and the complete manifest.
				for _, run := range receipt.Evidence.Runs {
					run.SHA256 = ""
					raw, e := json.Marshal(run)
					if e != nil {
						t.Fatal(e)
					}
					run.SHA256 = compilerModeReplayHash(raw)
				}
				receipt.Evidence.SHA256 = ""
				raw, e := json.Marshal(receipt.Evidence)
				if e != nil {
					t.Fatal(e)
				}
				receipt.Evidence.SHA256 = compilerModeReplayHash(raw)
				files["selection.json"], e = json.Marshal(receipt)
				if e != nil {
					t.Fatal(e)
				}
			} else {
				// The loader must reject duplicate keys even with a matching file digest.
				files["selection.json"] = append([]byte(`{"schema":"forged",`), files["selection.json"][1:]...)
			}
			manifest := map[string]string{}
			for name, data := range files {
				if name != "manifest.json" {
					manifest[name] = compilerModeReplayHash(data)
				}
			}
			files["manifest.json"], e = json.Marshal(manifest)
			if e != nil {
				t.Fatal(e)
			}
			training := filepath.Join(root, kind)
			for name, data := range files {
				write(filepath.Join(training, name), data)
			}
			// This proves the failure is deeper than a missing file or bad SHA256.
			if _, e = readLayoutModeArtifacts(training); e != nil {
				t.Fatal("resealed fixture not readable", e)
			}
			hirProfileLayoutFunction = ""
			hirProfileLayoutReplay = training
			hirProfileAutoDirectory = filepath.Join(root, kind+"-out")
			outputFile = filepath.Join(root, kind+".bin")
			write(outputFile, []byte("preserve existing output"))
			e = compile(source)
			if e == nil {
				t.Fatal("mutated training accepted", kind)
			}
			if (kind == "forged-timing" || kind == "forged-assembly") && !strings.Contains(e.Error(), "cannot be reproduced") {
				t.Fatal("did not reach reproduction", e)
			}
			if _, e = os.Lstat(hirProfileAutoDirectory); !os.IsNotExist(e) {
				t.Fatal("failure published directory", e)
			}
			out, e := os.ReadFile(outputFile)
			if e != nil || string(out) != "preserve existing output" {
				t.Fatal("failure replaced output", e)
			}
		})
	}
}
