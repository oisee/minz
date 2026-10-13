package vir

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const enrtStride = 4096
const maxENRTSize = int64(600 << 20)

type ENRTCheckpoint struct {
	First, Count   int
	Offset, Length int64
	SHA256         string
}
type ENRTIndex struct {
	Schema, SourceSHA256, HeaderSHA256 string
	SourceSize                         int64
	MaxVregs, Records, Stride          int
	Blocks                             []ENRTCheckpoint
}

// ENRTOperation holds a private validated index copy and one opened source file.
// Imported tables are provisional until Validate succeeds at operation end.
// Checkpoint metadata itself is untrusted proposal provenance, not a certificate.
type ENRTOperation struct {
	path   string
	file   *os.File
	index  ENRTIndex
	closed bool
}

func enrtHeader(h []byte) (int, int, error) {
	if len(h) != 16 || string(h[:4]) != "ENRT" || binary.LittleEndian.Uint32(h[4:]) != 1 || h[13] != 12 || h[14] != 0 || h[15] != 0 {
		return 0, 0, fmt.Errorf("requires canonical ENRT v1/12 metrics")
	}
	max := int(h[12])
	if max != 4 && max != 5 {
		return 0, 0, fmt.Errorf("checkpoint supports full 4v/5v only")
	}
	count := 0
	for n := 2; n <= max; n++ {
		count += countShapesWithLocSets(n, 4, 3)
	}
	if uint32(count) != binary.LittleEndian.Uint32(h[8:]) {
		return 0, 0, fmt.Errorf("noncanonical ENRT enumeration")
	}
	return max, count, nil
}
func expectedENRTVreg(i, max int) int {
	nv, boundary := 2, countShapesWithLocSets(2, 4, 3)
	for i >= boundary && nv < max {
		nv++
		boundary += countShapesWithLocSets(nv, 4, 3)
	}
	return nv
}
func readENRTRecord(r io.Reader, i, max int) ([]byte, error) {
	b := make([]byte, 1, 34)
	if _, e := io.ReadFull(r, b); e != nil {
		return nil, e
	}
	if b[0] == 255 {
		return b, nil
	}
	n := int(b[0])
	if n != expectedENRTVreg(i, max) {
		return nil, fmt.Errorf("invalid ENRT record vreg at%d", i)
	}
	b = append(b, make([]byte, 2+n+2+24)...)
	_, e := io.ReadFull(r, b[1:])
	return b, e
}

