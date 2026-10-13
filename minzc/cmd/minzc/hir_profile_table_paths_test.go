package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHIRProfileTableInputAliases(t *testing.T) {
	hirProfileTestState(t)
	oldDir, oldWork, oldFun, oldTable := hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileAutoFunction, checkedLocalTablePath
	t.Cleanup(func() {
		hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileAutoFunction, checkedLocalTablePath = oldDir, oldWork, oldFun, oldTable
	})
	d := t.TempDir()
	source := filepath.Join(d, "source.nanz")
	table := filepath.Join(d, "table.json")
	os.WriteFile(source, []byte("fun f()->u8{return 0}"), 0644)
	os.WriteFile(table, []byte("[]"), 0644)
	checkedLocalTablePath = table
	hirProfile = hirProfileFlags{profile: "profile.json", workload: "workload.json", model: "model.json", decisions: table}
	if e := validateHIRProfileFlags(source); e == nil {
		t.Fatal("decisions overwrite table")
	}
	hirProfile = hirProfileFlags{}
	hirProfileAutoDirectory = filepath.Join(d, "auto")
	hirProfileAutoFunction = "f"
	hirProfileAutoWorkload = "workload.json"
	outputFile = table
	if e := validateHIRProfileAuto(source); e == nil {
		t.Fatal("product overwrite table")
	}
	outputFile = ""
	os.MkdirAll(hirProfileAutoDirectory, 0755)
	checkedLocalTablePath = filepath.Join(hirProfileAutoDirectory, "model.json")
	os.Link(table, checkedLocalTablePath)
	if e := validateHIRProfileAuto(source); e == nil {
		t.Fatal("artifact overwrite table")
	}
	checkedLocalTablePath = table
	// Alias via hard link, distinct filename.
	os.Remove(filepath.Join(hirProfileAutoDirectory, "model.json"))
	os.Link(table, filepath.Join(hirProfileAutoDirectory, "model.json"))
	if e := validateHIRProfileAuto(source); e == nil {
		t.Fatal("hardlink artifact overwrite table")
	}
}
