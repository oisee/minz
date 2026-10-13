package pipeline

import (
	"bytes"
	"encoding/hex"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"testing"
)

func TestZMachineStateStreamNative(t *testing.T) {
	// Source is emitted by the production generator; expected bytes are formed
	// independently from raw physical globals and the original compact layout.
	cmd := exec.Command("python3", "-c", "from native_state_stream import emit, emit_packer; print(emit()+emit_packer())")
	cmd.Dir = "../../../scripts/zmachine"
	generated, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	src := `global pc:u32
 global frame:u8
 global v_sp:u16
 global random_state:u16
 global screen_enabled:u8
 global stream3_depth:u8
 global stream3_table_lo:[u8;16]
 global stream3_table_hi:[u8;16]
 global local_words:[u16;512]
 global ret_pc_lo:[u8;32]
 global ret_pc_hi:[u8;32]
 global ret_pc_bank:[u8;32]
 global ret_store:[u8;32]
 global ret_sp_lo:[u8;32]
 global ret_sp_hi:[u8;32]
 global stack_words:[u16;256]
 fun main() -> u8 { return native_state_byte(0) }
 ` + string(generated)
	h, err := nanz.Parse(src, "state.nanz")
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
	for p := 0; p < 8; p++ {
		for i := range m.Memory.RAM[p] {
			m.Memory.RAM[p][i] = byte(i*37 + p*17)
		}
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x9000+uint16(i), b, false)
	}
	// Fill every saved global with a distinct deterministic byte sequence.
	names := []string{"pc", "frame", "v_sp", "random_state", "screen_enabled", "stream3_depth", "stream3_table_lo", "stream3_table_hi", "local_words", "ret_pc_lo", "ret_pc_hi", "ret_pc_bank", "ret_store", "ret_sp_lo", "ret_sp_hi", "stack_words"}
	sizes := []int{3, 1, 2, 2, 1, 1, 16, 16, 1024, 32, 32, 32, 32, 32, 32, 512}
	expected := []byte{'M', 'Z', '3', 'S'}
	for _, r := range [][2]int{{2, 2}, {18, 6}, {28, 2}, {14, 2}} {
		expected = append(expected, m.Memory.RAM[0][r[0]:r[0]+r[1]]...)
	}
	for j, name := range names {
		address, ok := a.Symbols[name]
		if !ok {
			t.Fatal("missing global", name)
		}
		for i := 0; i < sizes[j]; i++ {
			value := byte(i*19 + j*43)
			m.Memory.Write(uint16(address+i), value, false)
			expected = append(expected, value)
		}
	}
	if len(expected) != 1786 {
		t.Fatal(len(expected))
	}
	m.Memory.Write(0xbf00, 0, false)
	m.Memory.Write(0xbf01, 0x70, false)
	before := m.Memory.RAM
	got := make([]byte, len(expected))
	for index := 0; index <= len(expected)+1; index++ {
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetPC(uint16(a.Symbols["native_state_byte"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetHL(uint16(index))
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 1000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatalf("index%d PC/SP%04x/%04x", index, m.CPU.PC(), m.CPU.SP())
		}
		value := byte(m.CPU.AF() >> 8)
		if index < len(expected) {
			got[index] = value
		} else if value != 0 {
			t.Fatal("out of range", index, value)
		}
	}
	if !bytes.Equal(got, expected) {
		for i := range got {
			if got[i] != expected[i] {
				t.Fatalf("snapshot byte%d got%x want%x", i, got[i], expected[i])
			}
		}
	}
	for p := 0; p < 8; p++ {
		for i, b := range before[p] {
			if p == 2 && i >= 0x3ef0 && i < 0x3f00 {
				continue
			}
			if m.Memory.RAM[p][i] != b {
				t.Fatalf("state reader wrote bank%d offset%x", p, i)
			}
		}
	}

	// Compress the complete stream directly, with bank0 mapped only during reads.
	reference := exec.Command("python3", "-c", "import sys; from restart_rle import pack; print(pack(bytes.fromhex(sys.argv[1])).hex())", hex.EncodeToString(expected))
	reference.Dir = "../../../scripts/zmachine"
	encodedHex, err := reference.Output()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := hex.DecodeString(string(bytes.TrimSpace(encodedHex)))
	if err != nil {
		t.Fatal(err)
	}
	for _, capacity := range []int{len(encoded) - 1, len(encoded), len(encoded) + 1} {
		m.Memory.RAM = before
		offset := 16384 - capacity
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetPC(uint16(a.Symbols["native_state_pack"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetAF(0x1700)
		m.CPU.SetHL(0)
		m.CPU.SetDE(1786)
		m.CPU.SetIX(uint16(offset))
		m.CPU.SetIY(uint16(capacity))
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 2000000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		want := uint16(len(encoded))
		if capacity < len(encoded) {
			want = 65535
		}
		if m.CPU.HL() != want || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatalf("state pack cap%d got%d want%d PC/SP%04x/%04x", capacity, m.CPU.HL(), want, m.CPU.PC(), m.CPU.SP())
		}
		if want != 65535 && !bytes.Equal(m.Memory.RAM[7][offset:offset+len(encoded)], encoded) {
			t.Fatal("state packed bytes mismatch")
		}
		scratch := map[uint16]bool{}
		for name, size := range map[string]int{"native_state_pack_begin": 2, "native_state_pack_literals": 1, "native_state_pack_bank": 1} {
			for i := 0; i < size; i++ {
				scratch[uint16(a.Symbols[name]+i)] = true
			}
		}
		for p := 0; p < 8; p++ {
			for i, value := range before[p] {
				if p == 7 && i >= offset && i < offset+capacity {
					continue
				}
				if p == 2 && (i >= 0x3ec0 && i < 0x3f00 || scratch[uint16(0x8000+i)]) {
					continue
				}
				if m.Memory.RAM[p][i] != value {
					t.Fatalf("state pack wrote outside slot/scratch/stack bank%d offset%04x", p, i)
				}
			}
		}
	}
	t.Logf("complete state stream encoded directly to %d bytes, exact/short/spare capacities", len(encoded))
	t.Log("1786 saved bytes match independent compact layout, including RNG, stream3, locals and all return/stack arrays")
}
