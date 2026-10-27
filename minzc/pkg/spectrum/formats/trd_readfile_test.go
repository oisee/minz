package formats

import (
	"bytes"
	"strconv"
	"testing"
)

func TestReadFileAcrossLogicalTracksAndSides(t *testing.T) {
	for _, start := range []int{79*16 + 15, 80*16 + 15, 159*16 + 13} {
		t.Run(strconv.Itoa(start), func(t *testing.T) {
			d := &TRDFile{}
			want := make([]byte, 0, 3*256)
			for i := 0; i < 3; i++ {
				pos := start + i
				block := bytes.Repeat([]byte{byte(i + 1)}, 256)
				d.WriteSector((pos/16)%80, (pos/16)/80, pos%16, block)
				want = append(want, block...)
			}
			entry := &TRDEntry{Extension: 'E', StartTrack: byte(start / 16), StartSec: byte(start % 16), Sectors: 3, Length: 3*256 - 17}
			if got := d.ReadFile(entry); !bytes.Equal(got, want[:len(want)-17]) {
				t.Fatalf("logical sector %d: contents differ", start)
			}
		})
	}
}
