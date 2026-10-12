package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hardwareTextureSetup = `setup "coords_u" "coords_v" "along_u" "along_v" "between_v" "between_u" "next_row_u" "next_row_v";`

func hardwareTextureSource(name, body string) string {
	return `import hw.texture
 global coords_u:u16=0
 global coords_v:u16=0
 global along_u:u16=0
 global along_v:u16=0
 global between_v:u16=0
 global between_u:u16=0
 global next_row_u:u16=0
 global next_row_v:u16=0
 @texture_walk("` + name + `") { ` + strings.ReplaceAll(body, ";", "\n") + ` }
 fun main()->void { ` + name + `() }
 `
}
func hardwareTextureExport(t *testing.T, name string, data []byte) {
	t.Helper()
	if dir := os.Getenv("MINZ_HW_TEXTURE_EXPORT"); dir != "" {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// Direct affine arithmetic is independent of the emitted ASM; OpAsm is not a VM oracle.
// Every run changes both coordinate words and the mutable texture.
func TestHardwareTextureWalkNative(t *testing.T) {
	type layout struct {
		name                   string
		base, page, cols, rows int
	}
	layouts := []layout{{"spectrum", 0x5800, 0xc000, 32, 24}, {"relocated", 0x6809, 0xd300, 32, 3}, {"tiny", 0x6a11, 0xc100, 4, 2}, {"singleton", 0x6b13, 0xc200, 1, 1}, {"last-page", 0x6c15, 0xff00, 8, 2}}
	for _, l := range layouts {
		for _, unroll := range []int{1, 2, 4, 8, 16, 32} {
			if l.cols%unroll != 0 {
				continue
			}
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(fmt.Sprintf("%s/unroll-%d/%s", l.name, unroll, mode), func(t *testing.T) {
					body := fmt.Sprintf(`destination "zx_halfcells" %d %d %d; texture "page16x16" %d; %s unroll %d;`, l.base, l.cols, l.rows, l.page, hardwareTextureSetup, unroll)
					source := hardwareTextureSource("mapped_texture", body)
					h, err := nanz.ParseWithOpts(source, "hardware_texture.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
					if err != nil {
						t.Fatal(err)
					}
					s, err := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if err != nil {
						t.Fatal(err)
					}
					asm := "ORG 0x8000\n" + s.Assembly
					a, err := z80asm.NewAssembler().AssembleString(asm)
					if err != nil {
						t.Fatal(err)
					}
					if len(a.Errors) > 0 {
						t.Fatal(a.Errors)
					}
					prefix := fmt.Sprintf("%s/unroll-%d/%s/", l.name, unroll, mode)
					hardwareTextureExport(t, prefix+"source.nanz", []byte(source))
					hardwareTextureExport(t, prefix+"program.a80", []byte(asm))
					hardwareTextureExport(t, prefix+"program.bin", a.Binary)
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					for addr := range mem {
						mem[addr] = byte((addr * 71) ^ (addr >> 5) ^ 0xb9)
					}
					z.LoadMemory(0x8000, a.Binary)
					type observation struct {
						Seed int `json:"seed"`
						T    int `json:"T_states"`
					}
					var observations []observation
					for seed := 0; seed < 8; seed++ {
						texture := make([]byte, 256)
						for i := range texture {
							texture[i] = byte((i*3 + (i>>3)*5 + seed*7) & 7)
						}
						if seed == 7 {
							for i := range texture {
								texture[i] = 5
							}
						}
						copy(mem[l.page:l.page+256], texture)
						u0, v0 := int(uint16(0xff71+seed*809)), int(uint16(0x00bd-seed*1631))
						du, dv := []int{320, -192, 65535, 1, -257, 0, 511, -1}[seed], []int{0, 263, -1, 65535, 257, -512, 1, 0}[seed]
						hu, hv := []int{0, 131, -129, 32767, -32768, 256, -256, 1}[seed], []int{160, -96, 129, -32768, 32767, -1, 1, 0}[seed]
						ru, rv := []int{0, 262, -257, 65535, 1, -511, 800, -800}[seed], []int{320, -192, 511, -1, 65535, 0, -257, 257}[seed]
						values := map[string]int{"coords_u": u0, "coords_v": v0, "along_u": du, "along_v": dv, "between_u": hu, "between_v": hv, "next_row_u": ru, "next_row_v": rv}
						for name, v := range values {
							addr, ok := a.Symbols[name]
							if !ok {
								t.Fatal("missing setup symbol", name)
							}
							mem[addr] = byte(v)
							mem[addr+1] = byte(v >> 8)
						}
						for j := 0; j < l.cols*l.rows; j++ {
							mem[l.base+j] = 0xa5
						}
						mem[0xeff0] = 0
						mem[0xeff1] = 0x70
						stackFloor := 0xefe8
						// Size may replace four RLCA instructions with a leaf helper.
						// The helper has no PUSH or nested CALL, so its return adds two bytes.
						if strings.Contains(asm, "CALL __rotate_4") {
							helper, ok := a.Symbols["__rotate_4"]
							if !ok {
								t.Fatal("missing rotate helper")
							}
							if !bytes.Equal(mem[helper:helper+5], []byte{0x07, 0x07, 0x07, 0x07, 0xc9}) {
								t.Fatal("rotate helper is not the audited four-RLCA/RET leaf")
							}
							stackFloor -= 2
						}
						before := append([]byte(nil), mem...)
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["mapped_texture"]), SP: 0xeff0})
						start := z.GetCycles()
						returned := false
						minimumSP := z.GetSP()
						for steps := 0; steps < 1000000; steps++ {
							if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
								returned = true
								break
							}
							z.Step()
							if sp := z.GetSP(); sp < minimumSP {
								minimumSP = sp
							}
						}
						if !returned {
							t.Fatal("kernel did not return with exact PC/SP", z.GetRegisters())
						}
						if int(minimumSP) != stackFloor {
							t.Fatalf("minimum SP %04x, expected audited floor %04x", minimumSP, stackFloor)
						}
						sample := func(u, v int) byte { return texture[int((uint16(v)>>8)&15)*16+int((uint16(u)>>8)&15)] }
						want := make([]byte, l.cols*l.rows)
						for y := 0; y < l.rows; y++ {
							for x := 0; x < l.cols; x++ {
								u, v := u0+x*du+y*ru, v0+x*dv+y*rv
								want[y*l.cols+x] = 64 | sample(u, v)<<3 | sample(u+hu, v+hv)
							}
						}
						if !bytes.Equal(mem[l.base:l.base+len(want)], want) {
							for j, w := range want {
								if mem[l.base+j] != w {
									t.Fatalf("seed %d byte %d: got %02x want %02x", seed, j, mem[l.base+j], w)
								}
							}
						}
						// No spill homes or hidden scratch globals: only output and the explicit kernel/leaf-helper stack may change.
						for addr, b := range before {
							if mem[addr] != b && !(addr >= l.base && addr < l.base+len(want)) && !(addr >= stackFloor && addr < 0xeff0) {
								t.Fatalf("seed %d unexpected RAM write %04x", seed, addr)
							}
						}
						observations = append(observations, observation{seed, z.GetCycles() - start})
					}
					data, err := json.MarshalIndent(map[string]any{"PASS": true, "layout": l.name, "columns": l.cols, "rows": l.rows, "destination": l.base, "texture_page": l.page, "unroll": unroll, "mode": mode, "bytes": len(a.Binary), "observations": observations, "scope": "independent wrapped affine pixel oracle; runtime setup and texture mutations; exact PC/SP; full RAM unchanged outside destination and bounded kernel/rotate-helper stack; ASM path has no VM oracle"}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					hardwareTextureExport(t, prefix+"result.json", append(data, '\n'))
					t.Logf("%d bytes, seed0 %d T-states, eight independent runtime setups/textures", len(a.Binary), observations[0].T)
				})
			}
		}
	}
}
func TestHardwareTextureWalkRejectsInvalidDescriptions(t *testing.T) {
	destination := `destination "zx_halfcells" 22528 32 24;`
	texture := `texture "page16x16" 49152;`
	valid := destination + texture + hardwareTextureSetup + `unroll 4;`
	cases := map[string]string{
		"missing-destination": strings.Replace(valid, destination, "", 1), "missing-texture": strings.Replace(valid, texture, "", 1), "missing-setup": strings.Replace(valid, hardwareTextureSetup, "", 1),
		"duplicate-destination": valid + destination, "duplicate-texture": valid + texture, "duplicate-setup": valid + hardwareTextureSetup, "duplicate-unroll": valid + `unroll 4;`,
		"unknown-field": valid + `via "magic";`, "destination-layout": strings.Replace(valid, `"zx_halfcells"`, `"bitmap"`, 1), "texture-layout": strings.Replace(valid, `"page16x16"`, `"page32x32"`, 1),
		"destination-arity": strings.Replace(valid, "22528 32 24", "22528 32", 1), "texture-arity": strings.Replace(valid, "49152;", "49152 256;", 1), "setup-arity": strings.Replace(valid, ` "next_row_v"`, "", 1), "unroll-arity": strings.Replace(valid, "unroll 4;", "unroll 4 8;", 1),
		"destination-negative": strings.Replace(valid, "22528", `"-1"`, 1), "destination-overflow": strings.Replace(valid, "22528", "65535", 1), "destination-zero-columns": strings.Replace(valid, "22528 32 24", "22528 0 24", 1), "destination-too-many-columns": strings.Replace(valid, "22528 32 24", "22528 33 24", 1), "destination-zero-rows": strings.Replace(valid, "22528 32 24", "22528 32 0", 1), "destination-too-many-rows": strings.Replace(valid, "22528 32 24", "22528 32 25", 1),
		"texture-negative": strings.Replace(valid, "49152", `"-256"`, 1), "texture-overflow": strings.Replace(valid, "49152", "65536", 1), "texture-unaligned": strings.Replace(valid, "49152", "49153", 1), "texture-destination-overlap": strings.Replace(valid, "49152", "22528", 1),
		"invalid-unroll": strings.Replace(valid, "unroll 4;", "unroll 3;", 1), "zero-unroll": strings.Replace(valid, "unroll 4;", "unroll 0;", 1), "nondividing-unroll": strings.Replace(valid, "22528 32 24", "22528 31 24", 1),
		"empty-symbol": strings.Replace(valid, `"coords_u"`, `""`, 1), "numeric-symbol": strings.Replace(valid, `"coords_u"`, `"0coords"`, 1), "injected-symbol": strings.Replace(valid, `"coords_u"`, `"coords_u) / LD A, 1"`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := nanz.ParseWithOpts(hardwareTextureSource("mapped_texture", body), "hardware_texture_invalid.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err == nil {
				t.Fatal("accepted invalid texture walk", body)
			}
			if !strings.Contains(err.Error(), "texture_walk:") {
				t.Fatalf("expected contract diagnostic, got %v", err)
			}
		})
	}
	for _, name := range []string{"", "7invalid", "render){}", strings.Repeat("x", 128)} {
		t.Run("function-name/"+fmt.Sprint(len(name)), func(t *testing.T) {
			_, err := nanz.ParseWithOpts(hardwareTextureSource(name, valid), "hardware_texture_invalid.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err == nil {
				t.Fatal("accepted invalid generated function name")
			}
			if !strings.Contains(err.Error(), "texture_walk:") {
				t.Fatalf("expected contract diagnostic, got %v", err)
			}
		})
	}
}
