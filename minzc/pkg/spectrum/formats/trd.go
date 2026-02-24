package formats

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/spectrum"
)

// TRD disk geometry constants.
const (
	TRDSectorSize     = 256
	TRDSectorsPerTrack = 16
	TRDSidesPerDisk   = 2
	TRDTracksPerSide  = 80
	TRDDiskSize       = TRDSectorSize * TRDSectorsPerTrack * TRDSidesPerDisk * TRDTracksPerSide // 655360

	// Directory is on track 0, sectors 0-7 (128 entries, 16 bytes each)
	TRDDirEntrySize = 16
	TRDMaxDirEntries = 128

	// System sector: track 0, sector 8
	TRDSysSectorOffset = 8 * TRDSectorSize
)

// TRDFile represents a .trd disk image.
type TRDFile struct {
	Data [TRDDiskSize]byte
	Path string
}

// TRDEntry is a directory entry on a TR-DOS disk.
type TRDEntry struct {
	Name      string // 8 chars
	Extension byte   // 'B'=BASIC, 'C'=Code, 'D'=Data, '#'=Sequential
	Start     uint16 // start address (Code) or autostart line (BASIC)
	Length    uint16 // length in bytes
	Sectors   byte   // length in sectors
	StartSec  byte   // starting sector
	StartTrack byte  // starting track
}

// LoadTRD loads a .trd disk image.
func LoadTRD(path string) (*TRDFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading .trd file: %w", err)
	}

	trd := &TRDFile{Path: path}

	// Handle undersized images (40-track single-sided, etc.)
	n := len(data)
	if n > TRDDiskSize {
		n = TRDDiskSize
	}
	copy(trd.Data[:n], data[:n])

	return trd, nil
}

// ReadSector reads a 256-byte sector from the disk image.
func (t *TRDFile) ReadSector(track, side, sector int) []byte {
	offset := ((track*TRDSidesPerDisk+side)*TRDSectorsPerTrack + sector) * TRDSectorSize
	if offset < 0 || offset+TRDSectorSize > len(t.Data) {
		empty := make([]byte, TRDSectorSize)
		return empty
	}
	result := make([]byte, TRDSectorSize)
	copy(result, t.Data[offset:offset+TRDSectorSize])
	return result
}

// WriteSector writes a 256-byte sector to the disk image.
func (t *TRDFile) WriteSector(track, side, sector int, data []byte) {
	offset := ((track*TRDSidesPerDisk+side)*TRDSectorsPerTrack + sector) * TRDSectorSize
	if offset < 0 || offset+TRDSectorSize > len(t.Data) {
		return
	}
	n := TRDSectorSize
	if len(data) < n {
		n = len(data)
	}
	copy(t.Data[offset:offset+n], data[:n])
}

// ListDirectory returns all directory entries.
func (t *TRDFile) ListDirectory() []TRDEntry {
	var entries []TRDEntry
	for i := 0; i < TRDMaxDirEntries; i++ {
		offset := i * TRDDirEntrySize
		if t.Data[offset] == 0 {
			break // end of directory
		}
		e := TRDEntry{
			Name:       string(t.Data[offset : offset+8]),
			Extension:  t.Data[offset+8],
			Start:      uint16(t.Data[offset+9]) | uint16(t.Data[offset+10])<<8,
			Length:     uint16(t.Data[offset+11]) | uint16(t.Data[offset+12])<<8,
			Sectors:    t.Data[offset+13],
			StartSec:   t.Data[offset+14],
			StartTrack: t.Data[offset+15],
		}
		entries = append(entries, e)
	}
	return entries
}

// FindFile finds a file by name and extension in the directory.
func (t *TRDFile) FindFile(name string, ext byte) *TRDEntry {
	entries := t.ListDirectory()
	// Pad name to 8 chars
	padded := name
	for len(padded) < 8 {
		padded += " "
	}
	for _, e := range entries {
		if e.Name == padded && e.Extension == ext {
			return &e
		}
	}
	return nil
}

// ReadFile reads the complete contents of a file from the disk.
func (t *TRDFile) ReadFile(entry *TRDEntry) []byte {
	data := make([]byte, 0, int(entry.Length))
	track := int(entry.StartTrack)
	sector := int(entry.StartSec)
	remaining := int(entry.Length)

	for remaining > 0 {
		side := 0
		if track >= TRDTracksPerSide {
			side = 1
			track -= TRDTracksPerSide
		}
		secData := t.ReadSector(track, side, sector)
		n := TRDSectorSize
		if n > remaining {
			n = remaining
		}
		data = append(data, secData[:n]...)
		remaining -= n

		sector++
		if sector >= TRDSectorsPerTrack {
			sector = 0
			track++
		}
	}
	return data
}

