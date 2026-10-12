package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestWeightedV4CheckedPacketCaller(t *testing.T) {
	checksumPath := "../../../examples/nanz/grace_weighted_checksum.nanz"
	source, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatal(err)
	}
	source = append(source, []byte(`
fun byte_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun checked_packet(ptr:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
 var byte_value:u8=byte_sum(ptr,n)
 var weighted:u16=weighted_checksum(ptr,n,a,b,seed)
 return record_checksum(weighted+u16(byte_value))
}
`)...)
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), "checked-packet.nanz") }
	data, err := os.ReadFile("../../../docs/grace-pgo-scalar-artifacts/workload.json")
	if err != nil {
		t.Fatal(err)
	}
	var w hir.ProfileWorkload
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	for i := range w.Calls {
		w.Calls[i].Function = "checked_packet"
	}
	wants := make([]int64, len(w.Calls))
	snapshots := make([]hir.ProfileMemorySnapshot, len(w.Calls))
	calls := w.Globals["checksum_calls"][0]
	for i, c := range w.Calls {
		args := c.Arguments
		sum := uint64(*args[4].Value)
		var byteSum uint8
		for _, b := range w.Resources[args[0].Resource][:int(*args[1].Value)] {
			sum += uint64(*args[2].Value) + uint64(*args[3].Value) + uint64(b)
			byteSum += b
		}
		sum += uint64(byteSum)
		wants[i] = int64(uint16(sum))
		calls++
		snapshots[i] = hir.ProfileMemorySnapshot{Globals: map[string][]byte{"checksum_calls": {calls}, "checksum_last": {byte(sum), byte(sum >> 8)}}, Resources: w.Resources}
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(raw.Profile)
	for i, want := range wants {
		if raw.Results[i][0].I != want || !jointEqual(raw.Snapshots[i], snapshots[i]) {
			t.Fatal("source host oracle", i)
		}
	}
	for _, size := range []bool{false, true} {
		r, err := MeasureWeightedJointSourceSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if e.Schema != "minz.weighted-joint-source-site-variant-sets/v4" || len(e.EligibleSites) != 1 || len(e.EligiblePointerSites) != 2 || len(e.Candidates) != 16 || len(e.EligibleWeightedOverlapSites) != 1 {
			t.Fatal("incomplete joint source search")
		}
		best := 0
		for i, c := range e.Candidates {
			if c.RulePolicy["hir-constant-if"] != (i >= 8) || c.RequestedSitePolicy[e.EligibleSites[0]] != (i&4 != 0) {
				t.Fatal("incorrect mask", i)
			}
			pointers := map[string]bool{}
			for k, id := range e.EligiblePointerSites {
				pointers[id] = i&(1<<(len(e.EligiblePointerSites)-1-k)) != 0
			}
			if err := weightedSourceSitesFullyRealized(m, c.FinalLineage, c.RequestedSitePolicy, pointers, c.FinalWeightedProof); err != nil {
				t.Fatal(err)
			}
			var total int64
			for j, inv := range c.Run.Invocations {
				if inv.Results[0].I != wants[j] || !jointEqual(inv.Snapshot, snapshots[j]) {
					t.Fatal("target host oracle", i, j)
				}
				if inv.Access == nil || !jointEqual(*inv.Access, c.FinalRawRun.Accesses[j]) {
					t.Fatal("exact traffic", i, j)
				}
				// Independent expected declared-byte traffic: two traversals of the
				// input, plus one persistent counter read/write and word write.
				for name, resource := range w.Resources {
					counts := inv.Access.Resources[name]
					if len(counts.Reads) != len(resource) || len(counts.Writes) != len(resource) {
						t.Fatal("access region size", i, j, name)
					}
					for k := range resource {
						reads := uint64(0)
						if name == w.Calls[j].Arguments[0].Resource && k < int(*w.Calls[j].Arguments[1].Value) {
							reads = 2
						}
						if counts.Reads[k] != reads || counts.Writes[k] != 0 {
							t.Fatal("independent input access oracle", i, j, name, k)
						}
					}
				}
				for name, counts := range inv.Access.Globals {
					for k := range counts.Reads {
						reads := uint64(0)
						if name == "checksum_calls" {
							reads = 1
						}
						if counts.Reads[k] != reads || counts.Writes[k] != 1 {
							t.Fatal("independent global access oracle", i, j, name, k)
						}
					}
				}
				total += inv.TStates
			}
			if total != c.TotalTStates || c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) || c.Run.BinarySHA256 != c.BinarySHA256 {
				t.Fatal("cost/binary mismatch")
			}
			better := c.TotalTStates < e.Candidates[best].TotalTStates
			if size {
				better = c.ModuleBytes < e.Candidates[best].ModuleBytes
			}
			if better {
				best = i
			}
			t.Logf("size%v candidate%d %dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
		}
		copyEvidence := *e
		copyEvidence.SHA256 = ""
		encoded, err := json.Marshal(copyEvidence)
		if err != nil || e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("unbound joint policies")
		}
		if e.Selected != best || !bytes.Equal(r.Binary, e.Candidates[best].Binary) {
			t.Fatal("winner not measured minimum")
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, m, w, *raw)
		if err != nil || !jointEqual(replay, e.Candidates[best].Run) {
			t.Fatal("retained winner", err)
		}
		t.Logf("size%v winner%d", size, best)
		if dir := os.Getenv("MINZ_JOINT_SITE_ARTIFACTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			ev, _ := json.MarshalIndent(e, "", "  ")
			work, _ := json.MarshalIndent(w, "", "  ")
			host, _ := json.MarshalIndent(map[string]any{"results": wants, "snapshots": snapshots}, "", "  ")
			for name, b := range map[string][]byte{fmt.Sprintf("size-%v-evidence.json", size): ev, fmt.Sprintf("size-%v-selected.bin", size): r.Binary, fmt.Sprintf("size-%v-selected.a80", size): []byte(r.Steps.Assembly), "source.nanz": source, "workload.json": work, "profile.json": p, "host-oracle.json": host} {
				if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
