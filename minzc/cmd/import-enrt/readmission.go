package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/vir"
)

func readmitFile(source, proposals, output, evidence, stdlib string, limit int) error {
	if source == "" || proposals == "" || output == "" || evidence == "" || limit < 1 || limit > vir.MaxLegacyCandidates {
		return fmt.Errorf("requires --source --checked-proposals --out --evidence and limit1..1024")
	}
	out, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	ev, err := filepath.Abs(evidence)
	if err != nil {
		return err
	}
	if out == ev {
		return fmt.Errorf("table and evidence paths must differ")
	}
	for _, path := range []string{output, evidence} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("refusing existing output %s", path)
		}
	}
	src, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(proposals)
	if err != nil {
		return err
	}
	var pool vir.RegAllocTable
	pool.Init()
	if err := pool.LoadJSON(data); err != nil {
		return err
	}
	h, err := nanz.ParseWithOpts(string(src), source, nanz.ParseOpts{BaseDir: filepath.Dir(source), StdlibDir: stdlib})
	if err != nil {
		return err
	}
	table, notes, err := pipeline.ReadmitCheckedLocalTables(h, pipeline.Options{OptSize: true, ContractOpt: true, AssertMode: "none"}, &pool, limit)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".readmit-table-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if err := table.Save(name); err != nil {
		return err
	}
	tableData, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	packet := struct {
		Schema          string   `json:"schema"`
		SourceSHA256    string   `json:"source_sha256"`
		ProposalsSHA256 string   `json:"proposals_sha256"`
		TableSHA256     string   `json:"table_sha256"`
		Limit           int      `json:"proposal_limit"`
		Entries         int      `json:"entries"`
		Notes           []string `json:"notes"`
	}{"offline-checked-readmission/v1", fmt.Sprintf("%x", sha256.Sum256(src)), fmt.Sprintf("%x", sha256.Sum256(data)), fmt.Sprintf("%x", sha256.Sum256(tableData)), limit, table.Size(), notes}
	diagnostic, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return err
	}
	if err := writeNew(output, tableData); err != nil {
		return err
	}
	if err := writeNew(evidence, diagnostic); err != nil {
		os.Remove(output)
		return err
	}
	fmt.Printf("%d current checked entries; diagnostics %s\n", table.Size(), evidence)
	return nil
}