// ---- TR-DOS PC trap: full function dispatch ----

// TR-DOS ROM entry points that we trap.
const (
	// $3D13: Main entry point — execute function with code in C register
	TRDOSEntryPoint uint16 = 0x3D13
	// $3D00: Cold start / enter TR-DOS from BASIC
	TRDOSColdStart uint16 = 0x3D00

	// TR-DOS system variable addresses (in RAM)
	trdosVarFilename uint16 = 0x5CDD // 8 bytes: current filename
	trdosVarFileType uint16 = 0x5CE5 // 1 byte: file type
	trdosVarLoadAddr uint16 = 0x5CE6 // 2 bytes: load address (from descriptor)
	trdosVarFileLen  uint16 = 0x5CE8 // 2 bytes: file length (from descriptor)
	trdosVarSectors  uint16 = 0x5CEA // 1 byte: sectors count
	trdosVarStartSec uint16 = 0x5CEB // 1 byte: start sector
	trdosVarStartTrk uint16 = 0x5CEC // 1 byte: start track
	trdosVarCmpBytes uint16 = 0x5D06 // 1 byte: bytes to compare in search
	trdosVarError    uint16 = 0x5D0F // 1 byte: error code
	trdosVarLoadFlag uint16 = 0x5CF9 // 1 byte: $00=load, $FF=verify
	trdosVarBASFlag  uint16 = 0x5D10 // 1 byte: $00 for BASIC load
)

// TRDState holds TR-DOS emulation state for the trap handler.
type TRDState struct {
	disk    *TRDFile
	machine *spectrum.Machine
}

// InstallTRDTraps sets up PC traps implementing the TR-DOS function API.
// Supports the full $3D13 dispatch table: sector I/O, file ops, catalog, etc.
func InstallTRDTraps(m *spectrum.Machine, trd *TRDFile) {
	state := &TRDState{disk: trd, machine: m}

	// Trap the main TR-DOS entry — C register = function number
	m.SetPCTrap(TRDOSEntryPoint, func() {
		state.dispatch()
	})

	// Cold start — just return, prevent crash
	m.SetPCTrap(TRDOSColdStart, func() {
		emulateRET(m)
	})
}

// dispatch implements the TR-DOS $3D13 function table.
// C register holds the function number.
func (s *TRDState) dispatch() {
	m := s.machine
	cpu := m.CPU
	fn := byte(cpu.BC()) // C register = low byte of BC

	switch fn {
	case 0x00: // Interface initialization
		// Reset: nothing to do in emulation
		s.setBC(0) // success
		emulateRET(m)

	case 0x01: // Drive initialization
		// A = drive number (0-3); we only have drive 0
		s.setBC(0)
		emulateRET(m)

	case 0x02: // Seek track
		// A = track number; nothing to physically seek
		s.setBC(0)
		emulateRET(m)

	case 0x03: // Set sector number
		// A = sector number; store for later use
		a := byte(cpu.AF() >> 8)
		m.Memory.Write(0x5CFF, a, false) // system var: current sector
		emulateRET(m)

	case 0x04: // Set buffer address
		// HL = address; store for later use
		hl := cpu.HL()
		m.Memory.Write(0x5D00, byte(hl), false)
		m.Memory.Write(0x5D01, byte(hl>>8), false)
		emulateRET(m)

	case 0x05: // Read group of sectors
		s.readSectors()

	case 0x06: // Write group of sectors
		s.writeSectors()

	case 0x07: // Display catalog
		// A = stream (2=screen, 3=printer); we just return success
		s.readSystemSector()
		s.setBC(0)
		emulateRET(m)

	case 0x08: // Read file descriptor
		s.readFileDescriptor()

	case 0x09: // Write file descriptor
		s.writeFileDescriptor()

	case 0x0A: // Find file
		s.findFile()

	case 0x0B: // Save CODE file
		s.saveCodeFile()

	case 0x0C: // Save BASIC program
		// Not commonly needed in emulation; return success
		s.setBC(0)
		emulateRET(m)

	case 0x0E: // Load or verify file
		s.loadFile()

	case 0x12: // Delete file
		// Mark file as deleted (set first byte to $01)
		s.deleteFile()

	case 0x13: // Copy 16 bytes to descriptor area ($5CDD)
		s.copyToDescArea()

	case 0x14: // Copy 16 bytes from descriptor area
		s.copyFromDescArea()

	case 0x15: // Test track
		// D = track; just report no errors
		m.Memory.Write(trdosVarError, 0, false)
		m.Memory.Write(0x5CD6, 0, false) // bad sector count = 0
		s.setBC(0)
		emulateRET(m)

	case 0x16: // Select bottom side
		emulateRET(m)

	case 0x17: // Select top side
		emulateRET(m)

	case 0x18: // Read system sector (track 0, sector 8)
		s.readSystemSector()
		s.setBC(0)
		emulateRET(m)

	default:
		// Unknown function — just return with no error
		s.setBC(0)
		emulateRET(m)
	}
}