// BuildENRTIndex performs one bounded-memory canonical full stream scan.
func BuildENRTIndex(path string) (*ENRTIndex, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || stat.Size() > maxENRTSize {
		return nil, fmt.Errorf("ENRT source size limit")
	}
	all := sha256.New()
	r := bufio.NewReaderSize(io.TeeReader(io.LimitReader(f, maxENRTSize+1), all), 128<<10)
	h := make([]byte, 16)
	if _, e = io.ReadFull(r, h); e != nil {
		return nil, e
	}
	max, count, e := enrtHeader(h)
	if e != nil {
		return nil, e
	}
	idx := &ENRTIndex{Schema: "enrt-checkpoints/v1", MaxVregs: max, Records: count, Stride: enrtStride, HeaderSHA256: fmt.Sprintf("%x", sha256.Sum256(h))}
	offset := int64(16)
	for first := 0; first < count; first += enrtStride {
		block := ENRTCheckpoint{First: first, Count: min(enrtStride, count-first), Offset: offset}
		hash := sha256.New()
		for i := first; i < first+block.Count; i++ {
			b, e := readENRTRecord(r, i, max)
			if e != nil {
				return nil, e
			}
			hash.Write(b)
			offset += int64(len(b))
		}
		block.Length = offset - block.Offset
		block.SHA256 = fmt.Sprintf("%x", hash.Sum(nil))
		idx.Blocks = append(idx.Blocks, block)
	}
	if _, e = r.ReadByte(); e != io.EOF {
		return nil, fmt.Errorf("ENRT trailing data or size limit")
	}
	if offset != stat.Size() {
		return nil, fmt.Errorf("ENRT source changed during index build")
	}
	idx.SourceSize = offset
	idx.SourceSHA256 = fmt.Sprintf("%x", all.Sum(nil))
	return idx, nil
}
func hashENRTPath(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, maxENRTSize+1))
	if e != nil || n > maxENRTSize {
		return "", 0, fmt.Errorf("ENRT validation size/error")
	}
	return fmt.Sprintf("%x", h.Sum(nil)), n, nil
}
func OpenENRTOperation(path, indexPath string) (*ENRTOperation, error) {
	indexFile, e := os.Open(indexPath)
	if e != nil {
		return nil, e
	}
	defer indexFile.Close()
	raw, e := io.ReadAll(io.LimitReader(indexFile, (2<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > 2<<20 {
		return nil, fmt.Errorf("ENRT index too large")
	}
	idx := ENRTIndex{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&idx); e != nil {
		return nil, e
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("extra index data")
	}
	if idx.Schema != "enrt-checkpoints/v1" || idx.Stride != enrtStride || idx.SourceSize < 16 || idx.SourceSize > maxENRTSize || len(idx.SourceSHA256) != 64 || len(idx.HeaderSHA256) != 64 {
		return nil, fmt.Errorf("invalid checkpoint metadata")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	op := &ENRTOperation{path: path, file: f, index: idx}
	fail := func(e error) (*ENRTOperation, error) { f.Close(); return nil, e }
	h := make([]byte, 16)
	if _, e = f.ReadAt(h, 0); e != nil {
		return fail(e)
	}
	max, count, e := enrtHeader(h)
	if e != nil {
		return fail(e)
	}
	if idx.MaxVregs != max || idx.Records != count || idx.HeaderSHA256 != fmt.Sprintf("%x", sha256.Sum256(h)) || len(idx.Blocks) != (count+enrtStride-1)/enrtStride {
		return fail(fmt.Errorf("index/source header mismatch"))
	}
	offset := int64(16)
	for j, b := range idx.Blocks {
		if b.First != j*enrtStride || b.Count != min(enrtStride, count-b.First) || b.Offset != offset || b.Length < int64(b.Count) || b.Length > int64(b.Count*34) || len(b.SHA256) != 64 {
			return fail(fmt.Errorf("invalid checkpoint bounds/order"))
		}
		offset += b.Length
	}
	if offset != idx.SourceSize {
		return fail(fmt.Errorf("index/source size mismatch"))
	}
	// Bind BOTH the opened descriptor and current source path. Path replacement
	// cannot silently bind an old descriptor to the new path's hash.
	hash := sha256.New()
	n, e := io.Copy(hash, io.LimitReader(f, maxENRTSize+1))
	if e != nil || n != idx.SourceSize || fmt.Sprintf("%x", hash.Sum(nil)) != idx.SourceSHA256 {
		return fail(fmt.Errorf("source SHA mismatch"))
	}
	return op, nil
}
func (o *ENRTOperation) Close() {
	if o != nil && !o.closed {
		o.closed = true
		o.file.Close()
	}
}
func (o *ENRTOperation) Validate() error {
	if o == nil || o.closed {
		return fmt.Errorf("closed ENRT operation")
	}
	h, n, e := hashENRTPath(o.path)
	if e != nil || n != o.index.SourceSize || h != o.index.SourceSHA256 {
		return fmt.Errorf("ENRT source identity changed before publication")
	}
	return nil
}
func (o *ENRTOperation) Import(p *LocalTableProblem, t *RegAllocTable, quotient bool) (*IndexedLegacyImportEvidence, error) {
	if o == nil || o.closed || p == nil || t == nil {
		return nil, fmt.Errorf("invalid ENRT operation")
	}
	context, e := localFunctionContext(p.Function)
	if e != nil || context != p.Options.CheckedContext {
		return nil, fmt.Errorf("stale local binding")
	}
	var shape *EnrichedShape
	var groups [][]int
	if quotient {
		shape, groups, e = indexedQuotientShape(p)
	} else {
		shape, e = indexedLegacyShape(p)
	}
	if e != nil {
		return nil, e
	}
	idx, e := EnrichedIndexOfWithLocSets(*shape, o.index.MaxVregs, 4, 3)
	if e != nil {
		return nil, e
	}
	block := o.index.Blocks[idx/enrtStride]
	data := make([]byte, block.Length)
	if _, e = o.file.ReadAt(data, block.Offset); e != nil {
		return nil, e
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != block.SHA256 {
		return nil, fmt.Errorf("ENRT checkpoint bytes changed")
	}
	r := bytes.NewReader(data)
	var selected []byte
	for i := block.First; i < block.First+block.Count; i++ {
		b, e := readENRTRecord(r, i, o.index.MaxVregs)
		if e != nil {
			return nil, e
		}
		if i == idx {
			selected = b
		}
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("checkpoint count/length mismatch")
	}
	ev := &IndexedLegacyImportEvidence{Schema: "vir-checkpoint-enrt-proposal/v1", Path: o.path, HeaderSHA256: o.index.HeaderSHA256, SourceSHA256: o.index.SourceSHA256, BlockSHA256: block.SHA256, BlockOffset: block.Offset, Count: uint32(o.index.Records), Shape: shape, RecordIndex: idx, BytesRead: block.Length}
	if selected[0] == 255 {
		ev.Reason = "historical record infeasible"
		return ev, nil
	}
	assignment := []int{}
	for _, loc := range selected[3 : 3+int(selected[0])] {
		assignment = append(assignment, int(loc))
	}
	admitted, e := admitIndexedAssignment(ev, assignment, int(binary.LittleEndian.Uint16(selected[1:3])), p, t, groups)
	if admitted != nil {
		admitted.Schema = "vir-checkpoint-enrt-proposal/v1"
	}
	return admitted, e
}
