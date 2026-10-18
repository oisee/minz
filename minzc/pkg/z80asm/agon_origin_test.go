package z80asm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func agonAddress24(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }

// This checks ADL encodings and their physical placement, not MOS execution.
func TestAgonHeaderAddressPlacement(t *testing.T) {
	for _, origin := range []string{"", "ORG $040045\n"} {
		t.Run(strings.TrimSpace(origin), func(t *testing.T) {
			a := NewAssembler()
			if err := a.SetTarget(TargetAgonLight2); err != nil {
				t.Fatal(err)
			}
			r, err := a.AssembleString(origin + `entry:
    CALL worker
    LD HL, payload
    JP finish
worker:
    LD A, (payload)
    RET
finish:
    RET
payload:
    DB $A5, $5A
`)
			if err != nil || len(r.Errors) != 0 {
				t.Fatal(err, r)
			}
			image, err := GetTargetConfig(TargetAgonLight2).OutputFormat.Generator(r)
			if err != nil {
				t.Fatal(err)
			}
			if r.Origin != 0x040045 || r.Symbols["entry"] != 0x040045 {
				t.Errorf("code origin=%06x entry=%06x, want 040045", r.Origin, r.Symbols["entry"])
			}
			if len(image) != 69+len(r.Binary) || !bytes.Equal(image[69:], r.Binary) {
				t.Fatal("MOS packaging changed code bytes or added a second relocation")
			}
			if got := agonAddress24(image[1:4]); got != r.Symbols["entry"] {
				t.Errorf("header startup JP=%06x, entry=%06x", got, r.Symbols["entry"])
			}
			for _, ref := range []struct {
				offset int
				opcode byte
				symbol string
			}{
				{69, 0xCD, "worker"},
				{73, 0x21, "payload"},
				{77, 0xC3, "finish"},
				{81, 0x3A, "payload"},
			} {
				if image[ref.offset] != ref.opcode || agonAddress24(image[ref.offset+1:]) != r.Symbols[ref.symbol] {
					t.Errorf("reference at file offset %d does not encode %s", ref.offset, ref.symbol)
				}
			}
			for _, line := range r.Listing {
				offset := line.Address - 0x040000
				if offset < 69 || offset+len(line.Bytes) > len(image) || !bytes.Equal(image[offset:offset+len(line.Bytes)], line.Bytes) {
					t.Errorf("assembled address %06x does not map to its physical image bytes", line.Address)
				}
			}
			if p := r.Symbols["payload"] - 0x040000; p < 69 || p+2 > len(image) || !bytes.Equal(image[p:p+2], []byte{0xA5, 0x5A}) {
				t.Fatal("data pointer does not address physical payload")
			}
		})
	}
}

func TestAgonHeaderIncludedDataCoverage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blob.bin")
	if err := os.WriteFile(path, []byte{0x11, 0xA5, 0x5A, 0x22}, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"middle", "RET\nINCBIN %q, 1, 2\nRET\n", false},
		{"tail", "RET\nINCBIN %q, 1, 2\n", false},
		{"only", "INCBIN %q, 1, 2\n", false},
		{"sparse tail", "RET\nORG $040080\nINCBIN %q, 1, 2\n", true},
		{"overlapping tail", "RET\nORG $040045\nINCBIN %q, 1, 2\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAssembler()
			if err := a.SetTarget(TargetAgonLight2); err != nil {
				t.Fatal(err)
			}
			r, err := a.AssembleString("ORG $040045\n" + fmt.Sprintf(tc.body, path))
			if err != nil || len(r.Errors) > 0 {
				t.Fatal(err, r)
			}
			image, err := generateAgonBin(r)
			if tc.wantError {
				if err == nil {
					t.Fatal("accepted noncontiguous INCBIN payload")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(image[69:], r.Binary) || !bytes.Contains(image[69:], []byte{0xA5, 0x5A}) {
				t.Fatal("included data changed during packaging")
			}
			// Removing any emitting listing entry must invalidate packaging,
			// including the last entry, which has no successor to reveal a gap.
			if len(r.Listing) == 0 {
				t.Fatal("included bytes lack address metadata")
			}
			r.Listing = r.Listing[:len(r.Listing)-1]
			if _, err := generateAgonBin(r); err == nil {
				t.Fatal("accepted incomplete binary address coverage")
			}
		})
	}
}

