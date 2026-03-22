package ide

import (
	"os"
	"testing"
)

func tempDisk(t *testing.T, sizeKB int) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "disk*.img")
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(int64(sizeKB) * 1024)
	f.Close()
	return f.Name()
}

func TestNemo_IdentifyAndReadWrite(t *testing.T) {
	path := tempDisk(t, 1440) // 1.44MB floppy
	c, err := New(Nemo, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// IDENTIFY command
	c.WritePort(0xD0, 0xE0) // drive 0, LBA mode
	c.WritePort(0xF0, CmdIdentify)

	// Read status
	st, _ := c.ReadPort(0xF0)
	if st&StatusDRQ == 0 {
		t.Fatal("expected DRQ after IDENTIFY")
	}

	// Read first word (general config)
	lo, _ := c.ReadPort(0x10) // data low
	hi, _ := c.ReadPort(0x11) // data high (latch)
	t.Logf("identify word 0: 0x%02X%02X", hi, lo)

	// Write a sector
	c.WritePort(0x70, 0x01) // sector 1
	c.WritePort(0x90, 0x00) // cyl lo
	c.WritePort(0xB0, 0x00) // cyl hi
	c.WritePort(0x50, 0x01) // count = 1
	c.WritePort(0xD0, 0xE0) // drive 0, LBA

	// Fill sector buffer: write 256 words (512 bytes)
	c.WritePort(0xF0, CmdWriteSectors)
	st, _ = c.ReadPort(0xF0)
	if st&StatusDRQ == 0 {
		t.Fatal("expected DRQ after WRITE")
	}
	for i := 0; i < 256; i++ {
		c.WritePort(0x11, byte(i>>4))  // high byte first
		c.WritePort(0x10, byte(i&0xFF)) // low byte triggers transfer
	}

	// Read it back
	c.WritePort(0x70, 0x01) // same sector
	c.WritePort(0x50, 0x01)
	c.WritePort(0xD0, 0xE0)
	c.WritePort(0xF0, CmdReadSectors)

	st, _ = c.ReadPort(0xF0)
	if st&StatusDRQ == 0 {
		t.Fatal("expected DRQ after READ")
	}

	// Verify first few words
	for i := 0; i < 4; i++ {
		lo, _ := c.ReadPort(0x10) // data low
		hi, _ := c.ReadPort(0x11) // data high
		expect_lo := byte(i & 0xFF)
		expect_hi := byte(i >> 4)
		if lo != expect_lo || hi != expect_hi {
			t.Errorf("word %d: got 0x%02X%02X, want 0x%02X%02X", i, hi, lo, expect_hi, expect_lo)
		}
	}
}

func TestNemo_DecodePort(t *testing.T) {
	c := &Controller{iface: Nemo}
	tests := []struct {
		port uint16
		reg  int
		high bool
		ok   bool
	}{
		{0x10, 0, false, true}, // data low
		{0x11, 0, true, true},  // data high
		{0x30, 1, false, true}, // error
		{0x50, 2, false, true}, // sector count
		{0x70, 3, false, true}, // sector num
		{0x90, 4, false, true}, // cyl lo
		{0xB0, 5, false, true}, // cyl hi
		{0xD0, 6, false, true}, // drive/head
		{0xF0, 7, false, true}, // status/command
		{0x12, 0, false, false}, // bit 1 set — invalid
		{0x14, 0, false, false}, // bit 2 set — invalid
	}
	for _, tt := range tests {
		reg, high, ok := c.DecodePort(tt.port)
		if ok != tt.ok || (ok && (reg != tt.reg || high != tt.high)) {
			t.Errorf("DecodePort(0x%04X): got (%d, %v, %v), want (%d, %v, %v)",
				tt.port, reg, high, ok, tt.reg, tt.high, tt.ok)
		}
	}
}

func TestDivIDE_DecodePort(t *testing.T) {
	c := &Controller{iface: DivIDE}
	tests := []struct {
		port uint16
		reg  int
		ok   bool
	}{
		{0xA3, 0, true}, // data
		{0xA7, 1, true}, // error
		{0xAB, 2, true}, // sector count
		{0xAF, 3, true}, // sector num
		{0xB3, 4, true}, // cyl lo
		{0xB7, 5, true}, // cyl hi
		{0xBB, 6, true}, // drive/head
		{0xBF, 7, true}, // status/command
		{0x23, 0, false}, // not divIDE
	}
	for _, tt := range tests {
		reg, _, ok := c.DecodePort(tt.port)
		if ok != tt.ok || (ok && reg != tt.reg) {
			t.Errorf("DecodePort(0x%04X): got (%d, _, %v), want (%d, _, %v)",
				tt.port, reg, ok, tt.reg, tt.ok)
		}
	}
}

func TestSMUC_DecodePort(t *testing.T) {
	c := &Controller{iface: SMUC}
	tests := []struct {
		port uint16
		reg  int
		high bool
		ok   bool
	}{
		{0xF8BE, 0, false, true}, // data low
		{0xD8BE, 0, true, true},  // data high latch
		{0xF9BE, 1, false, true}, // error
		{0xFABE, 2, false, true}, // sector count
		{0xFBBE, 3, false, true}, // sector num
		{0xFCBE, 4, false, true}, // cyl lo
		{0xFDBE, 5, false, true}, // cyl hi
		{0xFEBE, 6, false, true}, // drive/head
		{0xFFBE, 7, false, true}, // status/command
		{0xF8BF, 0, false, false}, // wrong low byte
	}
	for _, tt := range tests {
		reg, high, ok := c.DecodePort(tt.port)
		if ok != tt.ok || (ok && (reg != tt.reg || high != tt.high)) {
			t.Errorf("DecodePort(0x%04X): got (%d, %v, %v), want (%d, %v, %v)",
				tt.port, reg, high, ok, tt.reg, tt.high, tt.ok)
		}
	}
}
