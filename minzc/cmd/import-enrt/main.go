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
	output := flag.String("out", "", "new checked JSON table")
	evidence := flag.String("evidence", "", "JSON diagnostics, including rejections")
	stdlib := flag.String("stdlib", "", "original source import search path")
	flag.Parse()
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
	start = time.Now()
	for _, f := range base.MIR2Module.Funcs {
		p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
		d := decision{Function: f.Name}
		if e == nil {
			d.Evidence, e = vir.ImportIndexedENRT(*asset, p, table)
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
	if e = table.Save(*output); e != nil {
		return e
	}
	if e = os.WriteFile(*evidence, data, 0644); e != nil {
		return e
	}
	fmt.Printf("%d independently checked entries; diagnostics %s\n", table.Size(), *evidence)
	return nil
}
