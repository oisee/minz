package pipeline

import (
	"bytes"
	"encoding/hex"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"strconv"
	"testing"
)

func TestZMachineArchiveStreamNative(t *testing.T) {
	data := make([]byte, 1024)
	for i := range data {
		if i%257 < 133 {
			data[i] = byte(i / 257)
		} else {
			data[i] = byte(i)
		}
	}
	command := exec.Command("python3", "-c", "from restart_rle import pack; import sys; print(pack(bytes.fromhex(sys.argv[1])).hex())", hex.EncodeToString(data))
	command.Dir = "../../../scripts/zmachine"
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	packed, err := hex.DecodeString(string(bytes.TrimSpace(output)))
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, bank := range []int{1, 3, 4, 6, 7} {
		offset := 16384 - len(packed)
		command = exec.Command("python3", "-c", "from native_shared_reader import emit; from native_save_stream import emit as pack; import sys; print(emit(int(sys.argv[1]),int(sys.argv[2]))+pack())", strconv.Itoa(0x10|bank), strconv.Itoa(offset))
		command.Dir = "../../../scripts/zmachine"
		generated, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		h, err := nanz.Parse(string(generated)+"\nfun native_state_byte(index:u16)->u8 { return 0 }\nfun main() -> u8 { return native_original_byte(0) }", "archive.nanz")
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
		copy(m.Memory.RAM[bank][offset:], packed)
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		delta := make([]byte, len(data))
		for i := range delta {
			delta[i] = byte(i / 133)
			m.Memory.RAM[0][i] = data[i] ^ delta[i]
		}
		before := m.Memory.RAM
		scratch := map[uint16]bool{}
		for name, size := range map[string]int{"original_cursor": 12, "saved_cursor": 12} {
			for i := 0; i < size; i++ {
				scratch[uint16(a.Symbols[name]+i)] = true
			}
		}
		var indices []int
		for i := range data {
			indices = append(indices, i, i)
		}
		indices = append(indices, 0, 130, 129, 1023, 511, 512, 0)
		for _, index := range indices {
			m.Memory.SetPagingForce(0x10)
			m.CPU.SetPC(uint16(a.Symbols["native_original_byte"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetHL(uint16(index))
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for i := 0; i < 100000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			if byte(m.CPU.AF()>>8) != data[index] || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
				t.Fatalf("bank%d index%d got%x want%x PC/SP%04x/%04x", bank, index, m.CPU.AF()>>8, data[index], m.CPU.PC(), m.CPU.SP())
			}
			cases++
		}

		command = exec.Command("python3", "-c", "from restart_rle import pack; import sys; print(pack(bytes.fromhex(sys.argv[1])).hex())", hex.EncodeToString(delta))
		command.Dir = "../../../scripts/zmachine"
		encodedHex, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := hex.DecodeString(string(bytes.TrimSpace(encodedHex)))
		if err != nil {
			t.Fatal(err)
		}
		for name, size := range map[string]int{"native_state_pack_begin": 2, "native_state_pack_literals": 1, "native_state_pack_bank": 1, "native_save_mode": 1} {
			for i := 0; i < size; i++ {
				scratch[uint16(a.Symbols[name]+i)] = true
			}
		}
		for _, capacity := range []int{len(encoded) - 1, len(encoded), len(encoded) + 1} {
			m.Memory.SetPagingForce(0x10)
			m.CPU.SetPC(uint16(a.Symbols["native_delta_pack"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetAF(0x1100)
			m.CPU.SetHL(0)
			m.CPU.SetDE(uint16(len(data)))
			m.CPU.SetIX(0)
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
				t.Fatalf("delta pack bank%d cap%d got%d want%d PC/SP%04x/%04x", bank, capacity, m.CPU.HL(), want, m.CPU.PC(), m.CPU.SP())
			}
			if want != 65535 && !bytes.Equal(m.Memory.RAM[1][:len(encoded)], encoded) {
				t.Fatal("delta encoded bytes mismatch")
			}
		}
		for p := 0; p < 8; p++ {
			for i, b := range before[p] {
				if p == 1 && i < len(encoded)+1 {
					continue
				}
				if p == 2 && (i >= 0x3ec0 && i < 0x3f00 || scratch[uint16(0x8000+i)]) {
					continue
				}
				if m.Memory.RAM[p][i] != b {
					t.Fatalf("archive reader wrote bank%d offset%x", p, i)
				}
			}
		}
	}
	t.Logf("%d indexed archive reads including repeated/backward requests", cases)
}
