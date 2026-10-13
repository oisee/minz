package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"strconv"
	"testing"
)

func TestZMachineDecimalNative(t *testing.T) {
	command := exec.Command("python3", "-c", "from native_decimal import emit; print(emit())")
	command.Dir = "../../../scripts/zmachine"
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	// Hook only the Z3 stream-routing entry. Production emitter is tested unchanged.
	src := `global digits:[u8;8]
 global used:u8
 fun emit_zscii(@z80_hl value:u16) {
 asm z80 (in value) (clob A,F,DE,HL) {
 LD A,(used)
 LD E,A
 LD D,0
 LD A,L
 LD HL,digits
 ADD HL,DE
 LD (HL),A
 LD A,E
 INC A
 LD (used),A
 }
 }
 fun main() {print_num(0)}
 ` + string(output)
	h, err := nanz.Parse(src, "number.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x9000+uint16(i), b, false)
	}
	digits := uint16(a.Symbols["digits"])
	used := uint16(a.Symbols["used"])
	m.Memory.SetPagingForce(0x10)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	for value := 0; value < 65536; value++ {
		m.Memory.Write(used, 0, false)
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		m.CPU.SetPC(uint16(a.Symbols["print_num"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetHL(uint16(value))
		for i := 0; i < 10000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		count := int(m.Memory.Read(used))
		var got []byte
		for i := 0; i < count && i < 8; i++ {
			got = append(got, m.Memory.Read(digits+uint16(i)))
		}
		want := strconv.Itoa(int(int16(value)))
		if string(got) != want || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatalf("value%d got%q want%q PC/SP%04x/%04x", value, got, want, m.CPU.PC(), m.CPU.SP())
		}
	}
	t.Log("all 65536 signed print_num values match independent decimal oracle")
}
