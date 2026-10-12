package vir

import (
	"encoding/binary"
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"path/filepath"
	"testing"
)

func checkpointFixture(t *testing.T) (string, string) {
	t.Helper()
	h := make([]byte, 16)
	copy(h, "ENRT")
	binary.LittleEndian.PutUint32(h[4:], 1)
	binary.LittleEndian.PutUint32(h[8:], 156506)
	h[12] = 4
	h[13] = 12
	data := append([]byte{}, h...)
	for i := 0; i < 156506; i++ {
		n := expectedENRTVreg(i, 4)
		r := make([]byte, 1+2+n+2+24)
		r[0] = byte(n)
		data = append(data, r...)
	}
	path := filepath.Join(t.TempDir(), "source.enr")
	os.WriteFile(path, data, 0600)
	idx, e := BuildENRTIndex(path)
	if e != nil {
		t.Fatal(e)
	}
	index := path + ".json"
	raw, _ := json.Marshal(idx)
	os.WriteFile(index, raw, 0600)
	return path, index
}
func TestCheckpointCanonicalAndMutationRejection(t *testing.T) {
	path, index := checkpointFixture(t)
	op, e := OpenENRTOperation(path, index)
	if e != nil {
		t.Fatal(e)
	}
	defer op.Close()
	f, a := productionTableFunction(mir2.ClassAcc)
	p, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	tab := &RegAllocTable{}
	tab.Init()
	ev, e := op.Import(p, tab, false)
	if e != nil || ev.Import.Admitted != 1 || ev.SourceSHA256 == "" || ev.BlockSHA256 == "" || ev.BytesRead > 4096*34 {
		t.Fatal(e, ev)
	}
	if e = op.Validate(); e != nil {
		t.Fatal(e)
	}
	// In-place same-size change in the selected checkpoint fails before proposal.
	fdata, _ := os.ReadFile(path)
	original := append([]byte{}, fdata...)
	fdata[20] ^= 1
	os.WriteFile(path, fdata, 0600)
	empty := &RegAllocTable{}
	empty.Init()
	if _, e = op.Import(p, empty, false); e == nil || empty.Size() != 0 {
		t.Fatal("changed block admitted", e)
	}
	if e = op.Validate(); e == nil {
		t.Fatal("in-place mutation passed final SHA")
	}
	os.WriteFile(path, original, 0600)
	// Same-size replacement with an unchanged opened descriptor is caught by final
	// pathname hashing, including changes outside any looked-up checkpoint.
	fdata = append([]byte{}, original...)
	fdata[len(fdata)-1] ^= 1
	replacement := path + ".replacement"
	os.WriteFile(replacement, fdata, 0600)
	os.Rename(replacement, path)
	if e = op.Validate(); e == nil {
		t.Fatal("same-size replacement missed")
	}
	if _, e = OpenENRTOperation(path, index); e == nil {
		t.Fatal("changed source opened")
	}
	os.WriteFile(path, original, 0600)
	raw, _ := os.ReadFile(index)
	var idx ENRTIndex
	json.Unmarshal(raw, &idx)
	cases := map[string]func(*ENRTIndex){
		"offset":  func(i *ENRTIndex) { i.Blocks[1].Offset++ },
		"order":   func(i *ENRTIndex) { i.Blocks[1].First++ },
		"count":   func(i *ENRTIndex) { i.Blocks[0].Count-- },
		"version": func(i *ENRTIndex) { i.Schema = "enrt-checkpoints/v2" },
		"sourcehash": func(i *ENRTIndex) {
			i.SourceSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var c ENRTIndex
			json.Unmarshal(raw, &c)
			mutate(&c)
			b, _ := json.Marshal(c)
			os.WriteFile(index, b, 0600)
			if bad, e := OpenENRTOperation(path, index); e == nil {
				bad.Close()
				t.Fatal("malformed index accepted")
			}
		})
	}
	os.WriteFile(index, raw, 0600)
	// Crafted same-width redirect is explicitly an untrusted index proposal:
	// a forged valid block digest must not become a canonical record claim.
	// Reversing offsets fails monotonic validation before opening.
	var redirect ENRTIndex
	json.Unmarshal(raw, &redirect)
	redirect.Blocks[0].Offset = redirect.Blocks[1].Offset
	rb, _ := json.Marshal(redirect)
	os.WriteFile(index, rb, 0600)
	if bad, e := OpenENRTOperation(path, index); e == nil {
		bad.Close()
		t.Fatal("redirected index accepted")
	}
	os.WriteFile(index, raw, 0600)
	// A dishonest block hash is never trusted as canonical shape provenance.
	idx.Blocks[0].SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	b, _ := json.Marshal(idx)
	os.WriteFile(index, b, 0600)
	bad, e := OpenENRTOperation(path, index)
	if e != nil {
		t.Fatal(e)
	}
	defer bad.Close()
	if _, e = bad.Import(p, empty, false); e == nil {
		t.Fatal("wrong block hash accepted")
	}
	// Trailing bytes and malformed records fail complete offline build.
	os.WriteFile(path, append(original, 7), 0600)
	if _, e = BuildENRTIndex(path); e == nil {
		t.Fatal("trailing source accepted")
	}
	original[16] = 1
	os.WriteFile(path, original, 0600)
	if _, e = BuildENRTIndex(path); e == nil {
		t.Fatal("bad record accepted")
	}
}
