package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Surface addresses come from the declared linear byte layout, not generated
// address helpers. Publication snapshots desired values before later mutation.
func TestHardwareSceneSnapshotNativeVM(t *testing.T)    { hardwareSceneSnapshotCheck(t, false) }
func TestHardwareSceneBakedBoundaryMatrix(t *testing.T) { hardwareSceneSnapshotCheck(t, true) }
func hardwareSceneSnapshotCheck(t *testing.T, baked bool) {
	cases := []struct {
		name, body string
		addresses  []int
	}{
		{"interleaved", "surface 22528 32 24 32; plane 11 2 10 20; plane 23 3 4 4; plane 5 3 4 4;", nil},
		{"last-byte", "surface 65535 1 1 1; plane 0 0 1 1;", []int{65535}},
		{"full-queue", "surface 24576 255 1 255; plane 0 0 255 1;", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addresses := tc.addresses
			if tc.name == "interleaved" {
				for _, r := range [][4]int{{11, 2, 10, 20}, {23, 3, 4, 4}, {5, 3, 4, 4}} {
					for y := 0; y < r[3]; y++ {
						for x := 0; x < r[2]; x++ {
							addresses = append(addresses, 22528+(r[1]+y)*32+r[0]+x)
						}
					}
				}
			}
			if tc.name == "full-queue" {
				for i := 0; i < 255; i++ {
					addresses = append(addresses, 24576+i)
				}
			}
			source := fmt.Sprintf(`import hw.scene
@scene("image") { %s }
fun seed() { var i:u8=0; while i<%d { image_values[i]=i+1; i=i+1 } }
fun mutate() { var i:u8=0; while i<%d { image_values[i]=i xor 0xA5; i=i+1 } }
`, strings.ReplaceAll(tc.body, ";", "\n"), len(addresses), len(addresses))
			for _, mode := range []struct {
				name    string
				opts    Options
				prepare string
			}{
				{"ordinary", Options{AssertMode: "none"}, "loop"},
				{"grace-size", Options{AssertMode: "none", UseGrace: true, OptSize: true}, "loop"},
				{"baked8", Options{AssertMode: "none"}, "baked8"},
				{"baked8-grace-size", Options{AssertMode: "none", UseGrace: true, OptSize: true}, "baked8"},
				{"baked32", Options{AssertMode: "none"}, "baked32"},
				{"baked32-grace-size", Options{AssertMode: "none", UseGrace: true, OptSize: true}, "baked32"},
				{"baked64", Options{AssertMode: "none"}, "baked64"},
				{"baked64-grace-size", Options{AssertMode: "none", UseGrace: true, OptSize: true}, "baked64"},
			} {
				if baked == (mode.prepare == "loop") {
					continue
				}
				t.Run(mode.name, func(t *testing.T) {
					h, e := nanz.ParseWithOpts(strings.ReplaceAll(scenePreparationSource(t, source, mode.prepare), ";", "\n"), "scene.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
					if e != nil {
						t.Fatal(e)
					}
					steps, e := CompileHIRSteps(h, mode.opts)
					if e != nil {
						t.Fatal(e)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
					if e != nil {
						t.Fatal(e)
					}
					if len(a.Errors) > 0 {
						t.Fatal(a.Errors)
					}
					for _, backend := range []string{"VM", "Z80"} {
						t.Run(backend, func(t *testing.T) {
							var call func(string)
							var read func(int) byte
							var global func(string) int
							if backend == "VM" {
								v := mir2.NewVM(steps.MIR2Module)
								v.EnsureHeap(65536)
								call = func(n string) {
									if _, e := v.Call(n, nil); e != nil {
										t.Fatal(e)
									}
								}
								read = func(a int) byte { return v.ReadHeap(int64(a), 1)[0] }
								global = func(n string) int { return int(v.GlobalAddr(n)) }
							} else {
								z := emulator.NewRemogattoZ80()
								if err := z.LoadMemory(0x8000, a.Binary); err != nil {
									t.Fatal(err)
								}
								mem := z.Memory()
								read = func(a int) byte { return mem[a] }
								global = func(n string) int {
									v, ok := a.Symbols[n]
									if !ok {
										t.Fatal("missing", n)
									}
									return v
								}
								call = func(n string) {
									mem[0xeff0], mem[0xeff1] = 0, 0x70
									z.SetRegisters(emulator.Registers{PC: uint16(global(n)), SP: 0xeff0})
									for i := 0; i < 2000000; i++ {
										if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
											return
										}
										z.Step()
									}
									t.Fatal("return PC/SP", n, z.GetRegisters())
								}
							}
							addr := global("image_addresses")
							order := global("image_order")
							seen := map[int]bool{}
							last := -1
							for i, want := range addresses {
								got := int(read(addr+2*i)) | int(read(addr+2*i+1))<<8
								if got != want {
									t.Fatalf("address %d=%04x want %04x", i, got, want)
								}
								index := int(read(order + i))
								if index >= len(addresses) || seen[index] {
									t.Fatal("order permutation", index)
								}
								seen[index] = true
								if addresses[index] <= last {
									t.Fatal("raster order")
								}
								last = addresses[index]
							}
							call("seed")
							call("image_prepare")
							if got := int(read(global("image_dirty_count"))); got != len(addresses) {
								t.Fatalf("queue %d want %d", got, len(addresses))
							}
							for _, address := range addresses {
								if read(address) != 0 {
									t.Fatal("prepare wrote surface", address)
								}
							}
							call("mutate")
							call("image_publish")
							for i, address := range addresses {
								if got := read(address); got != byte(i+1) {
									t.Fatalf("snapshot [%04x]=%02x want %02x", address, got, byte(i+1))
								}
								if read(global("image_shown")+i) != byte(i+1) {
									t.Fatal("shown")
								}
							}
							if read(global("image_dirty_count")) != 0 {
								t.Fatal("queue not consumed")
							}
							call("image_prepare")
							call("image_publish")
							call("image_prepare")
							if read(global("image_dirty_count")) != 0 {
								t.Fatal("unchanged image queued")
							}
							for i, address := range addresses {
								if got := read(address); got != byte(i)^0xA5 {
									t.Fatal("second image", address, got)
								}
							}
						})
					}
				})
			}
		})
	}
}

