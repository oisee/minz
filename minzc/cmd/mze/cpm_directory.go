package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Host names are admitted before ASCII case normalization. Unicode case folding
// must never invent a CP/M spelling that cannot reopen the same host identity.
func cpmName83(name string) ([11]byte, bool) {
	out := [11]byte{}
	for i := range out {
		out[i] = ' '
	}
	for _, b := range []byte(name) {
		if b < 33 || b > 126 {
			return out, false
		}
	}
	parts := strings.Split(name, ".")
	if len(parts) > 2 || len(parts[0]) < 1 || len(parts[0]) > 8 {
		return out, false
	}
	for p, s := range parts {
		if p == 1 && (len(s) < 1 || len(s) > 3) {
			return out, false
		}
		offset := 0
		if p == 1 {
			offset = 8
		}
		for i := range s {
			b := s[i]
			if strings.ContainsRune(`<>.,;:=?*[]/\`, rune(b)) {
				return out, false
			}
			if b >= 'a' && b <= 'z' {
				b -= 32
			}
			out[offset+i] = b
		}
	}
	return out, true
}

type cpmHostEntry struct {
	name [11]byte
	path string
}

const cpmHostDirectoryLimit = 4096

func cpmDirectoryEntries(dir string, pattern [11]byte) ([]cpmHostEntry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var matches []cpmHostEntry
	seen := map[[11]byte]bool{}
	for {
		entries, err := f.ReadDir(64)
		if err != nil && err != io.EOF {
			return nil, err
		}
		for _, e := range entries {
			if !e.Type().IsRegular() {
				continue
			}
			name, ok := cpmName83(e.Name())
			if !ok {
				continue
			}
			match := true
			for i, b := range pattern {
				b &= 127
				if b >= 'a' && b <= 'z' {
					b -= 32
				}
				if b != '?' && b != name[i] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			if seen[name] {
				return nil, fmt.Errorf("ambiguous CP/M canonical filename %q", string(name[:]))
			}
			seen[name] = true
			if len(matches) == cpmHostDirectoryLimit {
				return nil, fmt.Errorf("CP/M directory exceeds admitted %d matching-file limit", cpmHostDirectoryLimit)
			}
			matches = append(matches, cpmHostEntry{name: name, path: filepath.Join(dir, e.Name())})
		}
		if err == io.EOF {
			break
		}
	}
	sort.Slice(matches, func(i, j int) bool { return bytes.Compare(matches[i].name[:], matches[j].name[:]) < 0 })
	return matches, nil
}
func cpmDirectory(dir string, pattern [11]byte) ([][11]byte, error) {
	entries, err := cpmDirectoryEntries(dir, pattern)
	if err != nil {
		return nil, err
	}
	names := make([][11]byte, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names, nil
}
func cpmOpen83(dir string, name [11]byte) (*os.File, error) {
	entries, err := cpmDirectoryEntries(dir, name)
	if err != nil {
		return nil, err
	}
	if len(entries) != 1 {
		return nil, os.ErrNotExist
	}
	// Reject a replacement symlink or a different file raced into this spelling.
	before, err := os.Lstat(entries[0].path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("CP/M open requires regular file")
	}
	f, err := os.Open(entries[0].path)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		f.Close()
		return nil, fmt.Errorf("CP/M file identity changed while opening")
	}
	return f, nil
}

// FCB names are padded ASCII components, not host paths. Attribute bits are
// masked; padding must remain padding. Search admits '?' in padded positions.
func cpmFCBName(read func(uint16) byte, addr uint16, currentDisk byte, pattern bool) ([11]byte, error) {
	out := [11]byte{}
	if currentDisk != 0 {
		return out, fmt.Errorf("unsupported CP/M current drive %d", currentDisk)
	}
	drive := read(addr)
	if drive != 0 && drive != currentDisk+1 {
		return out, fmt.Errorf("unsupported CP/M drive %d", drive)
	}
	for part := 0; part < 2; part++ {
		start, end := 0, 8
		if part == 1 {
			start, end = 8, 11
		}
		padding := false
		for i := start; i < end; i++ {
			b := read(addr+1+uint16(i)) & 127
			if b == ' ' {
				padding = true
			} else {
				if padding || b < 33 || b > 126 || strings.ContainsRune(`<>.,;:=*[]/\`, rune(b)) || b == '?' && !pattern {
					return out, fmt.Errorf("invalid CP/M FCB name/padding")
				}
				if b >= 'a' && b <= 'z' {
					b -= 32
				}
			}
			out[i] = b
		}
	}
	if out[0] == ' ' {
		return out, fmt.Errorf("empty CP/M FCB basename")
	}
	return out, nil
}

func cpmSequentialRecord(read func(uint16) byte, addr uint16) int64 {
	return (int64(read(addr+14)&63)*32+int64(read(addr+12)&31))*128 + int64(read(addr+32)&127)
}
func cpmAdvanceRecord(write func(uint16, byte), addr uint16, record int64) {
	write(addr+32, byte(record%128))
	write(addr+12, byte((record/128)%32))
	// Preserve the exhausted extent sentinel instead of wrapping to record zero.
	write(addr+14, byte(record/4096))
}