func TestAgonHeaderSourceEntryBounds(t *testing.T) {
	for _, tc := range []struct {
		end       string
		wantError bool
	}{
		{"END 0", true},
		{"END zero", true},
		{"END $040046", true},
		{"END start", false},
		{"", false},
	} {
		t.Run(tc.end, func(t *testing.T) {
			a := NewAssembler()
			if err := a.SetTarget(TargetAgonLight2); err != nil {
				t.Fatal(err)
			}
			r, err := a.AssembleString("zero EQU 0\nORG $040045\nstart:\nRET\n" + tc.end + "\n")
			if err != nil || len(r.Errors) > 0 {
				t.Fatal(err, r)
			}
			_, err = generateAgonBin(r)
			if (err != nil) != tc.wantError {
				t.Fatalf("entry=%06x, error=%v, wantError=%v", r.EntryPoint, err, tc.wantError)
			}
		})
	}
}

func TestAgonRawOutputOrigin(t *testing.T) {
	for _, tc := range []struct {
		org  string
		base int
	}{
		{"", 0x040045},
		{"ORG $040000\n", 0x040000},
		{"ORG $030000\n", 0x030000},
	} {
		t.Run(tc.org, func(t *testing.T) {
			a := NewAssembler()
			if err := a.SetTarget(TargetAgonLight2); err != nil {
				t.Fatal(err)
			}
			r, err := a.AssembleString(tc.org + "start:\nLD HL, payload\nRET\npayload:\nDB $A5\n")
			if err != nil || len(r.Errors) > 0 {
				t.Fatal(err, r)
			}
			raw, err := LookupOutputFormat("code").Generator(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(raw, r.Binary) || len(raw) != 6 || raw[0] != 0x21 || raw[5] != 0xA5 || agonAddress24(raw[1:4]) != tc.base+5 || r.Symbols["start"] != tc.base {
				t.Fatalf("raw output changed explicit/default addresses or added a header: %x, start=%06x", raw, r.Symbols["start"])
			}
		})
	}
}

func TestAgonHeaderRejectsIncompatibleOrigins(t *testing.T) {
	for _, src := range []string{
		"ORG $040000\nRET\n", // old default overlaps the MOS header
		"ORG $040046\nRET\n", // unsupported custom origin
		"ORG $030000\nRET\n",
		"ORG $040045\nNOP\nORG $040080\nRET\n", // sparse output is not a contiguous image
		"ORG $040045\nNOP\nORG $040045\nRET\n", // overlapping sections
	} {
		t.Run(strings.ReplaceAll(src, "\n", ";"), func(t *testing.T) {
			a := NewAssembler()
			if err := a.SetTarget(TargetAgonLight2); err != nil {
				t.Fatal(err)
			}
			r, err := a.AssembleString(src)
			if err != nil || len(r.Errors) != 0 {
				t.Fatal(err, r)
			}
			if _, err := generateAgonBin(r); err == nil {
				t.Fatal("packaged an image with incompatible assembled addresses")
			}
		})
	}
	for _, origin := range []int{0, 0x040000, 0x040046} {
		if _, err := generateAgonBin(&Result{Origin: origin, Binary: []byte{0xC9}}); err == nil {
			t.Errorf("packaged incompatible Result.Origin=%06x", origin)
		}
	}
}

func TestAgonHeaderEntryPoint(t *testing.T) {
	a := NewAssembler()
	if err := a.SetTarget(TargetAgonLight2); err != nil {
		t.Fatal(err)
	}
	r, err := a.AssembleString("ORG $040045\nDB 0\nstart:\nRET\nEND start\n")
	if err != nil || len(r.Errors) != 0 {
		t.Fatal(err, r)
	}
	image, err := generateAgonBin(r)
	if err != nil {
		t.Fatal(err)
	}
	if target := agonAddress24(image[1:4]); target != r.Symbols["start"] || image[target-0x040000] != 0xC9 {
		t.Fatalf("header transfer does not reach explicit entry: %x", image[:4])
	}
	for _, entry := range []int{0x040000, 0x040047, 0xFFFFFF} {
		r.EntryPoint = entry
		if _, err := generateAgonBin(r); err == nil {
			t.Errorf("packaged entry %06x outside code image", entry)
		}
	}
}