func TestHardwareSceneRejectsInvalidDescriptions(t *testing.T) {
	for name, body := range map[string]string{
		"dimension-overflow": "surface 0 1 65539 65535; plane 0 0 1 1",
		"bad-prepare":        "surface 22528 32 24 32; prepare \"unknown\"; plane 0 0 1 1",
		"group-zero":         "surface 22528 32 24 32; prepare \"baked\" 0; plane 0 0 1 1",
		"group-overflow":     "surface 22528 32 24 32; prepare \"baked\" 65; plane 0 0 1 1",
		"duplicate-prepare":  "surface 22528 32 24 32; prepare \"loop\"; prepare \"loop\"; plane 0 0 1 1",
		"loop-group":         "surface 22528 32 24 32; prepare \"loop\" 8; plane 0 0 1 1",
		"missing":            "plane 0 0 1 1", "unknown": "surface 22528 32 24 32; actor 0 0 1 1",
		"overlap":       "surface 22528 32 24 32; plane 1 1 3 3; plane 2 2 1 1",
		"overflow":      "surface 65535 2 1 2; plane 0 0 1 1",
		"out-of-bounds": "surface 22528 32 24 32; plane 31 23 2 1",
		"capacity":      "surface 24576 256 1 256; plane 0 0 256 1",
		"empty":         "surface 22528 32 24 32; plane 0 0 0 1",
	} {
		t.Run(name, func(t *testing.T) {
			_, e := nanz.ParseWithOpts("import hw.scene\n@scene(\"image\") {\n"+strings.ReplaceAll(body, ";", "\n")+"\n}", "invalid-scene.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if e == nil || !strings.Contains(e.Error(), "scene:") {
				t.Fatalf("missing scene rejection: %v", e)
			}
		})
	}
}
