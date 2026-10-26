package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
)

type recordWriter struct {
	n      int
	err    error
	offset int64
}

func (w *recordWriter) WriteAt(_ []byte, offset int64) (int, error) {
	w.offset = offset
	return w.n, w.err
}

func TestCPMRecordShortWrite(t *testing.T) {
	failed := errors.New("disk full")
	for _, tc := range []struct {
		n         int
		err, want error
	}{
		{128, nil, nil}, {127, nil, io.ErrShortWrite}, {0, failed, failed}, {64, failed, failed},
	} {
		w := &recordWriter{n: tc.n, err: tc.err}
		if err := cpmWriteRecord(w, make([]byte, 128), 129); !errors.Is(err, tc.want) {
			t.Fatalf("n=%d: %v want %v", tc.n, err, tc.want)
		}
		if w.offset != 129*128 {
			t.Fatal("wrong physical record offset", w.offset)
		}
	}
}

// Actual CALL 5 instructions exercise create/write/close, extent rollover,
// repositioning through the FCB, and propagation of an OS write failure.
func TestCPMNativeSequentialWrite(t *testing.T) {
	dir := t.TempDir()
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"mze", filepath.Join(dir, "probe.com")}
	z := emulator.NewRemogattoZ80WithScreen()
	setupCPMBDOS(z)
	m := z.Memory()
	const fcb = uint16(0x3000)
	const dma = uint16(0x3100)
	copy(m[fcb+1:], "NOTES   BIN")
	// Create must reset every sequential cursor component, not just CR.
	m[fcb+12], m[fcb+14], m[fcb+32] = 31, 63, 127
	copy(m[dma:], bytes.Repeat([]byte{0xA7}, 128))
	var code []byte
	call := func(fn byte, de, result uint16) {
		code = append(code, 0x0e, fn, 0x11, byte(de), byte(de>>8), 0xcd, 5, 0, 0x32, byte(result), byte(result>>8))
	}
	store := func(addr uint16, value byte) { code = append(code, 0x3e, value, 0x32, byte(addr), byte(addr>>8)) }
	copyByte := func(src, dst uint16) {
		code = append(code, 0x3a, byte(src), byte(src>>8), 0x32, byte(dst), byte(dst>>8))
	}
	call(26, dma, 0x2000)
	call(22, fcb, 0x2001)
	for i := 0; i < 129; i++ {
		call(21, fcb, 0x2100+uint16(i))
	}
	copyByte(fcb+12, 0x2002)
	copyByte(fcb+32, 0x2003)
	// Rewind the FCB: record zero must be replaced rather than appended.
	store(fcb+12, 0)
	store(fcb+32, 0)
	store(dma, 0x42)
	call(21, fcb, 0x2004)
	store(fcb+14, 64)
	call(21, fcb, 0x2005)
	copyByte(fcb+14, 0x2006)
	call(16, fcb, 0x2007)
	call(21, fcb, 0x2008)
	call(15, fcb, 0x2009) // Existing read-only host handle: failure must not be success.
	call(21, fcb, 0x200a)
	copyByte(fcb+32, 0x200b)
	call(16, fcb, 0x200c)
	code = append(code, 0xf3, 0x76)
	z.LoadAt(0x100, code)
	z.SetPC(0x100)
	z.SetSP(0xfe00)
	setExecutionLimit(z.RemogattoZ80, 100000)
	if err := z.Execute(); err != nil {
		t.Fatal(err)
	}
	for i, want := range []byte{0, 0, 1, 1, 0, 2, 64, 0, 255, 0, 2, 0, 0} {
		if m[0x2000+i] != want {
			t.Fatalf("call/state %d=%d want %d", i, m[0x2000+i], want)
		}
	}
	for i := 0; i < 129; i++ {
		if m[0x2100+i] != 0 {
			t.Fatalf("record %d write failed: %d", i, m[0x2100+i])
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "NOTES.BIN"))
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.Repeat([]byte{0xA7}, 129*128)
	want[0] = 0x42
	if !bytes.Equal(got, want) {
		t.Fatalf("wrong record contents or file length: %d", len(got))
	}
}