// readSectors: function $05 — read B sectors from track D, sector E to address HL.
func (s *TRDState) readSectors() {
	m := s.machine
	cpu := m.CPU
	hl := cpu.HL()
	bc := cpu.BC()
	de := cpu.DE()
	count := int(bc >> 8) // B register = sector count
	track := int(de >> 8) // D register = track
	sector := int(de & 0xFF) // E register = sector (1-based in TR-DOS)

	if count == 0 {
		// B=0 means verify sector address mark only
		s.setBC(0)
		emulateRET(m)
		return
	}

	dest := hl
	for i := 0; i < count; i++ {
		// TR-DOS uses logical tracks: 0-79 = side 0, 80-159 = side 1
		physTrack := track
		side := 0
		if physTrack >= TRDTracksPerSide {
			side = 1
			physTrack -= TRDTracksPerSide
		}

		// TR-DOS sectors are 1-based in the API; our image is 0-based
		secIdx := sector - 1
		if secIdx < 0 {
			secIdx = 0
		}

		data := s.disk.ReadSector(physTrack, side, secIdx)
		for j := 0; j < TRDSectorSize; j++ {
			addr := dest + uint16(j)
			if addr >= 0x4000 { // don't write to ROM area
				m.Memory.Write(addr, data[j], false)
			}
		}
		dest += TRDSectorSize

		// Advance to next sector
		sector++
		if sector > TRDSectorsPerTrack {
			sector = 1
			track++
		}
	}
	s.setBC(0) // success
	emulateRET(m)
}

// writeSectors: function $06 — write B sectors from HL to track D, sector E.
func (s *TRDState) writeSectors() {
	m := s.machine
	cpu := m.CPU
	hl := cpu.HL()
	bc := cpu.BC()
	de := cpu.DE()
	count := int(bc >> 8)
	track := int(de >> 8)
	sector := int(de & 0xFF)

	src := hl
	for i := 0; i < count; i++ {
		physTrack := track
		side := 0
		if physTrack >= TRDTracksPerSide {
			side = 1
			physTrack -= TRDTracksPerSide
		}
		secIdx := sector - 1
		if secIdx < 0 {
			secIdx = 0
		}

		data := make([]byte, TRDSectorSize)
		for j := 0; j < TRDSectorSize; j++ {
			data[j] = m.Memory.Read(src + uint16(j))
		}
		s.disk.WriteSector(physTrack, side, secIdx, data)
		src += TRDSectorSize

		sector++
		if sector > TRDSectorsPerTrack {
			sector = 1
			track++
		}
	}
	s.setBC(0)
	emulateRET(m)
}

// readFileDescriptor: function $08 — read 16-byte descriptor #A into $5CDD.
func (s *TRDState) readFileDescriptor() {
	m := s.machine
	cpu := m.CPU
	index := int(cpu.AF() >> 8) // A register
	if index >= TRDMaxDirEntries {
		s.setBC(1) // no files found
		emulateRET(m)
		return
	}
	offset := index * TRDDirEntrySize
	for i := 0; i < 15; i++ { // 15 bytes to descriptor area
		m.Memory.Write(trdosVarFilename+uint16(i), s.disk.Data[offset+i], false)
	}
	s.setBC(0)
	emulateRET(m)
}

// writeFileDescriptor: function $09 — write 15 bytes from $5CDD to descriptor #A.
func (s *TRDState) writeFileDescriptor() {
	m := s.machine
	cpu := m.CPU
	index := int(cpu.AF() >> 8)
	if index >= TRDMaxDirEntries {
		s.setBC(1)
		emulateRET(m)
		return
	}
	offset := index * TRDDirEntrySize
	for i := 0; i < 15; i++ {
		s.disk.Data[offset+i] = m.Memory.Read(trdosVarFilename + uint16(i))
	}
	s.setBC(0)
	emulateRET(m)
}

