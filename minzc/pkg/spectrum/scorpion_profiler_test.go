package spectrum

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScorpionProfilerExtendedPages(t *testing.T) {
	m, err := NewScorpionMemory256(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfiler()
	m.SetProfiler(p)
	for page := 0; page < 16; page++ {
		m.Memory.SetScorpionPaging(byte(page/8) << 4)
		m.Memory.SetPaging(0x10 | byte(page%8))
		m.Memory.WriteByte(0xc07b, byte(page+1))
		if b := m.Memory.ReadByte(0xc07b); b != byte(page+1) {
			t.Fatalf("page%d: %d", page, b)
		}
		p.OnExecPaged(0xc07b, page)
		if p.PagedWrite[page][123] != 1 || p.PagedRead[page][123] != 1 || p.PagedExec[page][123] != 1 {
			t.Fatalf("page%d heatmaps", page)
		}
	}
	for page := 0; page < 16; page++ {
		if got := m.Memory.ReadRAMDirect(page, 123); got != byte(page+1) {
			t.Fatalf("bank%d corrupted: %d", page, got)
		}
	}
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := p.ExportProfile(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var exported struct {
		Write map[string]map[string]uint32 `json:"paged_write"`
	}
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Write["page_8"]["C07B"] != 1 || exported.Write["page_15"]["C07B"] != 1 {
		t.Fatal("extended heatmaps missing from exported profile")
	}
}
