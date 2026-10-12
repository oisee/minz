package main

import "testing"

func TestContractStrategyCLIAdmission(t *testing.T) {
	oldStrategy, oldBackend, oldAuto, oldReplay := contractStrategy, backend, hirProfileAutoDirectory, hirProfileCompilerModesReplay
	defer func() {
		contractStrategy, backend, hirProfileAutoDirectory, hirProfileCompilerModesReplay = oldStrategy, oldBackend, oldAuto, oldReplay
	}()
	backend = "z80"
	contractStrategy = "pbqp"
	if err := validateContractStrategyCLI("input.nanz"); err != nil {
		t.Fatal(err)
	}
	backend = "ez80"
	if validateContractStrategyCLI("input.nanz") == nil {
		t.Fatal("accepted ez80")
	}
	backend = "z80"
	for _, s := range []string{"bogus", "PBQP"} {
		contractStrategy = s
		if validateContractStrategyCLI("input.nanz") == nil {
			t.Fatal("accepted unknown")
		}
	}
	contractStrategy = "pbqp"
	hirProfileAutoDirectory = "evidence"
	if validateContractStrategyCLI("input.nanz") == nil {
		t.Fatal("silently ignored in search")
	}
	hirProfileAutoDirectory = ""
	hirProfileCompilerModesReplay = "saved"
	if validateContractStrategyCLI("input.nanz") == nil {
		t.Fatal("silently ignored in replay")
	}
	hirProfileCompilerModesReplay = ""
	if validateContractStrategyCLI("input.a80") == nil {
		t.Fatal("ignored outside HIR")
	}
}