// findFile: function $0A — find file matching name/type at $5CDD.
// Returns descriptor index in C, or $FF if not found.
func (s *TRDState) findFile() {
	m := s.machine

	// Read search name from descriptor area
	var searchName [8]byte
	for i := 0; i < 8; i++ {
		searchName[i] = m.Memory.Read(trdosVarFilename + uint16(i))
	}
	searchType := m.Memory.Read(trdosVarFileType)
	cmpBytes := int(m.Memory.Read(trdosVarCmpBytes))
	if cmpBytes == 0 || cmpBytes > 9 {
		cmpBytes = 9 // default: full name + type match
	}

	for i := 0; i < TRDMaxDirEntries; i++ {
		offset := i * TRDDirEntrySize
		if s.disk.Data[offset] == 0 {
			break // end of directory
		}
		if s.disk.Data[offset] == 0x01 {
			continue // deleted file
		}

		match := true
		for j := 0; j < 8 && j < cmpBytes; j++ {
			if s.disk.Data[offset+j] != searchName[j] {
				match = false
				break
			}
		}
		if match && cmpBytes >= 9 {
			if s.disk.Data[offset+8] != searchType {
				match = false
			}
		}
		if match {
			// Found — return index in C (low byte of BC)
			m.CPU.SetBC(uint16(i)) // B=0 (no error), C=index
			emulateRET(m)
			return
		}
	}

	// Not found
	m.CPU.SetBC(0x00FF) // B=0, C=$FF
	emulateRET(m)
}

// loadFile: function $0E — load file using descriptor at $5CDD.
// A register controls address/length source.
func (s *TRDState) loadFile() {
	m := s.machine
	cpu := m.CPU
	aReg := byte(cpu.AF() >> 8)

	// Read file metadata from descriptor area
	startSec := m.Memory.Read(trdosVarStartSec)
	startTrk := m.Memory.Read(trdosVarStartTrk)
	catLen := uint16(m.Memory.Read(trdosVarFileLen)) | uint16(m.Memory.Read(trdosVarFileLen+1))<<8
	catAddr := uint16(m.Memory.Read(trdosVarLoadAddr)) | uint16(m.Memory.Read(trdosVarLoadAddr+1))<<8

	var dest uint16
	var length uint16

	switch aReg {
	case 0x00: // Both from catalog
		dest = catAddr
		length = catLen
	case 0x03: // Both from registers: HL=addr, DE=length
		dest = cpu.HL()
		length = cpu.DE()
	default: // HL=addr, length from catalog
		dest = cpu.HL()
		length = catLen
	}

	// Check load/verify flag
	isVerify := m.Memory.Read(trdosVarLoadFlag) == 0xFF
	if isVerify {
		s.setBC(0)
		emulateRET(m)
		return
	}

	// Read sectors and copy to RAM
	track := int(startTrk)
	sector := int(startSec)
	remaining := int(length)
	addr := dest

	for remaining > 0 {
		physTrack := track
		side := 0
		if physTrack >= TRDTracksPerSide {
			side = 1
			physTrack -= TRDTracksPerSide
		}

		data := s.disk.ReadSector(physTrack, side, sector)
		n := TRDSectorSize
		if n > remaining {
			n = remaining
		}
		for i := 0; i < n; i++ {
			a := addr + uint16(i)
			if a >= 0x4000 {
				m.Memory.Write(a, data[i], false)
			}
		}
		addr += uint16(n)
		remaining -= n

		sector++
		if sector >= TRDSectorsPerTrack {
			sector = 0
			track++
		}
	}

	s.setBC(0)
	emulateRET(m)
}

