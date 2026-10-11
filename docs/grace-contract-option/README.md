# ContractOpt is respected by both compilation entry points

`CompileHIRSteps(h, Options{ContractOpt:false})` previously ran contract optimization unconditionally. `CompileHIRWithOptions` already respected the boolean. The steps entry point now uses the same condition. `DefaultOptions().ContractOpt` remains true.

The regression uses a runtime buffer summation loop whose first parameter changes class when optimization is enabled. With false, the original class must remain; with true, it must change. Both entry points produce assembled Z80 code returning the independent sum 1+2+3=6. The resulting MIR is also executed on the VM with the same data. The regression failed before the fix (see before.txt) and passes afterward.

Validation: `go test ./pkg/pipeline -run '^TestCompileHIRStepsRespectsContractOption$' -count=1 -v` passed (0.015 s).

Explicit zero-valued Options now disable contract optimization as their documented boolean requests. Historical measurements using that form actually had optimization enabled; their saved measurements are not rewritten. Tests that intended defaults should use DefaultOptions or explicitly set ContractOpt:true. This change establishes a real off baseline; it makes no native performance claim.
