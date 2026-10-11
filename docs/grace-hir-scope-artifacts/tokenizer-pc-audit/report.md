# Exact baseline tokenizer PC attribution

Replayed saved production assembly and workload through the unchanged `RunZ80ProfileWorkload` emulator/bootstrap/oracle harness. A temporary Go `-overlay` instruments only the existing step closure, recording instruction **start** PC and cycles returned by `z.Step()`. Bootstrap steps are excluded; all nested CALL/RET and tail-call instructions are charged to the physical function whose range contains that instruction, not inclusively to its caller.

Total: **2,270,286 T**, exactly the saved baseline. Binary SHA256: `048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33`. Saved assembly matches byte for byte. All 30 invocation results, snapshots, global/resource accesses, bootstrap hashes/costs, executed-step count and total timing match the saved target evidence. The instrumented helper's compatibility executable identity differs, as expected; that compatibility field is explicitly not asserted equal to the old executable.

| Physical function | Exclusive T | Share |
|---|---:|---:|
| intern | 524,924 | 23.12% |
| scan | 458,306 | 20.19% |
| peek_char | 301,086 | 13.26% |
| region_eq | 224,008 | 9.87% |
| checksum_records | 155,352 | 6.84% |
| emit | 113,616 | 5.00% |
| is_alpha | 102,822 | 4.53% |
| is_alnum | 75,434 | 3.32% |
| copy_forward | 74,271 | 3.27% |
| is_digit | 68,074 | 3.00% |
| scan$split_19 | 16,137 | 0.71% |

`intern` is the largest individual physical function. Its 23.12% excludes 9.87% executed in `region_eq`; these are not caller-inclusive totals. The scanner's work is distributed among scan, peek_char and character helpers. The data supports focusing on search and scanner call/address/spill overhead. It does not establish a speedup for any proposed rewrite.

Function boundaries come from every `; fun NAME(` assembly header, resolved against the actual assembled symbol table, ordered by address. Local labels stay within their owner range; generated `scan$split_19` is separate. All executed non-bootstrap PCs fall in these code ranges. `/tmp/minz-tokenizer-function-costs.json` contains per-call totals, function counts, boundaries and hottest PCs; `/tmp/minz-tokenizer-pc-audit.json` retains the complete call/PC/cycle table and actual symbols.

Reproduce from `/tmp/minz-grace-pgo-resume/minzc`:

```
MINZ_PC_AUDIT_OUTPUT=/tmp/minz-tokenizer-pc-audit.json GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run -overlay=/tmp/minz-tokenizer-pc-overlay.json /tmp/minz-tokenizer-pc-audit.go
```

Temporary helper `/tmp/minz-tokenizer-pc-audit.go`; temporary instrumented overlay `/tmp/minz-tokenizer-profile-workload-audit.go`; mapping `/tmp/minz-tokenizer-pc-overlay.json`; log `/tmp/minz-tokenizer-pc-audit.log`. No compiler/project files changed. The original helper attempted full evidence equality and rejected the compatibility identity change; the final assertion deliberately checks executable, all runtime evidence and timing instead, never hides a runtime mismatch.
