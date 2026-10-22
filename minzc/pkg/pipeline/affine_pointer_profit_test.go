package pipeline

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// The ceilings are emulator measurements of the original indexed lowering
// (origin/main 1815f5a35, n=100), using the critic's deterministic memory pattern.
// Keep these independently recorded costs: comparing only output values misses
// register-pair spills introduced by otherwise legal strength reduction.
func TestAffinePointerWalkProfitabilityNative(t *testing.T) {
	cdecl := "unsigned char big[300]; unsigned short wv[300]; unsigned short n; unsigned short res; unsigned short sum;\n"
	ndecl := "global big:[u8;300]\nglobal wv:[u16;300]\nglobal n:u16\nglobal res:u16\nglobal sum:u16\n"
	for _, tc := range []struct {
		name, src              string
		nanz                   bool
		ceiling                int
		sizeCeiling, sizeBytes int
		walking                bool
	}{
		{"n_u8_fwd", ndecl + "fun execute() {\n var i:u8 = 0\n while i < u8(n) { big[i+1] = big[i] + 1\n wv[i] = wv[i+1] + u16(i)\n i = i + 1 }\n res = u16(i)\n}", true, 48696, 48696, 1003, false},
		{"c_iv_body", cdecl + "void execute(void){ unsigned short i; if(n>290) return; for(i=0;i<n;i++){ wv[i] = i*5; sum += big[i+2] + i; } res = i; }", false, 57493, 57493, 1022, false},
		{"c_pressure", cdecl + "void execute(void){ short i; if(n>280) return; for(i=0;i<(short)n;i++){ wv[i]=wv[i+1]+wv[i+2]+big[i]+big[i+3]; big[i+1]^=(unsigned char)wv[i+4]; } }", false, 212390, 212390, 1246, false},
		{"c_u8_bound255", cdecl + "void execute(void){ unsigned char i; for(i=0;i<(unsigned char)n;i++) big[i+40] = i; res=i; }", false, 14896, 14896, 945, true},
	} {
		for _, size := range []bool{false, true} {
			mode := "default"
			if size {
				mode = "size"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				var h *hir.Module
				var err error
				if tc.nanz {
					h, err = nanz.Parse(tc.src, "p.nanz")
				} else {
					h, err = c89.CompileWithOpts(tc.src, "p.c", c89.CompileOpts{NoLibc: true})
				}
				if err != nil {
					t.Fatal(err)
				}
				raw := hir.LowerModule(h)
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: size})
				if err != nil {
					t.Fatal(err)
				}
				bumps := 0
				for _, f := range s.MIR2Module.Funcs {
					for _, b := range f.Blocks {
						for _, x := range b.Insts {
							if x.Op == mir2.OpPtrBump || x.Op == mir2.OpSub && x.Ty == mir2.TyPtr {
								bumps++
							}
						}
					}
				}
				if !tc.walking && bumps != 0 || tc.walking && !size && bumps == 0 {
					t.Errorf("unexpected pointer recurrence shape: %s", s.MIR2Opt)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil || len(a.Errors) > 0 {
					t.Fatal(err, a.Errors)
				}
				if size && len(a.Binary) > tc.sizeBytes {
					t.Errorf("size grew: %d > base %d bytes", len(a.Binary), tc.sizeBytes)
				}
				sym := func(name string) int {
					for _, key := range []string{name, "v_" + name, "_" + name} {
						if v, ok := a.Symbols[key]; ok {
							return v
						}
					}
					t.Fatalf("missing symbol %s", name)
					return 0
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				type read struct {
					name string
					size int
				}
				reads := []read{{"big", 300}, {"wv", 600}, {"res", 2}, {"sum", 2}}
				vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
				for k, r := range reads {
					pat := make([]byte, r.size)
					for i := range pat {
						pat[i] = byte(i*37 + 11 + k)
					}
					copy(mem[sym(r.name):], pat)
					for _, vm := range vms {
						vm.WriteHeapSlice(vm.GlobalAddr(r.name), pat)
					}
				}
				binary.LittleEndian.PutUint16(mem[sym("n"):], 100)
				for _, vm := range vms {
					vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{100, 0})
					if _, err := vm.Call("execute", nil); err != nil {
						t.Fatal(err)
					}
				}
				mem[0xeffe], mem[0xefff] = 0, 0xf1
				z.SetRegisters(emulator.Registers{PC: uint16(sym("execute")), SP: 0xeffe})
				cycles := 0
				for steps := 0; steps < 3000000 && z.GetPC() != 0xf100; steps++ {
					cycles += z.Step()
				}
				if z.GetPC() != 0xf100 || z.GetRegisters().SP != 0xf000 {
					t.Fatal("native call did not return")
				}
				ceiling := tc.ceiling
				if size {
					ceiling = tc.sizeCeiling
				}
				if cycles > ceiling {
					t.Errorf("T(new)=%d > T(base lowering)=%d", cycles, ceiling)
				}
				for _, r := range reads {
					want := vms[0].ReadHeap(vms[0].GlobalAddr(r.name), r.size)
					if !bytes.Equal(want, vms[1].ReadHeap(vms[1].GlobalAddr(r.name), r.size)) || !bytes.Equal(want, mem[sym(r.name):sym(r.name)+r.size]) {
						t.Errorf("VM/native memory mismatch: %s", r.name)
					}
				}
				t.Logf("T(new)=%d <= T(base)=%d", cycles, ceiling)
			})
		}
	}
}
