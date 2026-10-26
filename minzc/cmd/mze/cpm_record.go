package main

import "io"

// CP/M sequential I/O follows the FCB cursor, independently of the host file's
// seek position. Only a complete physical record counts as a successful write.
func cpmWriteRecord(f io.WriterAt, buf []byte, record int64) error {
	n, err := f.WriteAt(buf, record*128)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return io.ErrShortWrite
	}
	return nil
}
