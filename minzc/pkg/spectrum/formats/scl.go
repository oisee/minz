package formats

import (
	"fmt"
	"os"
)

// SCL format: "SINCLAIR" signature + file count + N×14-byte entries + file data.
// Each entry: 8-byte name + 1-byte ext + 2-byte start + 2-byte length + 1-byte sectors.
// Converted to TRD in memory for use with the existing TR-DOS trap system.

const (
	sclSignature  = "SINCLAIR"
	sclHeaderSize = 9  // 8 bytes signature + 1 byte file count
	sclEntrySize  = 14 // 8+1+2+2+1
)

// LoadSCL loads a .scl disk image and converts it to a TRDFile in memory.
func LoadSCL(path string) (*TRDFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading .scl file: %w", err)
	}

	if len(data) < sclHeaderSize {
		return nil, fmt.Errorf("scl file too small")
	}

	// Verify signature
	if string(data[:8]) != sclSignature {
		return nil, fmt.Errorf("invalid SCL signature: %q", string(data[:8]))
	}

	fileCount := int(data[8])
	dirEnd := sclHeaderSize + fileCount*sclEntrySize
	if dirEnd > len(data) {
		return nil, fmt.Errorf("scl directory extends past end of file (count=%d)", fileCount)
	}

	// Build a TRD image in memory
	trd := &TRDFile{Path: path}

	// Data starts after track 0 (directory + system sector).
	// Track 0 has 16 sectors × 256 bytes = 4096 bytes.
	// Directory: sectors 0-7 (entries), sector 8 (system), sectors 9-15 (free).
	// File data starts at track 0 sector 9, but conventionally at track 1 sector 0.
	// We'll place files starting at track 1, sector 0.
	dataTrack := 1
	dataSector := 0

	dataOffset := dirEnd // offset into SCL data area

	for i := 0; i < fileCount; i++ {
		entryOff := sclHeaderSize + i*sclEntrySize
		name := data[entryOff : entryOff+8]
		ext := data[entryOff+8]
		start := uint16(data[entryOff+9]) | uint16(data[entryOff+10])<<8
		length := uint16(data[entryOff+11]) | uint16(data[entryOff+12])<<8
		sectors := int(data[entryOff+13])

		// Write directory entry to TRD track 0
		dirOff := i * TRDDirEntrySize
		copy(trd.Data[dirOff:dirOff+8], name)
		trd.Data[dirOff+8] = ext
		trd.Data[dirOff+9] = byte(start)
		trd.Data[dirOff+10] = byte(start >> 8)
		trd.Data[dirOff+11] = byte(length)
		trd.Data[dirOff+12] = byte(length >> 8)
		trd.Data[dirOff+13] = byte(sectors)
		trd.Data[dirOff+14] = byte(dataSector)
		trd.Data[dirOff+15] = byte(dataTrack)

		// Copy file data sectors into TRD image using WriteSector
		// (handles the interleaved side layout correctly)
		for s := 0; s < sectors; s++ {
			physTrack := dataTrack
			side := 0
			if physTrack >= TRDTracksPerSide {
				side = 1
				physTrack -= TRDTracksPerSide
			}

			secData := make([]byte, TRDSectorSize)
			end := dataOffset + TRDSectorSize
			if end > len(data) {
				end = len(data)
			}
			if dataOffset < len(data) {
				copy(secData, data[dataOffset:end])
			}
			trd.WriteSector(physTrack, side, dataSector, secData)

			dataOffset += TRDSectorSize
			dataSector++
			if dataSector >= TRDSectorsPerTrack {
				dataSector = 0
				dataTrack++
			}
		}
	}

	// Write system sector (track 0, sector 8)
	sysOff := TRDSysSectorOffset
	trd.Data[sysOff+0xE1] = byte(dataSector)         // first free sector
	trd.Data[sysOff+0xE2] = byte(dataTrack)           // first free track
	trd.Data[sysOff+0xE3] = 0x16                      // disk type: 80 tracks, 2 sides
	trd.Data[sysOff+0xE4] = byte(fileCount)            // file count
	freeSectors := TRDSectorsPerTrack*TRDTracksPerSide*TRDSidesPerDisk - (dataTrack*TRDSectorsPerTrack + dataSector)
	trd.Data[sysOff+0xE5] = byte(freeSectors)
	trd.Data[sysOff+0xE6] = byte(freeSectors >> 8)
	trd.Data[sysOff+0xE7] = 0x10 // TR-DOS ID byte

	return trd, nil
}
