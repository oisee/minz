package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

// Independent host parser accepts exact input/output lengths. Native preflight
// must produce the same decision without touching any physical data bank.
func TestZMachineRLEValidationNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.banked\nfun main() -> u8 { return bank_validate_rle(7,0,0,0) }", "validate.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	oracle := func(data []byte, size int) bool {
		at, out := 0, 0
		for out < size {
			if at == len(data) {
				return false
			}
			token := int(data[at])
			at++
			if token < 128 {
				n := token + 1
				if at+n > len(data) {
					return false
				}
				at += n
				out += n
			} else {
				if at == len(data) {
					return false
				}
				at++
				out += (token & 127) + 3
			}
			if out > size {
				return false
			}
		}
		return at == len(data)
	}
	cases := 0
	for token := 0; token < 256; token++ {
		n := token + 1
		if token >= 128 {
			n = 1
		}
		payload := append([]byte{byte(token)}, make([]byte, n)...)
		decoded := token + 1
		if token >= 128 {
			decoded = (token & 127) + 3
		}
		for _, cut := range []int{0, 1, len(payload) - 1, len(payload)} {
			for _, size := range []int{0, decoded - 1, decoded, decoded + 1} {
				data := payload[:cut]
				offsets := []int{0, 16384 - len(data)}
				if token == 0 {
					offsets = append(offsets, 16385, 65535, 16384-len(data)+1)
				}
				for _, offset := range offsets {

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
					if offset <= 16384 {
						copy(m.Memory.RAM[7][offset:], data)
					}
					m.Memory.Write(0xbf00, 0, false)
					m.Memory.Write(0xbf01, 0x70, false)
					before := m.Memory.RAM
					m.Memory.SetPagingForce(0x10)
					m.CPU.SetPC(uint16(a.Symbols["zx__banked__bank_validate_rle"]))
					m.CPU.SetSP(0xbf00)
					m.CPU.SetAF(0x1700)
					m.CPU.SetHL(uint16(offset))
					m.CPU.SetDE(uint16(len(data)))
					m.CPU.SetIX(uint16(size))
					m.CPU.SetIFF1(false)
					m.CPU.SetIFF2(false)
					for i := 0; i < 10000; i++ {
						if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
							break
						}
						m.CPU.DoOpcode()
					}
					want := byte(0)
					if offset <= 16384 && offset+len(data) <= 16384 && oracle(data, size) {
						want = 1
					}
					if byte(m.CPU.AF()>>8) != want || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
						t.Fatalf("token%d cut%d size%d offset%d got%02x want%d PC/SP%04x/%04x", token, cut, size, offset, m.CPU.AF()>>8, want, m.CPU.PC(), m.CPU.SP())
					}
					for p := 0; p < 8; p++ {
						for i, v := range before[p] {
							if p == 2 && i >= 0x3ef0 && i < 0x3f00 {
								continue
							}
							if m.Memory.RAM[p][i] != v {
								t.Fatalf("validator wrote bank%d offset%04x", p, i)
							}
						}
					}
					cases++
				}
			}
		}
	}
	t.Logf("%d validation cases", cases)
}
