package pipeline

import "testing"

// Original CASE selects 66; the repeat terminates after Done=true.
// This is an independent exact stdout expectation, frozen before execution.
func TestOriginalPascalCaseCPMRuntime(t *testing.T) {
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			pascalCPMProgramMode(t, true, mode.name, mode.o, "../../../reports/pascal-case-runtime", []struct{ name, want string }{{"casetest", "Beta\r\n"}})
		})
	}
}
