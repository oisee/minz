package emulator

import "fmt"

// PagingModel selects the banked-RAM map the headless emulator decodes.
type PagingModel int

const (
	// PagingNone is the flat 64K map (the default).
	PagingNone PagingModel = iota
	// Paging128K is the ZX Spectrum 128K map: eight 16K banks, port $7FFD.
	Paging128K
	// PagingScorpion256 adds Scorpion ZS-256 banks 8..15 through port $1FFD
	// bit 4. Only the RAM map is modelled: no ROM switching, no RAM0 over ROM.
	PagingScorpion256
)

const bankSize = 0x4000

// ParsePagingModel maps the --model spelling shared with mzx.
func ParsePagingModel(s string) (PagingModel, error) {
	switch s {
	case "", "48k":
		return PagingNone, nil
	case "128k", "pentagon":
		return Paging128K, nil
	case "scorpion-memory":
		return PagingScorpion256, nil
	}
	return PagingNone, fmt.Errorf("unknown model %q (48k, 128k, pentagon, scorpion-memory)", s)
}

// pager keeps the banks that are not mapped. The flat Memory.data array stays
// authoritative for what the CPU sees: bank 5 at $4000, bank 2 at $8000 and
// bank hi at $C000. Switching copies 16K in and out, so every reader of the
// flat array (profiler snapshot, loaders, hooks) keeps working unchanged.
type pager struct {
	model    PagingModel
	banks    [][bankSize]byte
	hi       int // bank mapped at $C000
	port7FFD byte
	port1FFD byte
	locked   bool
}

// fixedSlot returns the flat address where a bank is permanently mapped.
func fixedSlot(bank int) (uint16, bool) {
	switch bank {
	case 5:
		return 0x4000, true
	case 2:
		return 0x8000, true
	}
	return 0, false
}

// EnablePaging switches the memory to a banked map. Bank 0 starts at $C000 and
// takes over whatever the flat array already holds there.
func (z *RemogattoZ80) EnablePaging(model PagingModel) {
	if model == PagingNone {
		z.memory.pager = nil
		z.ports.pager = nil
		return
	}
	n := 8
	if model == PagingScorpion256 {
		n = 16
	}
	p := &pager{model: model, banks: make([][bankSize]byte, n)}
	z.memory.pager = p
	z.ports.pager = p
	z.ports.pagedMem = z.memory
}

// PagedBanks reports how many 16K RAM banks the current model has (0 = flat).
func (z *RemogattoZ80) PagedBanks() int {
	if z.memory.pager == nil {
		return 0
	}
	return len(z.memory.pager.banks)
}

// PageHi reports the bank mapped at $C000 (0 when paging is off).
func (z *RemogattoZ80) PageHi() int {
	if z.memory.pager == nil {
		return 0
	}
	return z.memory.pager.hi
}

// LoadBank writes data into a physical bank at the given offset, whether or
// not the bank is currently mapped.
func (z *RemogattoZ80) LoadBank(bank int, offset uint16, data []byte) error {
	m := z.memory
	if m.pager == nil {
		return fmt.Errorf("paged load requires --model 128k or scorpion-memory")
	}
	if bank < 0 || bank >= len(m.pager.banks) {
		return fmt.Errorf("bank %d out of range (model has %d banks)", bank, len(m.pager.banks))
	}
	if int(offset)+len(data) > bankSize {
		return fmt.Errorf("load crosses the 16K bank boundary")
	}
	for i, b := range data {
		m.setBank(bank, offset+uint16(i), b)
	}
	return nil
}

// ReadBank reads one byte of a physical bank.
func (z *RemogattoZ80) ReadBank(bank int, offset uint16) byte {
	m := z.memory
	offset &= bankSize - 1
	if m.pager == nil || bank < 0 || bank >= len(m.pager.banks) {
		return 0xFF
	}
	if base, ok := fixedSlot(bank); ok {
		return m.data[base+offset]
	}
	if bank == m.pager.hi {
		return m.data[0xC000+offset]
	}
	return m.pager.banks[bank][offset]
}

func (m *Memory) setBank(bank int, offset uint16, v byte) {
	p := m.pager
	if base, ok := fixedSlot(bank); ok {
		m.data[base+offset] = v
		if bank == p.hi {
			m.data[0xC000+offset] = v
		}
		return
	}
	if bank == p.hi {
		m.data[0xC000+offset] = v
		return
	}
	p.banks[bank][offset] = v
}

// mirror keeps the two views of bank 5 or 2 equal while that bank is also
// mapped at $C000.
func (m *Memory) mirror(address uint16, v byte) {
	base, ok := fixedSlot(m.pager.hi)
	if !ok {
		return
	}
	switch {
	case address >= 0xC000:
		m.data[base+(address-0xC000)] = v
	case address >= base && address < base+bankSize:
		m.data[0xC000+(address-base)] = v
	}
}

// selectHi maps another bank at $C000.
func (m *Memory) selectHi(bank int) {
	p := m.pager
	if bank == p.hi {
		return
	}
	hiSlot := m.data[0xC000:]
	if _, fixed := fixedSlot(p.hi); !fixed {
		copy(p.banks[p.hi][:], hiSlot)
	}
	if base, fixed := fixedSlot(bank); fixed {
		copy(hiSlot, m.data[base:base+bankSize])
	} else {
		copy(hiSlot, p.banks[bank][:])
	}
	p.hi = bank
}

// writePagingPort decodes $7FFD and $1FFD. It reports whether the write was a
// paging port.
func (p *pager) writePagingPort(m *Memory, address uint16, v byte) bool {
	is7FFD := address&0x8002 == 0
	is1FFD := false
	if p.model == PagingScorpion256 {
		is7FFD = address&0xC002 == 0x4000
		is1FFD = address&0xF002 == 0x1000
	}
	if !is7FFD && !is1FFD {
		return false
	}
	if p.locked {
		return true
	}
	if is7FFD {
		p.port7FFD = v
	} else {
		p.port1FFD = v
	}
	bank := int(p.port7FFD & 0x07)
	if p.model == PagingScorpion256 {
		bank |= int(p.port1FFD&0x10) >> 1
	}
	m.selectHi(bank)
	if is7FFD && v&0x20 != 0 {
		p.locked = true
	}
	return true
}
