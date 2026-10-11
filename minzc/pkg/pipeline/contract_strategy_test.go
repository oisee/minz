package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestContractStrategyAdmissionAndIdentity(t *testing.T) {
	valid := Options{ContractOpt: true, AssertMode: "none"}
	legacyID, err := ProfileCompilationID(valid, "default")
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []string{"", "greedy", "pbqp"} {
		opts := valid
		opts.ContractStrategy = strategy
		if err := ValidateContractStrategy(opts); err != nil {
			t.Fatal(err)
		}
		id, err := ProfileCompilationID(opts, "default")
		if err != nil {
			t.Fatal(err)
		}
		if strategy != "" && id == legacyID {
			t.Fatal("explicit option unbound")
		}
		h, err := nanz.Parse("fun f(x:u8)->u8 { return x+3 }", "strategy.nanz")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = CompileHIRSteps(h, opts); err != nil {
			t.Fatal(err)
		}
		if _, err = CompileHIRWithOptions(h, opts); err != nil {
			t.Fatal(err)
		}
	}
	for _, opts := range []Options{{ContractOpt: true, ContractStrategy: "bogus"}, {ContractStrategy: "pbqp"}, {ContractStrategy: "greedy"}, {ContractOpt: true, ContractStrategy: "pbqp", Backend: "ez80"}} {
		if _, err := CompileHIRSteps(&hir.Module{}, opts); err == nil {
			t.Fatal("accepted invalid options")
		}
		if _, err := CompileHIRWithOptions(&hir.Module{}, opts); err == nil {
			t.Fatal("assembly API accepted invalid options")
		}
		if _, err := ProfileCompilationID(opts, "default"); err == nil {
			t.Fatal("identity accepted invalid options")
		}
	}
	h, _ := nanz.Parse(contractFlagSource, "strategy.nanz")
	base, err := CompileHIRSteps(h, valid)
	if err != nil {
		t.Fatal(err)
	}
	h, _ = nanz.Parse(contractFlagSource, "strategy.nanz")
	explicit := valid
	explicit.ContractStrategy = "greedy"
	same, err := CompileHIRSteps(h, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if base.Assembly != same.Assembly {
		t.Fatal("default is not greedy")
	}
}
