package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestCPMTetrisInvalidBoardStoreBeforeWrite(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/tetris_cpm.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, guarded := range []bool{false, true} {
		h, e := nanz.ParseWithOpts(string(src), "tetris_cpm.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		opt := Options{AssertMode: "none", ContractOpt: true}
		if guarded {
			opt.Runtime = mir2.Z80RuntimeCPM
			opt.RuntimeStackBytes = 1024
		}
		s, e := CompileHIRSteps(h, opt)
		if e != nil {
			t.Fatal(e)
		}
		asm := s.Assembly
		// Only the test entry is changed: the unchanged compiled original board_set
		// runs x=y=255. The public application main remains covered separately.
		wrapper := "\n__test_bad_board:\n LD C,255\n LD D,255\n LD A,42\n CALL board_set\n RET\n"
		if guarded {
			asm = bytesToReplace(asm, "    CALL main", "    CALL __test_bad_board")
			asm += wrapper
		} else {
			asm = "ORG 0x100\nCALL __test_bad_board\nRET\n" + asm + wrapper
		}
		a, e := z80asm.NewAssembler().AssembleString(asm)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
		copy(mem[0x100:], a.Binary)
		copy(mem[0x7000:], []byte{0xcd, 0, 1, 0})
		before := append([]byte(nil), mem...)
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
		minSP := uint16(0xf000)
		if guarded {
			minSP = uint16(a.Symbols["__cpm_stack_top"])
		}
		done := false
		for n := 0; n < 100000; n++ {
			if z.GetPC() == 0x7003 {
				done = true
				break
			}
			sp := z.GetSP()
			if (!guarded || int(sp) <= a.Symbols["__cpm_stack_top"]) && sp < minSP {
				minSP = sp
			}
			z.Step()
		}
		if !done || z.GetSP() != 0xf000 {
			t.Fatal("OS return", guarded, z.GetRegisters())
		}
		target := a.Symbols["board"] + 2805
		want := append([]byte(nil), before...)
		label := "ordinary-out-of-object"
		if guarded {
			label = "guarded-before-write"
			if mem[a.Symbols["__cpm_status"]] != 2 {
				t.Fatal("status2")
			}
			want[a.Symbols["__cpm_status"]] = 2
			copy(want[a.Symbols["__cpm_saved_sp"]:a.Symbols["__cpm_saved_sp"]+2], mem[a.Symbols["__cpm_saved_sp"]:a.Symbols["__cpm_saved_sp"]+2])
			copy(want[minSP:a.Symbols["__cpm_stack_top"]], mem[minSP:a.Symbols["__cpm_stack_top"]])
			copy(want[0xeffe:0xf000], mem[0xeffe:0xf000])
			if mem[target] != before[target] {
				t.Fatal("invalid write occurred")
			}
		} else {
			if mem[target] != 42 {
				t.Fatal("original out-of-object counterexample", target, mem[target])
			}
			want[target] = 42
			copy(want[minSP:0xf000], mem[minSP:0xf000])
		}
		for _, l := range a.Listing {
			if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
				copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
			}
		}
		if !bytes.Equal(want, mem) {
			for p := range mem {
				if want[p] != mem[p] {
					t.Fatalf("%s RAM%04x got%x want%x", label, p, mem[p], want[p])
				}
			}
		}
		if root := os.Getenv("MINZ_CPM_TETRIS_ARTIFACTS"); root != "" {
			dir := filepath.Join(root, label)
			os.MkdirAll(dir, 0755)
			os.WriteFile(filepath.Join(dir, "program.a80"), []byte(asm), 0644)
			os.WriteFile(filepath.Join(dir, "program.bin"), a.Binary, 0644)
		}
		t.Log(label, "actual original board_set(255,255,42), fullRAM/SP PASS")
	}
}
func bytesToReplace(s, old, new string) string {
	return string(bytes.Replace([]byte(s), []byte(old), []byte(new), 1))
}
