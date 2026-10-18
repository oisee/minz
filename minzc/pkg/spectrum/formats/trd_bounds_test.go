package formats

import (
	"bytes"
	"github.com/minz/minzc/pkg/spectrum"
	"testing"
)

func TestTRDCoordinateBounds(t *testing.T) {
	d := &TRDFile{}
	for i := range d.Data {
		d.Data[i] = 0xa7
	}
	before := d.Data
	maximum := int(^uint(0) >> 1)
	for _, c := range [][3]int{{0, 0, 16}, {0, 2, 0}, {1, -1, 0}, {80, 0, 0}, {-1, 1, 0}, {maximum, maximum, maximum}, {0, 0, -1}} {
		if !bytes.Equal(d.ReadSector(c[0], c[1], c[2]), make([]byte, 256)) {
			t.Fatal("invalid read aliases valid data", c)
		}
		d.WriteSector(c[0], c[1], c[2], bytes.Repeat([]byte{9}, 256))
		if d.Data != before {
			t.Fatal("invalid write modifies disk", c)
		}
	}
}
func TestTRDTrapTransferBounds(t *testing.T) {
	for _, fn := range []byte{5, 6} {
		for _, c := range []struct {
			name                 string
			track, sector, count byte
			address              uint16
			valid                bool
		}{
			{"zero-sector", 0, 0, 1, 0x9000, false}, {"sector17", 0, 17, 1, 0x9000, false}, {"sector255", 0, 255, 1, 0x9000, false},
			{"track160", 160, 1, 1, 0x9000, false}, {"track255", 255, 1, 1, 0x9000, false},
			{"past-last-sector", 159, 16, 2, 0x9000, false}, {"last-sector", 159, 16, 1, 0x9000, true},
			{"head-rollover-legacy", 79, 16, 2, 0x9000, true}, {"track2-legacy", 2, 1, 1, 0x9000, true},
			{"side1-legacy", 80, 1, 1, 0x9000, true}, {"buffer-wrap", 0, 1, 2, 0xff00, false},
			{"zero-count-no-transfer", 255, 255, 0, 0x9000, true},
		} {
			t.Run(string(rune('0'+fn))+"/"+c.name, func(t *testing.T) {
				d := &TRDFile{}
				for i := range d.Data {
					d.Data[i] = byte(i/256 + 7)
				}
				before := d.Data
				m, e := spectrum.NewPentagon128(nil, nil)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0; i < 1024; i++ {
					m.Memory.Write(0x9000+uint16(i), 0xa5, false)
				}
				m.Memory.Write(0x7000, 0x76, false)
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				m.CPU.SetPC(TRDOSEntryPoint)
				m.CPU.SetSP(0xbf00)
				m.CPU.SetHL(c.address)
				m.CPU.SetBC(uint16(c.count)<<8 | uint16(fn))
				m.CPU.SetDE(uint16(c.track)<<8 | uint16(c.sector))
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				var beforeRAM [49152]byte
				for i := range beforeRAM {
					beforeRAM[i] = m.Memory.Read(0x4000 + uint16(i))
				}
				InstallTRDTraps(m, d, false)
				m.RunFrameFast()
				failed := m.CPU.BC()>>8 != 0
				if failed == c.valid {
					t.Fatalf("BC=%04x valid=%v", m.CPU.BC(), c.valid)
				}
				if m.CPU.SP() != 0xbf02 {
					t.Fatal("return stack", m.CPU.SP())
				}
				if !c.valid || c.count == 0 {
					if d.Data != before {
						t.Fatal("refusal partially changed disk")
					}
					for i, old := range beforeRAM {
						if m.Memory.Read(0x4000+uint16(i)) != old {
							t.Fatal("refusal partially changed RAM", i)
						}
					}
				} else if c.address == 0x9000 {
					for i := 0; i < int(c.count); i++ {
						logical := int(c.track)*16 + int(c.sector) - 1 + i
						track := logical / 16
						side := 0
						if track >= 80 {
							track -= 80
							side = 1
						}
						sector := logical % 16
						if fn == 5 {
							want := before[((track*2+side)*16+sector)*256]
							if m.Memory.Read(0x9000+uint16(i*256)) != want {
								t.Fatal("legacy mapping changed", track, side, sector)
							}
						} else if d.ReadSector(track, side, sector)[0] != 0xa5 {
							t.Fatal("write mapping changed")
						}
					}
				}
			})
		}
	}
}
