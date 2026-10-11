# Caller-owned struct ABI integration checkpoint

This is an intermediate correctness checkpoint, not completion of arbitrary aggregate ABI support.

The default caller-owned buffers replace dangling callee frame addresses. Calls now retain semantic indirect parameter/return types; hidden pointers are classified as pointers rather than inferred from their physical DE allocation. The earlier WIP 475298f7 reproduced an indirect write to the function image because HL still held the target address. This checkpoint corrects that argument setup.

TestStructReturnLifetimeBufferedNative: unchanged frozen source-call.nanz, ten independent delayed/two-return cases in each of ordinary/GraceSpeed/GraceSize (30 native calls), raw/final VM parity.
TestStructReturnNestedAndRecursiveNative: nested five-byte layouts, intervening calls, recursive parent values and typed indirect calls; 23 cases per mode (69 native calls), raw/final VM, expected scratch=83, full 64KiB comparison except exact declared global/spill bytes and observed stack extent; original PC/SP return required.
TestAggregateAddressClassesNative: 2304 physical direct calls, now supplies the derived hidden pointer.
TestStructReturnMetadataRejectsForgedIndirectABI: six malformed descriptor/signature cases reject.
TestStructReturnPublicVMKeepsSourceArguments: public invocation keeps source arity, supplies distinct owned buffers, and rejects physical hidden arguments.

Reproduce from minzc:
GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^Test(StructReturn|AggregateAddressClassesNative)' -count=1 -v
Set MINZ_STRUCT_BUFFER_ARTIFACT_DIR to an absolute empty directory to retain exact ASM/BIN/MIR. Ordinary test runs do not rewrite historical packets.

Remaining audit/implementation: native aggregate public assertions and complete multi-return adapters; dynamic VM indirect target signature validation; ownership of cloned ABI metadata; repeated loop call-site result reuse and nested nonliteral aggregate fields; original Widemath replay after integration. Extern struct returns and controlled value-only profile entrypoints are explicitly unsupported rather than silently assigned the wrong ABI. Existing indirect live-value admission remains conservative. No final whole-language aggregate or goal-completion claim.
