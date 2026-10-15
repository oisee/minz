package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

type directProfileObserver struct {
	*tetrisSceneObserver
	labels []struct {
		address uint16
		name    string
	}
	costs map[string]int64
}

func (c *directProfileObserver) DoOpcode() {
	pc := c.PC()
	i := sort.Search(len(c.labels), func(i int) bool { return c.labels[i].address > pc }) - 1
	before := c.m.AbsoluteTStates()
	c.tetrisSceneObserver.DoOpcode()
	if i >= 0 && pc >= 0x6000 && pc < 0xf000 {
		c.costs[c.labels[i].name] += c.m.AbsoluteTStates() - before
	}
}

func TestTetrisDirectTemporalMachine(t *testing.T) {
	src, e := os.ReadFile("../../../examples/zx/tetris_playable.nanz")
	if e != nil {
		t.Fatal(e)
	}
	boot, e := os.ReadFile("../../../scripts/zx_tetris_entry.a80")
	if e != nil {
		t.Fatal(e)
	}
	rom, e := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if e != nil {
		t.Fatal(e)
	}
	colors := []byte{0, 5, 6, 3, 4, 2, 0x46, 1}
	geometry := [][][2]int{{{0, 0}, {1, 0}, {2, 0}, {3, 0}}, {{0, 0}, {1, 0}, {0, 1}, {1, 1}}, {{1, 0}, {0, 1}, {1, 1}, {2, 1}}}
	type pose struct {
		kind, x, y, phase, next, hold int
		refresh                       bool
	}
	poses := []pose{{1, 3, 0, 0, 0, 255, true}, {1, 3, 0, 0, 0, 255, false}}
	for phase := 1; phase < 8; phase++ {
		poses = append(poses, pose{1, 3, 0, phase, 0, 255, false})
	}
	poses = append(poses, pose{1, 3, 1, 0, 0, 255, false}, pose{1, 4, 1, 3, 1, 255, false}, pose{2, 4, 1, 3, 2, 1, false}, pose{2, 4, 1, 3, 2, 1, true}, pose{2, 4, 1, 3, 2, 1, true})
	// Partial board updates: actor overlaps the last row of the range,
	// lies above it, or extends below it and must follow its final row.
	poses = append(poses, pose{2, 4, 1, 3, 2, 1, true}, pose{2, 5, 0, 0, 2, 1, true}, pose{1, 3, 0, 7, 2, 1, true})
	ranges := map[int][2]int{14: {2, 3}, 15: {3, 4}, 16: {1, 2}}
	for _, mode := range []string{"early-control", "ordinary", "grace-speed", "grace-size"} {
		t.Run(mode, func(t *testing.T) {
			control := mode == "early-control"
			source := string(src)
			if control {
				source = strings.Replace(source, "if game_over==0 {pixel_commit_actor()}", "if game_over==0 {pixel_wait_field_end() pixel_commit_actor()}", 1)
			}
			h, e := nanz.ParseWithOpts(source, "tetris-direct.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			// Match the playable CLI: contract optimization is enabled by default.
			// Keep the same bounded stack and complete temporal screen/RAM checks.
			steps, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: strings.HasPrefix(mode, "grace"), UseHIRGrace: strings.HasPrefix(mode, "grace"), OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			assembly := strings.Replace(string(boot), "ORG $6000", "ORG $8000", 1) + "\n" + steps.Assembly
			a, e := z80asm.NewAssembler().AssembleString(assembly)
			if e != nil || len(a.Errors) != 0 {
				t.Fatal(e, a.Errors)
			}
			if 0x8000+len(a.Binary) >= 0xfc60 {
				t.Fatalf("code overlaps bounded launcher stack: %d bytes", len(a.Binary))
			}
			symbol := func(name string) uint16 {
				v, ok := a.Symbols[name]
				if !ok {
					t.Fatal("missing symbol", name)
				}
				return uint16(v)
			}
			m, e := spectrum.New48K(rom)
			if e != nil {
				t.Fatal(e)
			}
			for i, b := range a.Binary {
				m.Memory.Write(0x8000+uint16(i), b, false)
			}
			for at := 0xfe00; at <= 0xff00; at++ {
				m.Memory.Write(uint16(at), 0xfd, false)
			}
			irq := symbol("demo_irq")
			m.Memory.Write(0xfdfd, 0xc3, false)
			m.Memory.Write(0xfdfe, byte(irq), false)
			m.Memory.Write(0xfdff, byte(irq>>8), false)
			initial := make([]byte, 6912)
			solid := func(x, y int) {
				for line := 0; line < 8; line++ {
					scan := y*8 + line
					initial[(scan/64)*2048+(scan%8)*256+((scan/8)%8)*32+x] = 255
				}
			}
			for y := 2; y < 22; y++ {
				for _, x := range []int{10, 21} {
					solid(x, y)
					initial[6144+y*32+x] = 7
				}
			}
			for x := 10; x < 22; x++ {
				solid(x, 22)
				initial[6144+22*32+x] = 7
			}
			for y := 3; y < 7; y++ {
				for _, base := range []int{5, 23} {
					for x := 0; x < 4; x++ {
						solid(base+x, y)
					}
				}
			}
			for i, b := range initial {
				m.Memory.Write(0x4000+uint16(i), b, false)
			}
			board := make([]byte, 200)
			c := &tetrisSceneObserver{CPUCore: m.CPU, t: t, m: m, returnSP: 0xfce2, returnPC: 0x5e00, publish: symbol("pixel_publish_direct"), phaseMax: map[string]int64{}, phases: map[uint16]string{symbol("stage_pixel_render"): "stage", symbol("stage_pixel_erase"): "erase-stage", symbol("pixel_prepare_bands"): "bands", symbol("pixel_restore_attributes"): "attributes", symbol("field_board_publish_range_prepared"): "board"}}
			if symbol("pixel_wait_field_end") < 0x8000 {
				t.Fatal("timed wait resides in contended RAM")
			}
			profiler := &directProfileObserver{tetrisSceneObserver: c, costs: map[string]int64{}}
			for _, line := range strings.Split(assembly, "\n") {
				if !strings.HasPrefix(line, "; fun ") {
					continue
				}
				name := strings.Fields(strings.TrimPrefix(line, "; fun "))[0]
				name = strings.Split(name, "(")[0]
				if at, ok := a.Symbols[name]; ok {
					profiler.labels = append(profiler.labels, struct {
						address uint16
						name    string
					}{uint16(at), name})
				}
			}
			sort.Slice(profiler.labels, func(i, j int) bool { return profiler.labels[i].address < profiler.labels[j].address })
			m.CPU = profiler
			previous := initial
			totalFrames, partial := 0, 0
			var maxPublish, maxRender int64
			for sample, p := range poses {
				if sample == 12 {
					for y := 0; y < 20; y++ {
						for x := 0; x < 10; x++ {
							board[y*10+x] = byte((x+y)%7 + 1)
							if y < 5 && x < 8 {
								board[y*10+x] = 0
							}
						}
					}
				}
				if sample == 13 {
					for i := range board {
						board[i] = 0
					}
				}
				if bounds, ok := ranges[sample]; ok {
					for y := bounds[0]; y < bounds[1]; y++ {
						for x := 0; x < 10; x++ {
							board[y*10+x] = 0
							if x < 3 || x > 7 {
								board[y*10+x] = byte(1 + sample%7)
							}
						}
					}
				}
				set := func(name string, value byte) { m.Memory.Write(symbol(name), value, false) }
				for i, b := range board {
					m.Memory.Write(symbol("board")+uint16(i), b, false)
				}
				for name, v := range map[string]int{"cur_type": p.kind, "cur_rot": 0, "cur_x": p.x, "cur_y": p.y, "pixel_phase": p.phase, "next_type": p.next, "hold_type": p.hold} {
					set(name, byte(v))
				}
				if p.refresh {
					set("board_dirty", 1)
					set("board_first", 20)
					set("board_limit", 0)
					if bounds, ok := ranges[sample]; ok {
						set("board_first", byte(bounds[0]))
						set("board_limit", byte(bounds[1]))
					}
				}
				want := append([]byte(nil), initial...)
				for cell, value := range board {
					x, y := 11+cell%10, 2+cell/10
					want[6144+y*32+x] = colors[value]
					if value != 0 {
						for scan := 0; scan < 8; scan++ {
							py := y*8 + scan
							want[(py/64)*2048+(py%8)*256+((py/8)%8)*32+x] = 255
						}
					}
				}
				for _, cell := range geometry[p.kind] {
					cx, cy := 11+p.x+cell[0], 2+p.y+cell[1]
					for scan := 1; scan <= 6; scan++ {
						py := cy*8 + p.phase + scan
						want[(py/64)*2048+(py%8)*256+((py/8)%8)*32+cx] |= 0x7e
					}
					if p.phase < 7 {
						want[6144+cy*32+cx] = colors[p.kind+1]
					}
					if p.phase > 1 {
						want[6144+(cy+1)*32+cx] = colors[p.kind+1]
					}
				}
				for _, preview := range []struct{ kind, base int }{{p.next, 23}, {p.hold, 5}} {
					if preview.kind < 3 {
						for _, cell := range geometry[preview.kind] {
							want[6144+(3+cell[1])*32+preview.base+cell[0]] = colors[preview.kind+1]
						}
					}
				}
				allowed := map[int]bool{}
				for name, width := range map[string]int{"tetris_values": 232, "pixel_geometry_ready": 1, "pixel_geometry_kind": 1, "pixel_geometry_rotation": 1, "pixel_geometry_x": 1, "pixel_geometry_y": 1, "pixel_geometry_last": 1, "pixel_new_x": 4, "pixel_new_y": 4, "pixel_old_x": 4, "pixel_old_y": 4, "board_dirty": 1, "drawn_valid": 1, "drawn_type": 1, "drawn_rot": 1, "drawn_x": 1, "drawn_y": 1, "drawn_ghost_y": 1, "drawn_next": 1, "drawn_hold": 1, "render_changed": 1, "drawn_phase": 1, "pixel_first_row": 1, "pixel_row_limit": 1, "pixel_left": 20, "pixel_right": 20, "field_pixels_count": 1, "direct_refresh": 1, "board_first": 1, "board_limit": 1, "direct_board_first": 1, "direct_board_limit": 1, "direct_actor_early": 1, "direct_preview": 1, "direct_actor_lastrow": 1, "field_pixels_sources": 40, "field_pixels_destinations": 40, "field_pixels_widths": 20} {
					for i := 0; i < width; i++ {
						allowed[int(symbol(name))+i] = true
					}
				}
				for instance := 0; instance < 4; instance++ {
					for row := 0; row < 8; row++ {
						at := int(symbol(fmt.Sprintf("FallingBlock_%d_draw_addresses", instance))) + row*2
						allowed[at], allowed[at+1] = true, true
					}
					allowed[int(symbol(fmt.Sprintf("FallingBlock_%d_ready", instance)))] = true
					allowed[int(symbol(fmt.Sprintf("FallingBlock_%d_erase_ready", instance)))] = true
					for row := 0; row < 8; row++ {
						slot := int(symbol(fmt.Sprintf("FallingBlock_%d_row_%d$dst$imm", instance, row)))
						allowed[slot] = true
						allowed[slot+1] = true
						clearSlot := int(symbol(fmt.Sprintf("FallingBlock_%d_clear_row_%d$dst$imm", instance, row)))
						allowed[clearSlot] = true
						allowed[clearSlot+1] = true
					}
				}
				for _, line := range strings.Split(assembly, "\n") {
					f := strings.Fields(line)
					if len(f) < 3 || !strings.HasPrefix(f[0], "_spill_") {
						continue
					}
					size := 1
					if f[1] == "DW" {
						size = 2
					}
					count := len(strings.Split(strings.Join(f[2:], ""), ","))
					for i := 0; i < size*count; i++ {
						allowed[int(symbol(strings.TrimSuffix(f[0], ":")))+i] = true
					}
				}
				ts := c.Tstates()
				c.Reset()
				c.SetTstates(ts)
				c.SetPC(symbol("render_smooth"))
				c.SetSP(0xfce0)
				c.SetI(0xfe)
				c.SetIM(2)
				c.SetIFF1(true)
				c.SetIFF2(true)
				m.Memory.Write(0xfce0, 0, false)
				m.Memory.Write(0xfce1, 0x5e, false)
				m.Memory.Write(0x5e00, 0x76, false)
				c.done = false
				c.irqActive = false
				c.minSP = 0xfce0
				c.start = m.AbsoluteTStates()
				c.publishStart = 0
				c.phaseLast = map[string]int64{}
				irqBefore := c.irq
				before := make([]byte, 65536)
				for at := range before {
					before[at] = m.Memory.ReadByteInternal(uint16(at))
				}
				legal := map[[32]byte]bool{sha256.Sum256(shadowScreenPixels(previous)): true, sha256.Sum256(shadowScreenPixels(want)): true}
				frames := 0
				for ; frames < 200; frames++ {
					m.RunFrame()
					got := shadowFetchedPixels(m)
					totalFrames++
					if !legal[sha256.Sum256(got)] {
						partial++
						if root := os.Getenv("MINZ_TETRIS_DIRECT_EXPORT"); root != "" {
							path := filepath.Join(root, mode, fmt.Sprintf("partial-%02d-%03d.rgba", sample, frames))
							if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(path, got, 0644); err != nil {
								t.Fatal(err)
							}
						}
					}
					if c.done {
						break
					}
				}
				if !c.done {
					t.Fatal("no balanced renderer return", sample)
				}
				m.RunFrame()
				totalFrames++
				if !bytes.Equal(shadowFetchedPixels(m), shadowScreenPixels(want)) {
					t.Fatalf("sample%d final fetched scene mismatch", sample)
				}
				for at, old := range before {
					expected := old
					if at == 0x5c78 {
						expected = old + byte(c.irq-irqBefore)
					}
					if at >= 0x4000 && at < 0x5b00 {
						expected = want[at-0x4000]
					} else if allowed[at] || (at >= int(c.minSP) && at < 0xfce0) {
						continue
					}
					if got := m.Memory.ReadByteInternal(uint16(at)); got != expected {
						t.Fatalf("sample%d phase%d memory%04x=%02x want%02x", sample, p.phase, at, got, expected)
					}
				}
				// The allowed coordinate metadata still has an independent
				// oracle: derive it from the fixture geometry, not its LUTs.
				for cell, offset := range geometry[p.kind] {
					for row := 0; row < 8; row++ {
						scan := (2+p.y+offset[1])*8 + p.phase + row
						address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + 11 + p.x + offset[0]
						slot := symbol(fmt.Sprintf("FallingBlock_%d_draw_addresses", cell)) + uint16(row*2)
						got := int(m.Memory.ReadByteInternal(slot)) | int(m.Memory.ReadByteInternal(slot+1))<<8
						if got != address {
							t.Fatalf("sample%d cached cell%d row%d=%04x want%04x", sample, cell, row, got, address)
						}
					}
					for name, want := range map[string]int{"pixel_new_x": p.x + offset[0], "pixel_new_y": p.y + offset[1]} {
						if got := m.Memory.ReadByteInternal(symbol(name) + uint16(cell)); got != byte(want) {
							t.Fatalf("sample%d %s[%d]=%d want%d", sample, name, cell, got, want)
						}
					}
				}
				if sample >= 2 {
					old := poses[sample-1]
					for cell, offset := range geometry[old.kind] {
						for name, want := range map[string]int{"pixel_old_x": old.x + offset[0], "pixel_old_y": old.y + offset[1]} {
							if got := m.Memory.ReadByteInternal(symbol(name) + uint16(cell)); got != byte(want) {
								t.Fatalf("sample%d %s[%d]=%d want%d", sample, name, cell, got, want)
							}
						}
					}
				}
				if sample >= 2 {
					first, limit := 20, 0
					{
						first, limit = 20, 0
						for _, pose := range []pose{poses[sample-1], p} {
							for _, offset := range geometry[pose.kind] {
								y := pose.y + offset[1]
								if y < first {
									first = y
								}
								end := y + 1
								if pose.phase > 1 {
									end++
								}
								if end > limit {
									limit = end
								}
							}
						}
					}
					for name, want := range map[string]int{"pixel_first_row": first, "pixel_row_limit": limit} {
						if got := m.Memory.ReadByteInternal(symbol(name)); got != byte(want) {
							t.Fatalf("sample%d %s=%d want%d", sample, name, got, want)
						}
					}
				}
				if sample != 1 {
					expectedPreview := byte(0)
					if sample == 0 || poses[sample-1].next != p.next || poses[sample-1].hold != p.hold {
						expectedPreview = 1
					}
					if got := m.Memory.ReadByteInternal(symbol("direct_preview")); got != expectedPreview {
						t.Fatalf("sample%d preview dirty=%d want%d", sample, got, expectedPreview)
					}
				}
				lastY := p.y
				for _, offset := range geometry[p.kind] {
					if p.y+offset[1] > lastY {
						lastY = p.y + offset[1]
					}
				}
				for name, want := range map[string]int{"pixel_geometry_ready": 1, "pixel_geometry_kind": p.kind, "pixel_geometry_rotation": 0, "pixel_geometry_x": p.x, "pixel_geometry_y": p.y, "pixel_geometry_last": lastY} {
					if got := m.Memory.ReadByteInternal(symbol(name)); got != byte(want) {
						t.Fatalf("sample%d %s=%d want%d", sample, name, got, want)
					}
				}
				expectedState := map[string]int{"board_first": 20, "board_limit": 0}
				if p.refresh {
					first, limit := 0, 20
					if bounds, ok := ranges[sample]; ok {
						first, limit = bounds[0], bounds[1]
					}
					minRow, maxRow := 20, 0
					for _, offset := range geometry[p.kind] {
						y := p.y + offset[1]
						begin := (8*y + p.phase + 1) / 8
						end := (8*y + p.phase + 6) / 8
						if begin < minRow {
							minRow = begin
						}
						if end > maxRow {
							maxRow = end
						}
					}
					early, last := 0, maxRow
					if maxRow < first || minRow >= limit {
						early = 1
					} else if maxRow >= limit {
						last = limit - 1
					}
					expectedState["direct_board_first"] = first
					expectedState["direct_board_limit"] = limit
					expectedState["direct_actor_early"] = early
					expectedState["direct_actor_lastrow"] = last
				}
				for name, w := range expectedState {
					if got := int(m.Memory.ReadByteInternal(symbol(name))); got != w {
						t.Fatalf("sample%d %s=%d want%d", sample, name, got, w)
					}
				}
				if c.minSP < 0xfc60 {
					t.Fatal("stack exceeded reserved 128 bytes", c.minSP)
				}
				if c.publishT > maxPublish {
					maxPublish = c.publishT
				}
				if c.elapsed > maxRender {
					maxRender = c.elapsed
				}
				t.Logf("sample%d phase%d frames%d render+wait%dT publish%dT stages%v", sample, p.phase, frames+1, c.elapsed, c.publishT, c.phaseLast)
				previous = want
			}
			if root := os.Getenv("MINZ_TETRIS_DIRECT_EXPORT"); root != "" {
				dir := filepath.Join(root, mode)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				data, err := json.MarshalIndent(profiler.costs, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "instruction-costs.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if control {
				if partial == 0 {
					t.Fatal("negative control missed mixed frames")
				}
			} else if partial != 0 {
				t.Fatalf("partial fetched frames %d/%d", partial, totalFrames)
			}
			if c.irq == 0 {
				t.Fatal("IRQ/frame budget", c.irq, maxPublish)
			}
			if bytes.Equal(previous, initial) {
				t.Fatal("final actor unexpectedly absent")
			}
			t.Logf("%d scenes, %d fetched frames, %d partial frames; max render+wait%dT publish%dT", len(poses), totalFrames, partial, maxRender, maxPublish)
			if root := os.Getenv("MINZ_TETRIS_DIRECT_EXPORT"); root != "" {
				dir := filepath.Join(root, mode)
				if e = os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				receipt := fmt.Sprintf("scenes=%d\nfetched_frames=%d\npartial_frames=%d\nmax_render_and_wait_T=%d\nmax_publish_T=%d\ncode_origin=0x8000\nshadow_bitmap=none\nbytes=%d\n", len(poses), totalFrames, partial, maxRender, maxPublish, len(a.Binary))
				for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(assembly), "program.bin": a.Binary, "receipt.txt": []byte(receipt), "last-expected.scr": previous, "last-fetched.rgba": shadowFetchedPixels(m)} {
					if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}
