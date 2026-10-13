package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type indexedCoverageAttempt struct {
	Method, Asset, Error string
	Evidence             *vir.IndexedLegacyImportEvidence
}
type indexedCoverageRow struct {
	Name, PrepareError, Result                                          string
	Attempts                                                            []indexedCoverageAttempt
	BaselineBytes, CandidateBytes, NativeSamples, BaselineT, CandidateT int
}
type indexedCoverageSample struct {
	File                                  string
	BaselineBytes, SelectedBytes, Entries int
	ImportMS                              float64
	Functions                             []indexedCoverageRow
}

func TestIndexedShapeCorpusCoverage(t *testing.T) {
	dir := os.Getenv("INDEXED_COVERAGE_EXPORT")
	if dir == "" {
		t.Skip("set INDEXED_COVERAGE_EXPORT")
	}
	os.MkdirAll(dir, 0755)
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	names := []string{"06_pbqp_weighted", "09_function_pointers", "tiny", "20_strref_test", "21_hashmap_test", "17_bool_retflag", "16_rotate_sled", "promoted_arithmetic", "signed_arithmetic", "08_arena_allocator", "50_impl_showcase", "self_tokenizer"}
	samples := []indexedCoverageSample{}
	for _, name := range names {
		src, e := os.ReadFile("../../../examples/nanz/" + name + ".nanz")
		if e != nil {
			t.Fatal(e)
		}
		base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
		if e != nil {
			t.Fatal(name, e)
		}
		raw := mir2.NewVM(hir.LowerModule(graceMeasureParse(t, string(src))))
		d := filepath.Join(dir, name)
		os.MkdirAll(d, 0755)
		tab := &vir.RegAllocTable{}
		tab.Init()
		sample := indexedCoverageSample{File: name}
		sample.BaselineBytes, e = checkedAsmSize(base.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		startTime := time.Now()
		for _, f := range base.MIR2Module.Funcs {
			row := indexedCoverageRow{Name: f.Name, BaselineBytes: -1, CandidateBytes: -1}
			p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
			if e != nil {
				row.PrepareError = e.Error()
				sample.Functions = append(sample.Functions, row)
				continue
			}
			for _, method := range []string{"direct", "quotient"} {
				for _, n := range []string{"4v", "5v"} {
					path := "/mnt/safe/z80-compiler/tables/enriched_" + n + ".enr"
					var ev *vir.IndexedLegacyImportEvidence
					if method == "direct" {
						ev, e = vir.ImportIndexedENRT(path, p, tab)
					} else {
						ev, e = vir.ImportIndexedENRTQuotient(path, p, tab)
					}
					attempt := indexedCoverageAttempt{Method: method, Asset: n, Evidence: ev}
					if e != nil {
						attempt.Error = e.Error()
					}
					row.Attempts = append(row.Attempts, attempt)
				}
			}
			if _, _, hit := tab.LookupChecked(p.Ops, vir.Z80, p.Options); !hit {
				row.Result = "no admitted assignment"
				sample.Functions = append(sample.Functions, row)
				continue
			}
			candidate, e := p.Emit(tab)
			if e != nil {
				t.Fatal(name, f.Name, e)
			}
			lo, hi, ok := checkedFunctionSpan(base.Assembly, f.Name)
			if !ok {
				t.Fatal("missing span", f.Name)
			}
			row.BaselineBytes, e = checkedAsmSize(base.Assembly[lo:hi])
			if e != nil {
				t.Fatal(e)
			}
			row.CandidateBytes, e = checkedAsmSize(candidate)
			if e != nil {
				t.Fatal(e)
			}
			row.Result = "equal"
			if row.CandidateBytes < row.BaselineBytes {
				row.Result = "smaller"
			} else if row.CandidateBytes > row.BaselineBytes {
				row.Result = "larger"
			}
			// Execute even equal/nonselected admitted proposals, inside complete original
			// module. Fresh raw-HIR VM is independent of table emission and allocation.
			row.NativeSamples, row.BaselineT, row.CandidateT = indexedCoverageNative(t, base, raw, f, base.Assembly[:lo]+candidate+base.Assembly[hi:])
			os.WriteFile(filepath.Join(d, f.Name+"-candidate.a80"), []byte(candidate), 0644)
			sample.Functions = append(sample.Functions, row)
		}
		sample.ImportMS = float64(time.Since(startTime).Microseconds()) / 1000
		sample.Entries = tab.Size()
		tablePath := filepath.Join(d, "table.json")
		if e = tab.Save(tablePath); e != nil {
			t.Fatal(e)
		}
		use := opts
		use.CheckedLocalTablePath = tablePath
		selected, e := CompileHIRSteps(graceMeasureParse(t, string(src)), use)
		if e != nil {
			t.Fatal(e)
		}
		sample.SelectedBytes, e = checkedAsmSize(selected.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		for fn, data := range map[string][]byte{"source.nanz": src, "baseline.a80": []byte(base.Assembly), "selected.a80": []byte(selected.Assembly), "mir2.txt": []byte(base.MIR2Module.Dump())} {
			os.WriteFile(filepath.Join(d, fn), data, 0644)
		}
		for fn, steps := range map[string]Steps{"baseline": base, "selected": selected} {
			b, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
			if e != nil || len(b.Errors) > 0 {
				t.Fatal(e, b.Errors)
			}
			os.WriteFile(filepath.Join(d, fn+".bin"), b.Binary, 0644)
		}
		samples = append(samples, sample)
		t.Logf("%s %d→%dB entries%d", name, sample.BaselineBytes, sample.SelectedBytes, sample.Entries)
	}
	data, _ := json.MarshalIndent(samples, "", "  ")
	os.WriteFile(filepath.Join(dir, "coverage.json"), data, 0644)
}

func indexedCoverageNative(t *testing.T, base Steps, raw *mir2.VM, f *mir2.Func, candidate string) (int, int, int) {
	t.Helper()
	samples := 1
	if len(f.Contract.Params) > 0 {
		samples = 65536
	}
	if len(f.Contract.Params) == 1 && f.Contract.Params[0].Ty.Width() == 8 {
		samples = 256
	}
	costs := [2]int{}
	for pass, asm := range []string{base.Assembly, candidate} {
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		address := bin.Symbols[f.Name]
		if address == 0 {
			address = bin.Symbols[strings.ToUpper(f.Name)]
		}
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, bin.Binary)
		z.LoadMemory(0xeffe, []byte{0, 0xf1})
		for x := 0; x < samples; x++ {
			values := []mir2.Value{}
			for i, p := range f.Contract.Params {
				v := x
				if i == 1 {
					v = x >> 8
				} else if i > 1 {
					v = x*53 + 17
				}
				if p.Ty.Width() == 8 {
					v &= 255
				}
				values = append(values, mir2.Value{I: int64(v)})
			}
			want, e := raw.Call(f.Name, values)
			if e != nil || len(want) != 1 {
				t.Fatal("raw oracle", f.Name, e, want)
			}
			regs := emulator.Registers{A: 0x77, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(address), SP: 0xeffe}
			z.SetRegisters(regs)
			for i, p := range f.Contract.Params {
				loc := base.Allocation.Locs[p.Reg].Name
				v := uint16(values[i].I)
				if p.Ty.Width() == 8 {
					if e = z.SetRegister8(loc, byte(v)); e != nil {
						t.Fatal(e)
					}
				} else {
					r := z.GetRegisters()
					switch loc {
					case "HL":
						r.HL = v
					case "DE":
						r.DE = v
					case "BC":
						r.BC = v
					default:
						t.Fatal("unsupported input", loc)
					}
					z.SetRegisters(r)
				}
			}
			before := z.GetCycles()
			for steps := 0; steps < 256 && z.GetRegisters().PC != 0xf100; steps++ {
				z.Step()
			}
			r := z.GetRegisters()
			ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
			got := uint16(0)
			switch ret {
			case "A":
				got = uint16(r.A)
			case "C":
				got = r.BC & 255
			case "B":
				got = r.BC >> 8
			case "HL":
				got = r.HL
			case "DE":
				got = r.DE
			case "BC":
				got = r.BC
			default:
				t.Fatal("unsupported return", ret)
			}
			if got != uint16(want[0].I) || r.SP != 0xf000 || r.PC != 0xf100 {
				t.Fatal(f.Name, x, ret, got, want, r)
			}
			cost := z.GetCycles() - before
			if x == 0 {
				costs[pass] = cost
			} else if costs[pass] != cost {
				t.Fatal("pure leaf cost varies", f.Name)
			}
		}
		if !bytes.Equal(z.Memory()[0x8000:0x8000+len(bin.Binary)], bin.Binary) {
			t.Fatal("image mutation", f.Name)
		}
	}
	t.Logf("native %s %dsamples %d→%dT", f.Name, samples, costs[0], costs[1])
	return samples, costs[0], costs[1]
}
