package nanz_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Full pipeline/PBQP, independent expected output bytes and live global successor.
func TestLiteralPrintingNative(t *testing.T) {
	cases := []struct {
		name, literal string
		want          []byte
	}{{"s", `"AB"`, []byte("AB")}, {"c", `c"AB"`, []byte("AB")}, {"l", `l"AB"`, []byte("AB")}, {"utf8", `"é"`, []byte{0xc3, 0xa9}}, {"empty_s", `""`, nil}, {"empty_c", `c""`, nil}, {"empty_l", `l""`, nil}, {"nul_s", `"A\0B"`, []byte{65, 0, 66}}, {"nul_c", `c"A\0B"`, []byte{65}}, {"nul_l", `l"A\0B"`, []byte{65, 0, 66}}, {"s255", "\"" + strings.Repeat("q", 255) + "\"", bytes.Repeat([]byte{'q'}, 255)}, {"l256", "l\"" + strings.Repeat("r", 256) + "\"", bytes.Repeat([]byte{'r'}, 256)}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := fmt.Sprintf("global __print_ptr_1:u16=123\nglobal __print_count_2:u16=456\nglobal live:u16=4660\nglobal answer:u16=0\nfun show()->void {\n let x:u16=live\n @print(%s)\n @print(c\"!\")\n answer=x+1\n}\n", c.literal)
			h, e := nanz.Parse(source, "native_strings.nanz")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := pipeline.CompileHIRSteps(h)
			if e != nil {
				t.Fatal(e)
			}
			assembled := "ORG 0x0100\n LD SP,0xF000\n CALL show\n test_done:\n DI\n HALT\n" + steps.Assembly
			a, e := z80asm.NewAssembler().AssembleString(assembled)
			if e != nil {
				t.Fatal(e)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1000000
			if e = z.LoadMemory(0x100, a.Binary); e != nil {
				t.Fatal(e)
			}
			var out []byte
			z.SetIOHandlers(nil, func(port uint16, v byte) {
				if byte(port) != 0x23 {
					t.Errorf("unexpected port%x", port)
				}
				out = append(out, v)
			})
			z.SetPC(0x100)
			runErr := z.Run()
			regs := z.GetRegisters()
			saveStringNative(t, c.name, source, assembled, steps, a, out, append(append([]byte{}, c.want...), byte('!')), regs, z, runErr)
			if runErr != nil {
				t.Fatal(runErr)
			}
			if regs.SP != 0xF000 || int(regs.PC) != a.Symbols["test_done"]+2 {
				t.Fatalf("termination PC/SP %x/%x", regs.PC, regs.SP)
			}
			for n, v := range map[string]uint16{"__print_ptr_1": 123, "__print_count_2": 456} {
				addr := a.Symbols[n]
				got := uint16(z.GetMemory(uint16(addr))) | uint16(z.GetMemory(uint16(addr+1)))<<8
				if got != v {
					t.Fatal("generated name clobbered global", n, got)
				}
			}
			want := append(append([]byte{}, c.want...), byte('!'))
			if !bytes.Equal(out, want) {
				t.Fatalf("output=%v want=%v", out, want)
			}
			addr, ok := a.Symbols["answer"]
			if !ok {
				t.Fatal("answer absent")
			}
			got := uint16(z.GetMemory(uint16(addr))) | uint16(z.GetMemory(uint16(addr+1)))<<8
			if got != 4661 {
				t.Fatalf("live successor %x", got)
			}
		})
	}
}

func TestLiteralStorageNative(t *testing.T) {
	for _, c := range []struct {
		name, lit string
		want      []byte
	}{{"s", `"AB"`, []byte{2, 65, 66}}, {"c", `c"AB"`, []byte{65, 66, 0}}, {"l", `l"AB"`, []byte{2, 0, 65, 66}}} {
		t.Run(c.name, func(t *testing.T) {
			source := fmt.Sprintf("global first:u8=0\nfun examine()->void {\n let p:^u8=%s\n first=p^\n}\n", c.lit)
			h, e := nanz.Parse(source, "storage.nanz")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := pipeline.CompileHIRSteps(h)
			if e != nil {
				t.Fatal(e)
			}
			assembled := "ORG 0x0100\n LD SP,0xF000\n CALL examine\n storage_done:\n DI\n HALT\n" + steps.Assembly
			a, e := z80asm.NewAssembler().AssembleString(assembled)
			if e != nil {
				t.Fatal(e)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			addr, ok := a.Symbols["_mir2_str_0"]
			if !ok {
				t.Fatal("pool symbol absent")
			}
			actual := a.Binary[addr-0x100 : addr-0x100+len(c.want)]
			if !bytes.Equal(actual, c.want) {
				t.Fatal("native pool encoding", actual, c.want)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1000000
			if e = z.LoadMemory(0x100, a.Binary); e != nil {
				t.Fatal(e)
			}
			z.SetPC(0x100)
			runErr := z.Run()
			r := z.GetRegisters()
			saveStringNative(t, "storage-"+c.name, source, assembled, steps, a, nil, c.want, r, z, runErr)
			if runErr != nil {
				t.Fatal(runErr)
			}
			if r.SP != 0xF000 || int(r.PC) != a.Symbols["storage_done"]+2 {
				t.Fatal("termination", r)
			}
			if z.GetMemory(uint16(a.Symbols["first"])) != c.want[0] {
				t.Fatal("actual dereference first byte")
			}
		})
	}
}

func saveStringNative(t *testing.T, name, source, assembled string, steps pipeline.Steps, a *z80asm.Result, output, expected []byte, regs emulator.Registers, z *emulator.RemogattoZ80, runErr error) {
	t.Helper()
	dir := os.Getenv("MINZ_STRING_NATIVE_ARTIFACTS")
	if dir == "" {
		return
	}
	dir = filepath.Join(dir, name)
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	files := map[string][]byte{"source.nanz": []byte(source), "target.a80": []byte(steps.Assembly), "assembled.a80": []byte(assembled), "binary.bin": a.Binary, "output.bin": output, "expected.bin": expected, "final.mir2": []byte(steps.MIR2Opt)}
	state := map[string]uint16{}
	for _, n := range []string{"answer", "first", "__print_ptr_1", "__print_count_2"} {
		if addr, ok := a.Symbols[n]; ok {
			v := uint16(z.GetMemory(uint16(addr)))
			if n != "first" {
				v |= uint16(z.GetMemory(uint16(addr+1))) << 8
			}
			state[n] = v
		}
	}
	errorText := ""
	if runErr != nil {
		errorText = runErr.Error()
	}
	expectedKind := "stdout"
	if strings.HasPrefix(name, "storage-") {
		expectedKind = "pool_bytes"
	}
	for name, v := range map[string]any{"execution.json": map[string]any{"cycles": z.GetCycles(), "error": errorText, "expected_kind": expectedKind, "globals": state}, "registers.json": regs, "allocation.json": steps.Allocation, "symbols.json": a.Symbols} {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		files[name] = b
	}
	for name, b := range files {
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