// saveCodeFile: function $0B — save HL bytes from DE address as CODE file.
func (s *TRDState) saveCodeFile() {
	m := s.machine
	cpu := m.CPU
	src := cpu.HL()
	length := cpu.DE()

	// Read system sector to find free space
	sysSec := s.disk.ReadSector(0, 0, 8)
	freeSec := int(sysSec[225])
	freeTrk := int(sysSec[226])
	fileCount := int(sysSec[228])

	if fileCount >= TRDMaxDirEntries {
		s.setBC(4) // directory full
		emulateRET(m)
		return
	}

	// Calculate sectors needed
	sectorsNeeded := (int(length) + TRDSectorSize - 1) / TRDSectorSize

	// Write data to disk
	track := freeTrk
	sector := freeSec
	remaining := int(length)
	addr := src

	for remaining > 0 {
		physTrack := track
		side := 0
		if physTrack >= TRDTracksPerSide {
			side = 1
			physTrack -= TRDTracksPerSide
		}

		data := make([]byte, TRDSectorSize)
		n := TRDSectorSize
		if n > remaining {
			n = remaining
		}
		for i := 0; i < n; i++ {
			data[i] = m.Memory.Read(addr + uint16(i))
		}
		s.disk.WriteSector(physTrack, side, sector, data)
		addr += uint16(n)
		remaining -= n

		sector++
		if sector >= TRDSectorsPerTrack {
			sector = 0
			track++
		}
	}

	// Write directory entry
	dirOffset := fileCount * TRDDirEntrySize
	for i := 0; i < 8; i++ {
		s.disk.Data[dirOffset+i] = m.Memory.Read(trdosVarFilename + uint16(i))
	}
	s.disk.Data[dirOffset+8] = m.Memory.Read(trdosVarFileType)
	s.disk.Data[dirOffset+9] = byte(src)
	s.disk.Data[dirOffset+10] = byte(src >> 8)
	s.disk.Data[dirOffset+11] = byte(length)
	s.disk.Data[dirOffset+12] = byte(length >> 8)
	s.disk.Data[dirOffset+13] = byte(sectorsNeeded)
	s.disk.Data[dirOffset+14] = byte(freeSec)
	s.disk.Data[dirOffset+15] = byte(freeTrk)

	// Update system sector
	newFreeSec := sector
	newFreeTrk := track
	sysSec[225] = byte(newFreeSec)
	sysSec[226] = byte(newFreeTrk)
	sysSec[228] = byte(fileCount + 1)
	freeCount := uint16(sysSec[229]) | uint16(sysSec[230])<<8
	freeCount -= uint16(sectorsNeeded)
	sysSec[229] = byte(freeCount)
	sysSec[230] = byte(freeCount >> 8)
	s.disk.WriteSector(0, 0, 8, sysSec)

	s.setBC(0)
	emulateRET(m)
}

// deleteFile: function $12 — mark matching file as deleted.
func (s *TRDState) deleteFile() {
	m := s.machine

	var searchName [8]byte
	for i := 0; i < 8; i++ {
		searchName[i] = m.Memory.Read(trdosVarFilename + uint16(i))
	}
	searchType := m.Memory.Read(trdosVarFileType)

	for i := 0; i < TRDMaxDirEntries; i++ {
		offset := i * TRDDirEntrySize
		if s.disk.Data[offset] == 0 {
			break
		}
		if s.disk.Data[offset] == 0x01 {
			continue
		}

		match := true
		for j := 0; j < 8; j++ {
			if s.disk.Data[offset+j] != searchName[j] {
				match = false
				break
			}
		}
		if match && s.disk.Data[offset+8] != searchType {
			match = false
		}
		if match {
			// Save original first char at $5D08, mark as deleted
			m.Memory.Write(0x5D08, s.disk.Data[offset], false)
			s.disk.Data[offset] = 0x01
		}
	}
	s.setBC(0)
	emulateRET(m)
}

// copyToDescArea: function $13 — copy 16 bytes from HL to $5CDD.
func (s *TRDState) copyToDescArea() {
	m := s.machine
	hl := m.CPU.HL()
	for i := 0; i < 16; i++ {
		b := m.Memory.Read(hl + uint16(i))
		m.Memory.Write(trdosVarFilename+uint16(i), b, false)
	}
	emulateRET(m)
}

// copyFromDescArea: function $14 — copy 16 bytes from $5CDD to HL.
func (s *TRDState) copyFromDescArea() {
	m := s.machine
	hl := m.CPU.HL()
	for i := 0; i < 16; i++ {
		b := m.Memory.Read(trdosVarFilename + uint16(i))
		m.Memory.Write(hl+uint16(i), b, false)
	}
	emulateRET(m)
}

// readSystemSector reads track 0, sector 8 (disk info) into system variables.
func (s *TRDState) readSystemSector() {
	// The real TR-DOS reads sector 9 (1-based) = sector 8 (0-based)
	// and updates several system variables. We just ensure the
	// descriptor area is usable.
}

