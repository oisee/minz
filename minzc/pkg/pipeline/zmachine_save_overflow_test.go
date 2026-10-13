package pipeline

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

// The real native encoder must fail within the 700-byte 128K slot. A
// deterministic incompressible state exercises capacity handling without a
// full-game route. Overflow may invalidate the previous volatile RAM record.
func TestZMachineRAMSaveOverflowNative(t *testing.T) {
	gen := exec.Command("python3", "-c", `
from native_save import emit
from native_save_stream import emit as pack
story=bytearray(1024);story[0]=3;story[14:16]=b'\x01\x00'
print('import zx.crc')
print('global save_check_first:u16\nglobal save_check_second:u16\nglobal save_check_length:u16')
print(emit(story,0x17,15684,700)+pack())
print('''fun native_state_byte(@z80_hl index:u16)->u8 {
 asm z80 (in index) (ret A) (clob A,F) { LD A,H
 XOR L }
}
fun native_original_byte(@z80_hl index:u16)->u8 {
 asm z80 (in index) (ret A) (clob A,F) { XOR A }
}
fun main()->u8 { return native_save() }''')
`)
	gen.Dir = "../../../scripts/zmachine"
	src, err := gen.Output()
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), "overflow.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, a.Errors)
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for page := range m.Memory.RAM {
		for i := range m.Memory.RAM[page] {
			m.Memory.RAM[page][i] = byte(i*29 + page*17)
		}
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x9000+uint16(i), b, false)
	}
	addr := uint16(a.Symbols["native_save_length"])
	m.Memory.Write(addr, 210, false)
	m.Memory.Write(addr+1, 4, false)
	m.Memory.SetPagingForce(0x10)
	m.Memory.Write(0xbf00, 0, false)
	m.Memory.Write(0xbf01, 0x70, false)
	before := m.Memory.RAM
	m.CPU.SetPC(uint16(a.Symbols["native_save"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	for i := 0; i < 1000000; i++ {
		if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
			break
		}
		m.CPU.DoOpcode()
		if m.CPU.SP() < 0xbe00 {
			t.Fatal("encoder stack escape")
		}
	}
	if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.CPU.AF()>>8 != 0 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() {
		t.Fatalf("overflow PC/SP %04x/%04x A%d", m.CPU.PC(), m.CPU.SP(), m.CPU.AF()>>8)
	}
	if m.Memory.Read(addr) != 0 || m.Memory.Read(addr+1) != 0 {
		t.Fatal("overflow published valid RAM record")
	}
	for _, page := range []int{0, 1, 3, 4, 5, 6} {
		if !bytes.Equal(m.Memory.RAM[page][:], before[page][:]) {
			t.Fatalf("overflow changed unrelated bank%d", page)
		}
	}
	if !bytes.Equal(m.Memory.RAM[7][:15684], before[7][:15684]) {
		t.Fatal("overflow wrote before bounded slot")
	}
	if bytes.Equal(m.Memory.RAM[7][15684+32:], before[7][15684+32:]) {
		t.Fatal("overflow did not exercise output writes")
	}
	t.Log("700-byte RAM overflow rejected; game banks, PC/SP and paging preserved; previous volatile slot invalidated")
}
