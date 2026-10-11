package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSeededPointerWalkMeasuredCaller(t *testing.T) {
	sourcePath := "../../../examples/nanz/grace_seeded_sum.nanz"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), sourcePath) }
	bytesIn := make([]byte, 256)
	for i := range bytesIn {
		bytesIn[i] = byte(i*37 + 11)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 2000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"data": bytesIn}, Globals: map[string][]byte{"seeded_calls": {250}, "seeded_last": {0x34, 0x12}}}
	wants := []int64{}
	snaps := []hir.ProfileMemorySnapshot{}
	calls := byte(250)
	for _, seed := range []int64{0, 1, 65535} {
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			seed, n := seed, n
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "seeded_packet", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &n}, {Value: &seed}}})
			sum := uint16(seed)
			for _, b := range bytesIn[:n] {
				sum += uint16(b)
			}
			calls++
			wants = append(wants, int64(sum))
			snaps = append(snaps, hir.ProfileMemorySnapshot{Globals: map[string][]byte{"seeded_calls": {calls}, "seeded_last": {byte(sum), byte(sum >> 8)}}, Resources: w.Resources})
		}
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	if len(hir.AnalyzeFacts(m).PointerWalks) != 1 {
		t.Fatal("seeded source not recognized")
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.Profile.Guards) != 1 {
		t.Fatal("missing seeded guard observations")
	}
	p, _ := json.Marshal(raw.Profile)
	for _, size := range []bool{false, true} {
		r, err := MeasureJointSourceSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Evidence.Candidates) != 4 || len(r.Evidence.EligibleSites) != 0 || len(r.Evidence.EligiblePointerSites) != 1 {
			t.Fatal("incomplete seeded candidates")
		}
		best := 0
		for mask, c := range r.Evidence.Candidates {
			for j, inv := range c.Run.Invocations {
				if inv.Results[0].I != wants[j] || !jointEqual(inv.Snapshot, snaps[j]) {
					t.Fatal("seeded host oracle", mask, j)
				}
				if inv.Access == nil {
					t.Fatal("missing accesses")
				}
				counts := inv.Access.Resources["data"]
				for k := range bytesIn {
					want := uint64(0)
					if k < int(*w.Calls[j].Arguments[1].Value) {
						want = 1
					}
					if counts.Reads[k] != want || counts.Writes[k] != 0 {
						t.Fatal("input byte access", mask, j, k)
					}
				}
			}
			metricBetter := c.TotalTStates < r.Evidence.Candidates[best].TotalTStates
			if size {
				metricBetter = c.ModuleBytes < r.Evidence.Candidates[best].ModuleBytes
			}
			if metricBetter {
				best = mask
			}
			t.Logf("size%v candidate%d %dT/%dB", size, mask, c.TotalTStates, c.ModuleBytes)
		}
		if best != 0 {
			t.Fatal("seeded regression must retain cheaper original", best)
		}
		if r.Evidence.Selected != best || !bytes.Equal(r.Binary, r.Evidence.Candidates[best].Binary) {
			t.Fatal("unmeasured winner")
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, m, w, *raw)
		if err != nil || !jointEqual(replay, r.Evidence.Candidates[best].Run) {
			t.Fatal("retained seeded winner", err)
		}
		if dir := os.Getenv("MINZ_SEEDED_ARTIFACTS"); dir != "" {
			os.MkdirAll(dir, 0755)
			e, _ := json.MarshalIndent(r.Evidence, "", "  ")
			work, _ := json.MarshalIndent(w, "", "  ")
			host, _ := json.MarshalIndent(map[string]any{"results": wants, "snapshots": snaps}, "", "  ")
			for name, b := range map[string][]byte{fmt.Sprintf("size-%v-evidence.json", size): e, "source.nanz": source, "workload.json": work, "profile.json": p, "host-oracle.json": host, fmt.Sprintf("size-%v-selected.bin", size): r.Binary, fmt.Sprintf("size-%v-selected.a80", size): []byte(r.Steps.Assembly)} {
				if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

const seededBoundarySource = `fun seeded_sum(ptr:^u8,n:u8,seed:u16)->u16 {
 var sum:u16=seed
 var i:u8=0
 while i<n {sum=sum+u16(ptr[i])
 i=i+1}
 return sum
}`

func TestSeededPointerWalkAddressDomain(t *testing.T) {
	for _, tuple := range []bool{false, true} {
		source := seededBoundarySource
		if tuple {
			source = strings.Replace(source, "->u16", "->(u16,u8)", 1)
			source = strings.Replace(source, "return sum", "return (sum,i)", 1)
		}
		before := hir.LowerModule(graceMeasureParse(t, source))
		afterHIR := graceMeasureParse(t, source)
		r, err := hir.RunGrace(afterHIR, 0)
		if err != nil || r.ByRule["hir-guarded-pointer-walk"] != 1 {
			t.Fatal("missing seeded transformation", err)
		}
		r, err = hir.RunGrace(afterHIR, 0)
		if err != nil || r.Applied != 0 {
			t.Fatal("fallback transformed twice")
		}
		after := hir.LowerModule(afterHIR)
		for _, heapSize := range []int{16, 65536} {
			memory := graceSumMemory(heapSize)
			for n := int64(0); n < 256; n++ {
				for _, base := range []int64{0, 15, 65535 - n, 65536 - n, 65535} {
					for _, seed := range []int64{0, 1, 65535} {
						var results [2][]mir2.Value
						var errors [2]string
						for j, m := range []*mir2.Module{before, after} {
							vm := mir2.NewVM(m)
							vm.EnsureHeap(int64(heapSize))
							vm.WriteHeapSlice(0, memory)
							got, err := vm.Call("seeded_sum", []mir2.Value{{I: base}, {I: n}, {I: seed}})
							results[j] = got
							errors[j] = graceSumError(err)
							if !bytes.Equal(vm.ReadHeap(0, heapSize), memory) {
								t.Fatal("input memory mutated")
							}
						}
						if errors[0] != errors[1] || !reflect.DeepEqual(results[0], results[1]) {
							t.Fatalf("heap%d ptr%d n%d seed%d before%v/%s after%v/%s", heapSize, base, n, seed, results[0], errors[0], results[1], errors[1])
						}
						sum := uint16(seed)
						bad := int64(-1)
						for i := int64(0); i < n; i++ {
							addr := int64(uint16(base + i))
							if addr >= int64(heapSize) {
								bad = addr
								break
							}
							sum += uint16(memory[addr])
						}
						if bad >= 0 {
							want := fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", bad, heapSize)
							if errors[1] != want {
								t.Fatal("independent trap oracle", errors[1], want)
							}
						} else {
							want := []mir2.Value{{I: int64(sum)}}
							if tuple {
								want = append(want, mir2.Value{I: n})
							}
							if errors[1] != "" || !reflect.DeepEqual(results[1], want) {
								t.Fatal("independent modulo arithmetic", results[1], want, errors[1])
							}
						}
					}
				}
			}
		}
	}
}

func TestSeededPointerWalkZ80Boundary(t *testing.T) {
	for _, n := range []int64{0, 1, 5, 255} {
		for _, base := range []int64{65535 - n, 65536 - n, 65535} {
			for _, seed := range []int64{0, 65535} {
				memory := map[uint16]byte{}
				sum := uint16(seed)
				for i := int64(0); i < n; i++ {
					addr := uint16(base + i)
					v := byte(i*37 + 11)
					memory[addr] = v
					sum += uint16(v)
				}
				for _, enabled := range []bool{false, true} {
					t.Logf("boundary ptr%d n%d seed%d enabled%v", base, n, seed, enabled)
					measureGraceZ80HIR(t, graceMeasureParse(t, seededBoundarySource), "seeded_sum", Options{AssertMode: "none", UseHIRGrace: enabled}, []int64{base, n, seed}, memory, int64(sum))
				}
			}
		}
	}
}

func TestSeededPointerWalkLegality(t *testing.T) {
	for name, source := range map[string]string{
		"seed_call":        strings.Replace(seededBoundarySource, "sum:u16=seed", "sum:u16=get_seed()", 1) + "\nfun get_seed()->u16 {return 7}",
		"seed_global":      "global initial:u16=7\n" + strings.Replace(seededBoundarySource, "sum:u16=seed", "sum:u16=initial", 1),
		"seed_expr":        strings.Replace(seededBoundarySource, "sum:u16=seed", "sum:u16=seed+1", 1),
		"index_nonzero":    strings.Replace(seededBoundarySource, "i:u8=0", "i:u8=1", 1),
		"stride":           strings.Replace(seededBoundarySource, "ptr[i]", "ptr[i*2]", 1),
		"mutated_seed":     strings.Replace(seededBoundarySource, "sum=sum+u16(ptr[i])", "seed=seed+1\nsum=sum+u16(ptr[i])", 1),
		"mutated_bound":    strings.Replace(seededBoundarySource, "sum=sum+u16(ptr[i])", "n=n-1\nsum=sum+u16(ptr[i])", 1),
		"global_collision": "global seed:u16=7\n" + seededBoundarySource,
	} {
		t.Run(name, func(t *testing.T) {
			m := graceMeasureParse(t, source)
			if len(hir.AnalyzeFacts(m).PointerWalks) != 0 {
				t.Fatal("unsupported form recognized")
			}
			r, err := hir.RunGrace(m, 0)
			if err != nil || r.ByRule["hir-guarded-pointer-walk"] != 0 {
				t.Fatal("unsafe rewrite", err)
			}
		})
	}
	for _, seed := range []string{"0", "1", "65535"} {
		m := graceMeasureParse(t, strings.Replace(seededBoundarySource, "sum:u16=seed", "sum:u16="+seed, 1))
		if len(hir.AnalyzeFacts(m).PointerWalks) != 1 {
			t.Fatal("literal seed refused", seed)
		}
	}
}