// setBC sets the BC register pair (B=high=error, C=low=result).
func (s *TRDState) setBC(v uint16) {
	s.machine.CPU.SetBC(v)
}

// LoadTRDFile loads a specific file from the .trd image into machine RAM.
// For BASIC files (ext 'B'/'b'), loads into the program area and executes RUN.
// For CODE files, loads at the specified address (or entry's Start address if destAddr=0).
func LoadTRDFile(m *spectrum.Machine, trd *TRDFile, name string, ext byte, destAddr uint16) error {
	entry := trd.FindFile(name, ext)
	if entry == nil {
		return fmt.Errorf("file not found on disk: %s.%c", name, ext)
	}

	data := trd.ReadFile(entry)

	if ext == 'B' || ext == 'b' {
		// BASIC program: load into program area and RUN
		WaitROMInit(m, 100)
		LoadBASICProgram(m, data)
		ExecBASIC(m, TokenizeRUN())
		fmt.Printf("Loaded BASIC program '%s' (%d bytes, autostart line %d)\n",
			name, len(data), entry.Start)
		return nil
	}

	// CODE file: load at destAddr (or entry.Start if 0)
	if destAddr == 0 {
		destAddr = entry.Start
	}
	for i, b := range data {
		addr := destAddr + uint16(i)
		if addr >= 0x4000 {
			m.Memory.Write(addr, b, false)
		}
	}
	fmt.Printf("Loaded CODE '%s' (%d bytes at $%04X)\n", name, len(data), destAddr)
	return nil
}

// AutoBootTRD simulates TR-DOS autoboot: finds "boot" file on disk,
// loads it as a BASIC program, sets up system variables, and executes RUN.
// This is what the TR-DOS ROM does on power-up when a disk is inserted.
func AutoBootTRD(m *spectrum.Machine, trd *TRDFile) error {
	WaitROMInit(m, 100)

	// Find "boot" file (type B = BASIC), then fall back to first BASIC file
	boot := trd.FindFile("boot", 'B')
	if boot == nil {
		boot = trd.FindFile("boot", 'b')
	}
	if boot == nil {
		// Try first BASIC file on disk as fallback
		entries := trd.ListDirectory()
		for i := range entries {
			if entries[i].Extension == 'B' || entries[i].Extension == 'b' {
				boot = &entries[i]
				break
			}
		}
		if boot == nil {
			if len(entries) > 0 {
				fmt.Printf("No BASIC file found. Disk directory:\n")
				for i, e := range entries {
					fmt.Printf("  [%d] %-8s.%c  start=$%04X len=%d (%d sectors)\n",
						i, e.Name, e.Extension, e.Start, e.Length, e.Sectors)
				}
			}
			return fmt.Errorf("no bootable file on disk (use --trd-load name:ext:addr)")
		}
	}

	// Load the BASIC program into the program area
	LoadBASICProgram(m, trd.ReadFile(boot))

	// Execute RUN via the unified automation system
	ExecBASIC(m, TokenizeRUN())

	fmt.Printf("Autoboot: loaded '%s' (%d bytes)\n", boot.Name, len(trd.ReadFile(boot)))
	return nil
}

// LoadBASICProgram loads raw BASIC program bytes into the Spectrum's
// program area and updates system variables accordingly.
func LoadBASICProgram(m *spectrum.Machine, data []byte) {
	prog := readWord(m, 0x5C53)
	if prog < 0x5CCB || prog > 0x8000 {
		prog = 0x5CCB
	}

	for i, b := range data {
		addr := prog + uint16(i)
		if addr >= 0x4000 {
			m.Memory.Write(addr, b, false)
		}
	}

	// Update system variables
	endProg := prog + uint16(len(data))

	// VARS — end of BASIC program (start of variables area)
	writeWord(m, 0x5C4B, endProg)

	// End-of-variables marker
	m.Memory.Write(endProg, 0x80, false)

	// E_LINE — editing line (after variables + marker)
	eLine := endProg + 1
	writeWord(m, 0x5C59, eLine)
}

func readWord(m *spectrum.Machine, addr uint16) uint16 {
	return uint16(m.Memory.Read(addr)) | uint16(m.Memory.Read(addr+1))<<8
}

func writeWord(m *spectrum.Machine, addr, val uint16) {
	m.Memory.Write(addr, byte(val), false)
	m.Memory.Write(addr+1, byte(val>>8), false)
}

