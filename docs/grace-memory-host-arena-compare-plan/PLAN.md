# Next slice: host-bound arena-first byte comparison, peel only

Reviewed actual unchanged `examples/nanz/self_lanz_parser.nanz` lines79-105 at5fdc6846. `region_match(arena_p:u16,src_start:u16,len:u8)->u8` is exactly three statements: local i:u8=0; while i<len { if peek((arena_p+1)+i)!=peek((src_buf+src_start)+i) return0; i=i+1 }; return1. `intern_region` calls it only after reading the candidate Pascal length and comparing that length with len. It scans variable-length arena entries, then copies source on miss. Do not transform intern scan/allocation yet.

Existing `hir-guarded-region-compare` certificate is inapplicable: it requires five typed pointer/offset/count parameters and two IndexExpr loads, whereas this function has three integer parameters, ordered host calls, and a mutable global src_buf read. Swapping argument interpretation into that recognizer or relabeling peek as pure would erase the host effect contract. Existing pointer guards are physical/logical nowrap certificates, not resource bounds and not host permissions.

Smallest implementable next slice: exact source-bound first-byte peeling ONLY, retaining both peek calls and original modular address expressions. No pointer induction/no guard/no global hoist. This creates a true reusable host-aware comparison proof without extending global or extern purity assumptions and lets actual caller decide profit.

Proposed transformed whole body:

if len==0 return1
if peek(arena_p+1)!=peek(src_buf+src_start) return0
if len==1 return1
var i:u8=1
while i<len {
  if peek((arena_p+1)+i)!=peek((src_buf+src_start)+i) return0
  i=i+1
}
return1

Addresses and call order are exact original left→right. All arithmetic remains u16 with the original associativity and modular wrap. The local induction value is never observable outside this exact body. Removing i=0 initialization/increment on len1 changes no declared memory/resource event. For len0 no host call or src_buf load occurs. First left failure prevents right call; first right failure preserves left read event. First mismatch returns immediately; len1 success returns immediately; tail begins at1, not0. No extra speculative memory access. Full caller profitability unknown until raw/native measurement; no source performance claim from pattern recognition.

## Host/source identity and legality

- Exact original whole function; three unique scalar parameters with typesu16/u16/u8, u8 single return, local inductionu8, no captures/SMC/IO/alternate ABI metadata. Match actual parsed CallExpr signatures and argument trees, not textual function names alone. Include function/global namespace collisions (mutated i global collision is lowerer-sensitive), generated names and exact metadata.
- `peek` must be the exact bound closed host descriptor `read-u8-at-u16`, matching extern ABI, workload context and privately emitted adapter; arbitrary extern with same name/signature is refused. Source descriptor may lack purity annotation; legality comes from closed host contract. No callbacks/ports. No replacing with generic LoadExpr, which would bypass adapter/permission/cost evidence.
- The left bound is parameter arena_p, right address uses src_buf GLOBAL on each original iteration. Keep those global reads and address/call evaluation order; no assumed immutability beyond the closed no-code-callback host. No source-global read hoisting. Preserve target global-provenance sidecar and raw symbolic-root checks unchanged.
- Whole-module pre/post proof: ordinary functions/globals/source units/source locations untouched; exact source function/path-bound loop identity and origin mapping. Original tail loop remains source-attributed; peeled prefix is explicit structural proof, not a fabricated fast/fallback count. Do not create fake function/loop observations. Fresh factory/retained origin identity and exact opt-in choices required.
- Closed memory-host baseline API currently has no optimization selection; initially a direct isolated candidate/test harness plus structural proof, not public PGO/default rule. Extend measured API only after actual original/peeled full30 independent host/native oracle passes. One actual loop site can be measured with original/peeled choices; old no-host APIs remain strict.

## Required independent negative/domain tests

- Every len0..255: equal and mismatch at each feasible position; arena/source aliases including same address and overlapping ranges. Source/resource bounds allow full declared region scans, but source logicallen does not substitute for host memory bounds.
- Modular addresses: arena_p65535 (first left wraps0), src_buf+src_start wraps, tail wraps; use original resource mapping, refuse protected code/global/stack/spill. No nowrap guard needed because expression semantics unchanged.
- First left denied: no right event. First right denied: exactly left read before typed failure. Tail left/right denied after equal prefix: exact prefix and typed failure{kind,address,write:false}. Equal byte values at wrong addresses cannot replace event identity. Never conflate infrastructure/codegen/stack/limit failures with semantic host denial.
- Forged post body: reverse calls, precompute right/global before left, hoist/copy src_buf once, extra duplicate read, drop failure call, len1 shortcut before firstpair, tail start0/2, altered u8 increment/u16 addition, wide peek, mutable host/other extern, wrong function metadata/source origins/loop marker, unrelated intern body change.
- Original unchanged full30 persistent parser source workload: full10globals/4096nodes/8192strings/4096source and every resource byte read/write match independent Python host, original/raw/optimized/native. Actual source/BIN/MIR/allocation/sidecar hashes and adapterTstates retained. Measure candidate separately, retain all failed/negative outcomes. Do not hardcode a speed winner.

## Deferred higher-cost variant

Only if peeling measured useful: guarded arena pointer induction for tail while keeping source-global right expression reads every iteration, or both pointers ONLY with independently justified closed host/global-event contract. Existing physical guard setup was expensive on many short comparisons; guard resource endpoints and modular expression rewrite separately. Never exchange arena/source order to reuse tokenizer's existing pointer certificate. Pascal variable-stride intern scanning is outside this slice.
