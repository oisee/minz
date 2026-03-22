// Package ide emulates IDE/ATA hard disk controllers for Z80 systems.
//
// Supports three interface variants:
//   - Nemo IDE (KAY, NedoPC ZX-BUS) — simplest, ports 0x10-0xF0
//   - divIDE (Baze, edge connector) — universal, ports 0xA3-0xBF
//   - SMUC (Scorpion ZS-256) — ports 0xF8BE-0xFFBE
//
// All use the same 8-bit latch principle for 16-bit IDE data register.
// Backed by a disk image file (raw sector access, 512 bytes/sector).
package ide

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// ATA command block register offsets.
const (
	RegData      = 0 // Data (16-bit via latch)
	RegError     = 1 // Error (read) / Features (write)
	RegSectCount = 2
	RegSectNum   = 3 // LBA[7:0]
	RegCylLo     = 4 // LBA[15:8]
	RegCylHi     = 5 // LBA[23:16]
	RegDriveHead = 6 // LBA[27:24] + drive select
	RegStatus    = 7 // Status (read) / Command (write)
)

// ATA status bits.
const (
	StatusBSY  = 0x80 // Busy
	StatusDRDY = 0x40 // Drive ready
	StatusDRQ  = 0x08 // Data request
	StatusERR  = 0x01 // Error
)

// ATA commands.
const (
	CmdIdentify    = 0xEC
	CmdReadSectors = 0x20
	CmdWriteSectors = 0x30
)

// Interface selects the port decoding scheme.
type Interface int

const (
	Nemo   Interface = iota // KAY, NedoPC — ports 0x10-0xF0, high byte on port|1
	DivIDE                  // Baze — ports 0xA3-0xBF, toggle hi/lo
	SMUC                    // Scorpion ZS-256 — ports 0xF8BE-0xFFBE
)

// Controller emulates an IDE/ATA device backed by a disk image file.
type Controller struct {
	mu        sync.Mutex
	iface     Interface
	file      *os.File
	size      int64 // image size in bytes
	verbose   bool

	// ATA registers
	regs      [8]byte
	highLatch byte // 16-bit data high byte latch

	// DivIDE toggle state
	divideToggle bool

	// Sector buffer for PIO transfer
	buf       [512]byte
	bufPos    int  // current byte position in buffer
	bufLen    int  // valid bytes in buffer
	writing   bool // true during write transfer
}

// New creates an IDE controller backed by the given disk image file.
func New(iface Interface, imgPath string, verbose bool) (*Controller, error) {
	f, err := os.OpenFile(imgPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("ide: open %s: %w", imgPath, err)
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("ide: stat %s: %w", imgPath, err)
	}

	c := &Controller{
		iface:   iface,
		file:    f,
		size:    fi.Size(),
		verbose: verbose,
	}
	// Drive ready, not busy
	c.regs[RegStatus] = StatusDRDY
	return c, nil
}

