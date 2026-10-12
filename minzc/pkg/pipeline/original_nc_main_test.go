package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// This first checkpoint exercises the real filesystem and main in raw/final VM.
// Native execution remains a separate requirement before corpus admission.
func TestOriginalNCWholeMainVM(t *testing.T) {
	const packet = "../../../reports/nc-whole-main/frozen/"
	read := func(name string) []byte {
		b, e := os.ReadFile(packet + name)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var freeze struct {
		Sources   map[string]string `json:"sources"`
		DiskSHA   string            `json:"disk_sha256"`
		Keys      []int64           `json:"input_keys"`
		Globals   map[string]int64  `json:"final_globals"`
		FS        map[string]int64  `json:"expected_filesystem"`
		EventsSHA string            `json:"terminal_oracle_sha256"`
	}
	if e := json.Unmarshal(read("freeze.json"), &freeze); e != nil {
		t.Fatal(e)
	}
	for p, want := range freeze.Sources {
		b, e := os.ReadFile("../../../" + p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != want {
			t.Fatal("source freeze", p, e)
		}
	}
	disk := read("disk.img")
	if fmt.Sprintf("%x", sha256.Sum256(disk)) != freeze.DiskSHA {
		t.Fatal("disk freeze")
	}
	b := read("terminal-events.json")
	if fmt.Sprintf("%x", sha256.Sum256(b)) != freeze.EventsSHA {
		t.Fatal("events freeze")
	}
	var expected []commanderEvent
	if e := json.Unmarshal(b, &expected); e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(string(read("nc.nanz")), "nc.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	save := func(name string, value any) {
		if dir := os.Getenv("MINZ_NC_MAIN_EXPORT"); dir != "" {
			p := filepath.Join(dir, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			b, e := json.MarshalIndent(value, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(mode, e)
		}
		for _, engine := range []struct {
			name string
			m    *mir2.Module
		}{{"raw", hir.LowerModule(parse())}, {"final", s.MIR2Module}} {
			t.Run(mode+"/"+engine.name, func(t *testing.T) {
				vm := mir2.NewVM(engine.m)
				var events []commanderEvent
				keys := 0
				var reads []int64
				host := func(name string, a []mir2.Value) ([]mir2.Value, error) {
					args := []any{}
					for _, v := range a {
						args = append(args, float64(v.I))
					}
					switch name {
					case "disk_read":
						if len(a) != 4 || a[0].I != 0 || a[1].I != vm.GlobalAddr("fs__fat12__win") || (a[2].I != 0 && a[2].I != 2) || a[3].I != 1 {
							return nil, fmt.Errorf("disk authority %v", a)
						}
						reads = append(reads, a[2].I)
						vm.WriteHeapSlice(a[1].I, disk[a[2].I*512:(a[2].I+1)*512])
						return []mir2.Value{{I: 0}}, nil
					case "disk_write":
						return nil, fmt.Errorf("unexpected disk write")
					case "tui_puts":
						if len(a) != 1 {
							return nil, fmt.Errorf("puts arity")
						}
						var text strings.Builder
						done := false
						for i := int64(0); i < 256; i++ {
							p := a[0].I + i
							if p < 0 || p >= 65536 {
								return nil, fmt.Errorf("CString bounds")
							}
							b := vm.ReadHeap(p, 1)[0]
							if b == 0 {
								done = true
								break
							}
							text.WriteByte(b)
						}
						if !done {
							return nil, fmt.Errorf("unterminated CString")
						}
						args = []any{text.String()}
					case "tui_read_key":
						if keys >= len(freeze.Keys) {
							return nil, fmt.Errorf("key exhausted")
						}
						events = append(events, commanderEvent{name, args})
						v := freeze.Keys[keys]
						keys++
						return []mir2.Value{{I: v}}, nil
					case "tui_width", "tui_height":
						events = append(events, commanderEvent{name, args})
						v := int64(80)
						if name == "tui_height" {
							v = 10
						}
						return []mir2.Value{{I: v}}, nil
					case "tui_goto", "tui_color", "tui_reset", "tui_clear", "tui_putch":
					default:
						return nil, fmt.Errorf("unknown host %s", name)
					}
					events = append(events, commanderEvent{name, args})
					return nil, nil
				}
				for _, f := range engine.m.Funcs {
					if f.Attrs.IsExtern {
						name := f.Name
						vm.Hosts[name] = func(a []mir2.Value) ([]mir2.Value, error) { return host(name, a) }
					}
				}
				result, e := vm.Call("main", nil)
				save(mode+"/"+engine.name+"-events.json", events)
				save(mode+"/"+engine.name+"-disk-reads.json", reads)
				if e != nil || len(result) != 0 {
					t.Fatal("main", result, e)
				}
				if !reflect.DeepEqual(events, expected) {
					for i := 0; i < len(events) && i < len(expected); i++ {
						if !reflect.DeepEqual(events[i], expected[i]) {
							t.Fatalf("event %d got%v want%v", i, events[i], expected[i])
						}
					}
					t.Fatalf("event count got%d want%d", len(events), len(expected))
				}
				if keys != len(freeze.Keys) || !reflect.DeepEqual(reads, []int64{0, 2}) {
					t.Fatal("input/disk", keys, reads)
				}
				for name, want := range freeze.Globals {
					a := vm.GlobalAddr(name)
					if a < 0 || int64(vm.ReadHeap(a, 1)[0]) != want {
						t.Fatal("global", name, a, want)
					}
				}
				for name, want := range freeze.FS {
					a := vm.GlobalAddr("fs__fat12__" + name)
					f := engine.m
					var width int
					for _, g := range f.Globals {
						if g.Name == "fs__fat12__"+name {
							width = mir2.ByteWidth(g.Ty)
						}
					}
					var got int64
					for i, b := range vm.ReadHeap(a, width) {
						got |= int64(b) << uint(8*i)
					}
					if a < 0 || got != want {
						t.Fatal("filesystem", name, got, want)
					}
				}
				if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("fs__fat12__win"), 512), disk[1024:1536]) {
					t.Fatal("sector window")
				}
				t.Logf("whole main events%d, disk reads%v, independent filesystem/selection state PASS", len(events), reads)
			})
		}
	}
}
