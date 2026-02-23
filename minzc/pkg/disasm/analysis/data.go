package analysis

// DetectStrings scans undefined regions for runs of printable ASCII.
// minLen is the minimum run length to consider a string (default: 4).
func (a *Analysis) DetectStrings(minLen int) {
	if minLen < 1 {
		minLen = 4
	}

	endAddr := int(a.Origin) + len(a.Data)
	addr := int(a.Origin)

	for addr < endAddr && addr <= 0xFFFF {
		if a.ByteMap[uint16(addr)] != ByteUndefined {
			addr++
			continue
		}

		// Scan for printable ASCII run
		runStart := addr
		runLen := 0
		for addr < endAddr && addr <= 0xFFFF {
			b, ok := a.ReadByte(uint16(addr))
			if !ok {
				break
			}
			if isPrintableASCII(b) {
				runLen++
				addr++
			} else {
				break
			}
		}

		if runLen < minLen {
			if runLen > 0 {
				addr = runStart + 1
			} else {
				addr++
			}
			continue
		}

		// Check for terminator
		totalLen := runLen
		terminator := byte(0)
		if addr < endAddr && addr <= 0xFFFF {
			b, ok := a.ReadByte(uint16(addr))
			if ok {
				switch {
				case b == 0x00: // NUL terminator
					terminator = 0x00
					totalLen++
				case b == 0x0D: // CR terminator
					terminator = 0x0D
					totalLen++
				case b == 0x24: // $ terminator (CP/M)
					terminator = 0x24
					totalLen++
				case b&0x80 != 0 && isPrintableASCII(b&0x7F):
					// Bit-7 set on last char (Spectrum convention)
					terminator = 0x80
					totalLen++
				}
			}
		}

		// Extract string content
		content := make([]byte, runLen)
		for i := 0; i < runLen; i++ {
			b, _ := a.ReadByte(uint16(runStart + i))
			content[i] = b
		}

		// If bit-7 terminator, include the last char (with bit 7 cleared) in content
		if terminator == 0x80 {
			b, _ := a.ReadByte(uint16(runStart + runLen))
			content = append(content, b&0x7F)
		}

		// Mark bytes as string
		for i := 0; i < totalLen; i++ {
			a.ByteMap[uint16(runStart+i)] = ByteString
		}

		a.Strings[uint16(runStart)] = &DetectedString{
			Addr:       uint16(runStart),
			Content:    string(content),
			Terminator: terminator,
			Length:     totalLen,
		}

		addr = runStart + totalLen
	}
}

// DetectDataBlocks classifies remaining undefined bytes that are
// referenced by data xrefs as ByteData.
func (a *Analysis) DetectDataBlocks() {
	for addr, refs := range a.XRefsTo {
		if a.ByteMap[addr] != ByteUndefined {
			continue
		}
		hasDataRef := false
		for _, ref := range refs {
			if ref.Type == XRefRead || ref.Type == XRefWrite {
				hasDataRef = true
				break
			}
		}
		if hasDataRef {
			a.ByteMap[addr] = ByteData
		}
	}
}

// isPrintableASCII returns true for bytes in the range 0x20-0x7E
// plus common control characters that appear in strings.
func isPrintableASCII(b byte) bool {
	return (b >= 0x20 && b <= 0x7E) || b == 0x0A || b == 0x0D || b == 0x09
}
