package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in read-only corpus audit: emitted diagnostic artifacts, no production edits.
func TestCheckedTableNegativeCorpusAudit(t *testing.T) {
	dir := os.Getenv("TABLE_NEGATIVE_EXPORT")
	if dir == "" {
		t.Skip("set TABLE_NEGATIVE_EXPORT")
	}
	solver := checkedRealSolver(t)
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	type row struct {
		HistoricalHit       bool
		HistoricalRawHit    bool
		HistoricalAdmission string

		Name                          string
		Blocks                        int
		Params                        any
		Returns                       any
		Ops                           []string
		Reason                        string
		BaselineBytes, CandidateBytes int
		CandidateCost                 int
		Selected                      bool
		SolveMS                       float64
	}
	type sample struct {
		File                                  string
		BaselineBytes, SelectedBytes, Entries int
		CompileMS, GenerateMS, SelectedMS     float64
		Notes                                 []string
		Functions                             []row
	}
	historical := &vir.RegAllocTable{}
	historical.Init()
	if e := historical.Load("../../../regalloc_table.json"); e != nil {
		t.Fatal(e)
	}
	if e := historical.LoadExhaustive("../vir/regalloc_exhaustive.json"); e != nil {
		t.Fatal(e)
	}
	var samples []sample
	for _, name := range []string{"06_pbqp_weighted", "09_function_pointers", "tiny", "20_strref_test", "21_hashmap_test", "17_bool_retflag", "16_rotate_sled", "promoted_arithmetic", "signed_arithmetic", "08_arena_allocator", "50_impl_showcase", "self_tokenizer"} {
		src, e := os.ReadFile("../../../examples/nanz/" + name + ".nanz")
		if e != nil {
			t.Fatal(e)
		}
		d := filepath.Join(dir, name)
		os.MkdirAll(d, 0755)
		start := time.Now()
		base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
		if e != nil {
			t.Fatal(name, e)
		}
		sample := sample{File: name, CompileMS: float64(time.Since(start).Microseconds()) / 1000}
		sample.BaselineBytes, e = checkedAsmSize(base.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		start = time.Now()
		table, notes, e := GenerateCheckedLocalTables(graceMeasureParse(t, string(src)), opts, vir.SolverOptions{Z3Path: solver})
		if e != nil {
			t.Fatal(e)
		}
		sample.GenerateMS = float64(time.Since(start).Microseconds()) / 1000
		sample.Entries = table.Size()
		sample.Notes = notes
		table.Save(filepath.Join(d, "table.json"))
		use := opts
		use.CheckedLocalTablePath = filepath.Join(d, "table.json")
		start = time.Now()
		selected, e := CompileHIRSteps(graceMeasureParse(t, string(src)), use)
		if e != nil {
			t.Fatal(e)
		}
		sample.SelectedMS = float64(time.Since(start).Microseconds()) / 1000
		sample.SelectedBytes, e = checkedAsmSize(selected.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		for _, f := range base.MIR2Module.Funcs {
			r := row{Name: f.Name, Blocks: len(f.Blocks), Params: f.Contract.Params, Returns: f.Contract.Returns, BaselineBytes: -1, CandidateBytes: -1}
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					r.Ops = append(r.Ops, i.Op.String())
				}
			}
			start, end, ok := checkedFunctionSpan(base.Assembly, f.Name)
			if ok {
				r.BaselineBytes, e = checkedAsmSize(base.Assembly[start:end])
				if e != nil {
					r.BaselineBytes = -1
				}
			}
			p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
			if e != nil {
				r.Reason = "prepare: " + e.Error()
			} else {
				rawSig := vir.ComputeSignature(p.Ops, vir.Z80)
				_, r.HistoricalRawHit = historical.LookupByKey(rawSig)
				old, _, hit := historical.Lookup(p.Ops, vir.Z80)
				r.HistoricalHit = hit
				if hit {
					checked := &vir.RegAllocTable{}
					checked.Init()
					if e := checked.AddChecked(p.Ops, old, vir.Z80, p.Options); e != nil {
						r.HistoricalAdmission = e.Error()
					} else {
						r.HistoricalAdmission = "admitted under exact target"
					}
				}
				local := &vir.RegAllocTable{}
				local.Init()
				begin := time.Now()
				e = p.Generate(local, vir.SolverOptions{Z3Path: solver})
				r.SolveMS = float64(time.Since(begin).Microseconds()) / 1000
				if e != nil {
					r.Reason = "generate: " + e.Error()
				} else {
					a, c, hit := local.LookupChecked(p.Ops, vir.Z80, p.Options)
					_ = a
					if !hit {
						t.Fatal("own admitted table missed")
					}
					r.CandidateCost = c
					asm, e := p.Emit(local)
					if e != nil {
						t.Fatal(e)
					}
					r.CandidateBytes, e = checkedAsmSize(asm)
					if e != nil {
						t.Fatal(e)
					}
					os.WriteFile(filepath.Join(d, f.Name+"-candidate.a80"), []byte(asm), 0644)
					if r.CandidateBytes >= r.BaselineBytes {
						r.Reason = "admitted: not strictly smaller"
					} else {
						r.Reason = "admitted: smaller"
					}
				}
			}
			r.Selected = selected.Traces[f.Name].Backend == "checked-local-table"
			sample.Functions = append(sample.Functions, r)
		}
		for n, b := range map[string][]byte{"source.nanz": src, "baseline.a80": []byte(base.Assembly), "selected.a80": []byte(selected.Assembly), "final.mir2": []byte(base.MIR2Module.Dump())} {
			os.WriteFile(filepath.Join(d, n), b, 0644)
		}
		samples = append(samples, sample)
		t.Logf("%s module%d→%d entries%d compile%.2fms generate%.2fms selected%.2fms", name, sample.BaselineBytes, sample.SelectedBytes, sample.Entries, sample.CompileMS, sample.GenerateMS, sample.SelectedMS)
	}
	b, e := json.MarshalIndent(samples, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "audit.json"), b, 0644)
	for _, s := range samples {
		for _, r := range s.Functions {
			if r.CandidateBytes >= 0 {
				t.Log(fmt.Sprintf("%s/%s baseline%d candidate%d cost%d %s", s.File, r.Name, r.BaselineBytes, r.CandidateBytes, r.CandidateCost, r.Reason))
			}
		}
	}
}
