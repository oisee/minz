// import-enrt converts addressed historical allocation proposals into current
// checked certificates. It never changes the legacy table or uses its cost.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/vir"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	source := flag.String("source", "", "original Nanz source")
	asset := flag.String("enrt", "", "canonical ENRT v1 table (4v or 5v)")
	checked := flag.String("checked-proposals", "", "saved checked JSON assignments to readmit without ENRT or solver")
	limit := flag.Int("proposal-limit", 1024, "maximum saved assignments for readmission (1..1024)")
	output := flag.String("out", "", "new checked JSON table")
	evidence := flag.String("evidence", "", "JSON diagnostics, including rejections")
	indexPath := flag.String("index", "", "optional checked checkpoint index; validates full ENRT once at open and final publication")
	buildIndex := flag.String("build-index", "", "build new checkpoint index offline; requires only --enrt")
	quotient := flag.Bool("quotient", false, "explicitly collapse nonoverlapping same-register allocation variables before historical lookup")
	stdlib := flag.String("stdlib", "", "original source import search path")
	flag.Parse()
	if *checked != "" {
		if flag.NArg() != 0 || *asset != "" || *indexPath != "" || *buildIndex != "" || *quotient {
			return fmt.Errorf("--checked-proposals cannot combine with ENRT/index/quotient modes")
		}
		return readmitFile(*source, *checked, *output, *evidence, *stdlib, *limit)
	}
	if *buildIndex != "" {
		if flag.NArg() != 0 || *asset == "" || *source != "" || *output != "" || *evidence != "" || *indexPath != "" || *quotient {
			return fmt.Errorf("--build-index requires only --enrt and new index path")
		}
		idx, e := vir.BuildENRTIndex(*asset)
		if e != nil {
			return e
		}
		data, e := json.MarshalIndent(idx, "", "  ")
		if e != nil {
			return e
		}
		return writeNew(*buildIndex, data)
	}
	if flag.NArg() != 0 || *source == "" || *asset == "" || *output == "" || *evidence == "" {
		return fmt.Errorf("requires --source --enrt --out --evidence; optional --stdlib")
	}
	outAbs, _ := filepath.Abs(*output)
	evAbs, _ := filepath.Abs(*evidence)
	if outAbs == evAbs {
		return fmt.Errorf("table and evidence paths must differ")
	}
	for _, p := range []string{*output, *evidence} {
		if _, e := os.Stat(p); !os.IsNotExist(e) {
			return fmt.Errorf("refusing existing output %s", p)
		}
	}
	src, e := os.ReadFile(*source)
	if e != nil {
		return e
	}
	h, e := nanz.ParseWithOpts(string(src), *source, nanz.ParseOpts{BaseDir: filepath.Dir(*source), StdlibDir: *stdlib})
	if e != nil {
		return e
	}
	start := time.Now()
	base, e := pipeline.CompileHIRSteps(h, pipeline.Options{ContractOpt: true, OptSize: true, AssertMode: "none"})
	if e != nil {
		return e
	}
	compileElapsed := time.Since(start)
	table := &vir.RegAllocTable{}
	table.Init()
	type decision struct {
		Function string                           `json:"function"`
		Evidence *vir.IndexedLegacyImportEvidence `json:"evidence,omitempty"`
		Error    string                           `json:"error,omitempty"`
	}
	notes := []decision{}
	var operation *vir.ENRTOperation
	if *indexPath != "" {
		operation, e = vir.OpenENRTOperation(*asset, *indexPath)
		if e != nil {
			return e
		}
		defer operation.Close()
	}

	start = time.Now()
	for _, f := range base.MIR2Module.Funcs {
		p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
		d := decision{Function: f.Name}
		if e == nil {
			if operation != nil {
				d.Evidence, e = operation.Import(p, table, *quotient)
			} else if *quotient {
				d.Evidence, e = vir.ImportIndexedENRTQuotient(*asset, p, table)
			} else {
				d.Evidence, e = vir.ImportIndexedENRT(*asset, p, table)
			}
		}
		if e != nil {
			d.Error = e.Error()
		}
		notes = append(notes, d)
	}
	packet := struct {
		Schema       string     `json:"schema"`
		Source       string     `json:"source"`
		SourceSHA256 string     `json:"source_sha256"`
		CompileNS    int64      `json:"compile_ns"`
		ImportNS     int64      `json:"import_ns"`
		Entries      int        `json:"entries"`
		Decisions    []decision `json:"decisions"`
	}{"offline-enrt-conversion/v1", *source, fmt.Sprintf("%x", sha256.Sum256(src)), compileElapsed.Nanoseconds(), time.Since(start).Nanoseconds(), table.Size(), notes}
	data, e := json.MarshalIndent(packet, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(filepath.Dir(*output), ".enrt-table-*")
	if e != nil {
		return e
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	if e = table.Save(tmpPath); e != nil {
		return e
	}
	if operation != nil {
		if e = operation.Validate(); e != nil {
			return e
		}
	}
	tableData, e := os.ReadFile(tmpPath)
	if e != nil {
		return e
	}
	if e = writeNew(*output, tableData); e != nil {
		return e
	}
	if e = writeNew(*evidence, data); e != nil {
		os.Remove(*output)
		return e
	}
	fmt.Printf("%d independently checked entries; diagnostics %s\n", table.Size(), *evidence)
	return nil
}

// Publish through a same-directory hard link: no overwriting existing files,
// and readers see the complete written bytes. Pair publication is sequential.
func writeNew(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".enrt-publish-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Link(tmp, path)
}
