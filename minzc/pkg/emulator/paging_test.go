package emulator

import "testing"

// runPaged assembles nothing: the programs are hand-encoded so the test does
// not depend on the assembler. Each ends with DI / HALT.
func runPaged(t *testing.T, model PagingModel, code []byte) *RemogattoZ80 {
	t.Helper()
	z := NewRemogattoZ80()
	z.EnablePaging(model)
	if err := z.LoadMemory(0x8000, code); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xBFF0)
	z.MaxCycles = 100000
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	return z
}

// outBC emits LD BC,port / LD A,v / OUT (C),A.
func outBC(port uint16, v byte) []byte {
	return []byte{0x01, byte(port), byte(port >> 8), 0x3E, v, 0xED, 0x79}
}

// poke emits LD A,v / LD (addr),A.
func poke(addr uint16, v byte) []byte {
	return []byte{0x3E, v, 0x32, byte(addr), byte(addr >> 8)}
}

func prog(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return append(out, 0xF3, 0x76) // DI / HALT
}

func TestPaging128KBanksAreSeparate(t *testing.T) {
	z := runPaged(t, Paging128K, prog(
		poke(0xC000, 0x10), // bank 0
		outBC(0x7FFD, 1), poke(0xC000, 0x11),
		outBC(0x7FFD, 6), poke(0xC000, 0x16),
		outBC(0x7FFD, 1), // leave bank 1 mapped
	))
	if z.PageHi() != 1 {
		t.Fatalf("PageHi = %d, want 1", z.PageHi())
	}
	for bank, want := range map[int]byte{0: 0x10, 1: 0x11, 6: 0x16, 3: 0x00} {
		if got := z.ReadBank(bank, 0); got != want {
			t.Errorf("bank %d byte 0 = %#02x, want %#02x", bank, got, want)
		}
	}
	if got := z.GetMemory(0xC000); got != 0x11 {
		t.Errorf("mapped byte = %#02x, want bank 1's 0x11", got)
	}
}

func TestPagingFixedBanksAliasTheirSlots(t *testing.T) {
	z := runPaged(t, Paging128K, prog(
		poke(0x4010, 0x55), // bank 5 through its fixed slot
		outBC(0x7FFD, 5),
		poke(0xC011, 0x66), // bank 5 through $C000
		poke(0x4012, 0x77), // fixed slot again while aliased
		outBC(0x7FFD, 2),
		poke(0xC020, 0x22), // bank 2 through $C000
		outBC(0x7FFD, 0),
	))
	for _, c := range []struct {
		addr uint16
		want byte
	}{{0x4010, 0x55}, {0x4011, 0x66}, {0x4012, 0x77}, {0x8020, 0x22}, {0xC010, 0}, {0xC020, 0}} {
		if got := z.GetMemory(c.addr); got != c.want {
			t.Errorf("mem[%#04x] = %#02x, want %#02x", c.addr, got, c.want)
		}
	}
	if got := z.ReadBank(5, 0x12); got != 0x77 {
		t.Errorf("bank 5 offset 0x12 = %#02x, want 0x77", got)
	}
}

func TestPagingLockBitFreezesTheMap(t *testing.T) {
	z := runPaged(t, Paging128K, prog(
		outBC(0x7FFD, 0x20|3),
		outBC(0x7FFD, 4),
		poke(0xC000, 0x99),
	))
	if z.PageHi() != 3 || z.ReadBank(3, 0) != 0x99 || z.ReadBank(4, 0) != 0 {
		t.Fatalf("after lock: hi=%d bank3=%#02x bank4=%#02x", z.PageHi(), z.ReadBank(3, 0), z.ReadBank(4, 0))
	}
}

func TestPagingScorpionExtendedBanks(t *testing.T) {
	z := runPaged(t, PagingScorpion256, prog(
		outBC(0x7FFD, 3), poke(0xC000, 0x03),
		outBC(0x1FFD, 0x10), poke(0xC000, 0x0B), // bank 3+8
		outBC(0x7FFD, 7), poke(0xC000, 0x0F), // bank 15
		outBC(0x1FFD, 0x00), poke(0xC000, 0x07), // bank 7
	))
	if z.PagedBanks() != 16 {
		t.Fatalf("PagedBanks = %d, want 16", z.PagedBanks())
	}
	for bank, want := range map[int]byte{3: 0x03, 11: 0x0B, 15: 0x0F, 7: 0x07} {
		if got := z.ReadBank(bank, 0); got != want {
			t.Errorf("bank %d = %#02x, want %#02x", bank, got, want)
		}
	}
}

// On a plain 128K, $1FFD has A15 and A1 low, so it decodes as $7FFD.
func TestPaging128KDecodes1FFDAs7FFD(t *testing.T) {
	z := runPaged(t, Paging128K, prog(outBC(0x1FFD, 4)))
	if z.PageHi() != 4 {
		t.Fatalf("PageHi = %d, want 4", z.PageHi())
	}
	if z.PagedBanks() != 8 {
		t.Fatalf("PagedBanks = %d, want 8", z.PagedBanks())
	}
}

func TestFlatModelIgnoresPagingPorts(t *testing.T) {
	z := runPaged(t, PagingNone, prog(
		poke(0xC000, 0x10), outBC(0x7FFD, 1),
	))
	if got := z.GetMemory(0xC000); got != 0x10 {
		t.Fatalf("flat memory changed under a $7FFD write: %#02x", got)
	}
	if err := z.LoadBank(1, 0, []byte{1}); err == nil {
		t.Fatal("LoadBank on the flat model must fail")
	}
}

func TestLoadBankReachesMappedAndUnmappedBanks(t *testing.T) {
	z := NewRemogattoZ80()
	z.EnablePaging(PagingScorpion256)
	for _, bank := range []int{0, 2, 5, 9} {
		if err := z.LoadBank(bank, 0x100, []byte{byte(0xA0 + bank)}); err != nil {
			t.Fatal(err)
		}
	}
	if z.GetMemory(0xC100) != 0xA0 || z.GetMemory(0x8100) != 0xA2 || z.GetMemory(0x4100) != 0xA5 {
		t.Fatalf("mapped banks: %#02x %#02x %#02x", z.GetMemory(0xC100), z.GetMemory(0x8100), z.GetMemory(0x4100))
	}
	if z.ReadBank(9, 0x100) != 0xA9 {
		t.Fatalf("bank 9 = %#02x", z.ReadBank(9, 0x100))
	}
	if err := z.LoadBank(16, 0, []byte{1}); err == nil {
		t.Error("bank 16 must be rejected")
	}
	if err := z.LoadBank(1, 0x3FFF, []byte{1, 2}); err == nil {
		t.Error("a load crossing the bank end must be rejected")
	}
}