// Close releases the disk image file.
func (c *Controller) Close() error {
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// DecodePort returns (register index, isHighByte, handled) for a Z80 port address.
func (c *Controller) DecodePort(port uint16) (reg int, highByte bool, ok bool) {
	switch c.iface {
	case Nemo:
		// Nemo: bits 1,2 must be 0, register = bits 7..5
		if port&0x06 != 0 {
			return 0, false, false
		}
		lo := byte(port & 0xFF)
		if lo < 0x10 || lo > 0xF8 {
			return 0, false, false
		}
		reg = int((lo >> 5) & 7)
		highByte = (lo & 1) == 1 && reg == 0 // high byte only for data reg
		return reg, highByte, true

	case DivIDE:
		// divIDE: (port & 0xA3) == 0xA3, register = bits 4..2
		if byte(port&0xFF)&0xA3 != 0xA3 {
			return 0, false, false
		}
		reg = int((port >> 2) & 7)
		return reg, false, true // toggle handled internally

	case SMUC:
		// SMUC: low byte == 0xBE, register = bits 10..8
		if byte(port&0xFF) != 0xBE {
			return 0, false, false
		}
		hi := byte(port >> 8)
		if hi&0xF8 != 0xF8 && hi != 0xD8 {
			return 0, false, false
		}
		if hi == 0xD8 {
			return 0, true, true // high byte latch
		}
		reg = int(hi & 7)
		return reg, false, true
	}
	return 0, false, false
}

// ReadPort handles IN from an IDE port. Returns (value, handled).
func (c *Controller) ReadPort(port uint16) (byte, bool) {
	reg, highByte, ok := c.DecodePort(port)
	if !ok {
		return 0xFF, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// High byte latch read
	if highByte {
		return c.highLatch, true
	}

	// DivIDE toggle: odd access = low, even = high
	if c.iface == DivIDE && reg == RegData {
		c.divideToggle = !c.divideToggle
		if !c.divideToggle {
			return c.highLatch, true
		}
	}

	if reg == RegData {
		return c.readDataByte(), true
	}

	return c.regs[reg], true
}

// WritePort handles OUT to an IDE port. Returns handled.
func (c *Controller) WritePort(port uint16, val byte) bool {
	reg, highByte, ok := c.DecodePort(port)
	if !ok {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// High byte latch write
	if highByte {
		c.highLatch = val
		return true
	}

	// DivIDE toggle
	if c.iface == DivIDE && reg == RegData {
		c.divideToggle = !c.divideToggle
		if !c.divideToggle {
			c.highLatch = val
			return true
		}
	}

	if reg == RegData {
		c.writeDataByte(val)
		return true
	}

	if reg == RegStatus {
		// Write to reg 7 = command
		c.regs[reg] = val
		c.execCommand(val)
		return true
	}

	c.regs[reg] = val
	return true
}

// ── Internal: sector I/O ────────────────────────────────────────────────────

func (c *Controller) lba() int64 {
	lba := int64(c.regs[RegSectNum])
	lba |= int64(c.regs[RegCylLo]) << 8
	lba |= int64(c.regs[RegCylHi]) << 16
	lba |= int64(c.regs[RegDriveHead]&0x0F) << 24
	return lba
}

func (c *Controller) execCommand(cmd byte) {
	switch cmd {
	case CmdReadSectors:
		sector := c.lba()
		count := int(c.regs[RegSectCount])
		if count == 0 {
			count = 256
		}
		offset := sector * 512
		n, err := c.file.ReadAt(c.buf[:512], offset)
		if err != nil && err != io.EOF {
			if c.verbose {
				fmt.Fprintf(os.Stderr, "ide: read sector %d: %v\n", sector, err)
			}
			c.regs[RegStatus] = StatusDRDY | StatusERR
			return
		}
		// Zero-fill short reads
		for i := n; i < 512; i++ {
			c.buf[i] = 0
		}
		c.bufPos = 0
		c.bufLen = 512
		c.writing = false
		c.regs[RegStatus] = StatusDRDY | StatusDRQ
		if c.verbose {
			fmt.Fprintf(os.Stderr, "ide: READ sector=%d count=%d\n", sector, count)
		}

	case CmdWriteSectors:
		c.bufPos = 0
		c.bufLen = 512
		c.writing = true
		c.regs[RegStatus] = StatusDRDY | StatusDRQ
		if c.verbose {
			sector := c.lba()
			count := int(c.regs[RegSectCount])
			if count == 0 {
				count = 256
			}
			fmt.Fprintf(os.Stderr, "ide: WRITE sector=%d count=%d\n", sector, count)
		}

	case CmdIdentify:
		// Return minimal identify data
		for i := range c.buf {
			c.buf[i] = 0
		}
		// Word 0: general config
		c.buf[0] = 0x40
		c.buf[1] = 0x00
		// Words 1: cylinders (for CHS)
		sectors := c.size / 512
		cyls := sectors / (16 * 63)
		if cyls > 16383 {
			cyls = 16383
		}
		c.buf[2] = byte(cyls)
		c.buf[3] = byte(cyls >> 8)
		// Word 3: heads
		c.buf[6] = 16
		c.buf[7] = 0
		// Word 6: sectors per track
		c.buf[12] = 63
		c.buf[13] = 0
		// Words 27-46: model string "MinZ IDE Disk"
		model := "MinZ IDE Disk                           "
		for i := 0; i < 40 && i < len(model); i += 2 {
			c.buf[54+i] = model[i+1]
			c.buf[54+i+1] = model[i]
		}
		// Word 49: capabilities (LBA supported)
		c.buf[98] = 0x00
		c.buf[99] = 0x02 // LBA
		// Words 60-61: total sectors (LBA)
		c.buf[120] = byte(sectors)
		c.buf[121] = byte(sectors >> 8)
		c.buf[122] = byte(sectors >> 16)
		c.buf[123] = byte(sectors >> 24)

		c.bufPos = 0
		c.bufLen = 512
		c.writing = false
		c.regs[RegStatus] = StatusDRDY | StatusDRQ
		if c.verbose {
			fmt.Fprintf(os.Stderr, "ide: IDENTIFY (%d sectors, %d MB)\n", sectors, sectors/2048)
		}

	default:
		if c.verbose {
			fmt.Fprintf(os.Stderr, "ide: unknown command 0x%02X\n", cmd)
		}
		c.regs[RegStatus] = StatusDRDY | StatusERR
		c.regs[RegError] = 0x04 // abort
	}
}

func (c *Controller) readDataByte() byte {
	if c.bufPos >= c.bufLen {
		return 0xFF
	}
	lo := c.buf[c.bufPos]
	c.bufPos++
	if c.bufPos < c.bufLen {
		c.highLatch = c.buf[c.bufPos]
		c.bufPos++
	}
	if c.bufPos >= c.bufLen {
		// Transfer complete
		c.regs[RegStatus] = StatusDRDY
		// TODO: multi-sector — load next sector
	}
	return lo
}

func (c *Controller) writeDataByte(lo byte) {
	if c.bufPos >= c.bufLen {
		return
	}
	c.buf[c.bufPos] = lo
	c.bufPos++
	if c.bufPos < c.bufLen {
		c.buf[c.bufPos] = c.highLatch
		c.bufPos++
	}
	if c.bufPos >= c.bufLen {
		// Sector complete — flush to disk
		sector := c.lba()
		offset := sector * 512
		if _, err := c.file.WriteAt(c.buf[:512], offset); err != nil {
			if c.verbose {
				fmt.Fprintf(os.Stderr, "ide: write flush sector %d: %v\n", sector, err)
			}
			c.regs[RegStatus] = StatusDRDY | StatusERR
			return
		}
		c.regs[RegStatus] = StatusDRDY
		if c.verbose {
			fmt.Fprintf(os.Stderr, "ide: flushed sector %d\n", sector)
		}
	}
}
