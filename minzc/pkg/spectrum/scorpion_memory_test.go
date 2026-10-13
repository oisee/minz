package spectrum

import "testing"

// Reference mapping: Fuse machines/scorpion.c: bank=(1FFD&16)/2|(7FFD&7).
func TestScorpionMemoryPaging(t *testing.T) {
	m, err := NewScorpionMemory256(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for page := 0; page < 16; page++ {
		m.Memory.WriteRAMDirect(page, 123, byte(page+71))
	}
	for high := 0; high < 2; high++ {
		for low := 0; low < 8; low++ {
			m.Ports.WritePort(0x1ffd, byte(high<<4))
			m.Ports.WritePort(0x7ffd, byte(low|16))
			page := high*8 + low
			if m.Memory.PageHi() != page || m.Memory.Read(0xc07b) != byte(page+71) {
				t.Fatal("bank", page, m.Memory.PageHi())
			}
			m.Memory.Write(0xc07c, byte(page+1), false)
			if m.Memory.ReadRAMDirect(page, 124) != byte(page+1) {
				t.Fatal("write", page)
			}
			if m.Memory.Read(0x407b) != 76 || m.Memory.Read(0x807b) != 73 {
				t.Fatal("fixed banks changed")
			}
		}
	}
	m.Memory.ROM[0][42] = 10
	m.Memory.ROM[1][42] = 11
	m.Memory.ROM[2][42] = 12
	m.Ports.WritePort(0x1ffd, 2)
	if m.Memory.Read(42) != 12 {
		t.Fatal("ROM2")
	}
	m.Ports.WritePort(0x1ffd, 1)
	m.Memory.Write(42, 99, false)
	if m.Memory.Read(42) != 99 || m.Memory.RAM[0][42] != 99 {
		t.Fatal("RAM0 overlay")
	}
	if m.Memory.Data()[42] != 99 {
		t.Fatal("flat view")
	}
	m.Ports.WritePort(0x1ffd, 0)
	if m.Memory.Read(42) != 11 {
		t.Fatal("ROM restored")
	}
	m.Ports.WritePort(0x7ffd, 0x3b)
	before := m.Memory.PageHi()
	m.Ports.WritePort(0x1ffd, 0x10)
	m.Ports.WritePort(0x7ffd, 0)
	if m.Memory.PageHi() != before || m.Memory.ScorpionPagingState() != 0 || m.Memory.ScreenPage() != 7 {
		t.Fatal("lock")
	}
	m.Memory.ResetPaging()
	if m.Memory.PageHi() != 0 || m.Memory.ScorpionPagingState() != 0 || m.Memory.PagingState() != 0 {
		t.Fatal("reset")
	}
}

func TestScorpionExtensionAbsentOn128K(t *testing.T) {
	m, err := NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Memory.SetScorpionPaging(16)
	m.Memory.WriteRAMDirect(8, 0, 42)
	if m.Memory.ReadRAMDirect(8, 0) != 0 || m.Memory.ScorpionPagingState() != 0 {
		t.Fatal("extension active")
	}
}

func TestScorpionNativeBankSwitch(t *testing.T) {
	m, err := NewScorpionMemory256(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Execute OUT (C),A from fixed RAM, select bank13, store a byte, return bank0.
	code := []byte{0x01, 0xfd, 0x1f, 0x3e, 0x10, 0xed, 0x79, 0x01, 0xfd, 0x7f, 0x3e, 0x15, 0xed, 0x79, 0x3e, 0xa7, 0x32, 0x7b, 0xc0, 0x01, 0xfd, 0x1f, 0xaf, 0xed, 0x79, 0x01, 0xfd, 0x7f, 0x3e, 0x10, 0xed, 0x79, 0xc9}
	for i, b := range code {
		m.Memory.Write(0x8000+uint16(i), b, false)
	}
	m.Memory.Write(0xbf00, 0, false)
	m.Memory.Write(0xbf01, 0x70, false)
	m.CPU.SetPC(0x8000)
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	for i := 0; i < 100; i++ {
		if m.CPU.PC() == 0x7000 {
			break
		}
		m.CPU.DoOpcode()
	}
	if m.Memory.ReadRAMDirect(13, 123) != 0xa7 || m.Memory.ReadRAMDirect(5, 123) != 0 || m.Memory.PageHi() != 0 || m.Memory.PagingState() != 0x10 || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 {
		t.Fatal("native paging/return failed")
	}
}
