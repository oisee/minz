package main

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
)

func TestTimeoutExecutionLimit(t *testing.T) {
	for _, tc := range []struct {
		timeout uint
		want    int
	}{{0, -1}, {1234, 1234}} {
		cpu := emulator.NewRemogattoZ80()
		setExecutionLimit(cpu, tc.timeout)
		if cpu.MaxCycles != tc.want {
			t.Errorf("timeout %d: MaxCycles = %d, want %d", tc.timeout, cpu.MaxCycles, tc.want)
		}
	}
}

func TestParseBankLoad(t *testing.T) {
	for _, tc := range []struct {
		spec   string
		file   string
		bank   int
		offset uint16
		bad    bool
	}{
		{"w.bin@3", "w.bin", 3, 0, false},
		{"dir@x/w.bin@12:C100", "dir@x/w.bin", 12, 0x0100, false},
		{"w.bin@1:$20", "w.bin", 1, 0x20, false},
		{"w.bin", "", 0, 0, true},
		{"w.bin@", "", 0, 0, true},
		{"w.bin@x", "", 0, 0, true},
		{"w.bin@1:zz", "", 0, 0, true},
	} {
		file, bank, offset, err := parseBankLoad(tc.spec)
		if (err != nil) != tc.bad {
			t.Errorf("%q: err = %v, want error %v", tc.spec, err, tc.bad)
			continue
		}
		if !tc.bad && (file != tc.file || bank != tc.bank || offset != tc.offset) {
			t.Errorf("%q: got %q %d %#x", tc.spec, file, bank, offset)
		}
	}
}

func TestSetupPagingLoadsBanks(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/w.bin"
	if err := os.WriteFile(path, []byte{0xAB, 0xCD}, 0o644); err != nil {
		t.Fatal(err)
	}
	cpu := emulator.NewRemogattoZ80()
	if err := setupPaging(cpu, "scorpion-memory", []string{path + "@10:4"}); err != nil {
		t.Fatal(err)
	}
	if cpu.ReadBank(10, 4) != 0xAB || cpu.ReadBank(10, 5) != 0xCD {
		t.Fatalf("bank 10 = %#02x %#02x", cpu.ReadBank(10, 4), cpu.ReadBank(10, 5))
	}
	if err := setupPaging(emulator.NewRemogattoZ80(), "128k", []string{path + "@10"}); err == nil {
		t.Error("bank 10 on a 128K must be rejected")
	}
	if err := setupPaging(emulator.NewRemogattoZ80(), "48k", []string{path + "@1"}); err == nil {
		t.Error("--load-bank on the flat model must be rejected")
	}
	if err := setupPaging(emulator.NewRemogattoZ80(), "zx81", nil); err == nil {
		t.Error("unknown model must be rejected")
	}
}
