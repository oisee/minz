# Frozen system-C validation

This exercises **inih r62 string parsing** and the **heatshrink v0.4.1 static
decoder**. It is not a claim of complete libc, ISO C conformance, file I/O,
dynamic allocation, or the heatshrink encoder on Z80.

Upstream files under `upstream/` are byte-for-byte copies with licenses and
commit/file hashes in `upstream/manifest.json`. `freeze.json` separately freezes
headers, adapters, configuration and workloads. The runner verifies both before
and after every compiler profile.

The adapters include the original C source. inih uses its supported stack-buffer
configuration (`INI_MAX_LINE=128`, stop on first error). File functions are
explicit unavailable-file stubs; tests call the string API. heatshrink uses its
supported static configuration (32-byte input, window bits 8, lookahead bits 4).
`target_string.c` supplies portable target memcpy/memset/strlen/strchr. These
shims and the adapters are separate from upstream source.

inih expectations are manually written section/name/value records and status
codes for eight inputs: ordinary keys, comments, multiline values, malformed
section/key, empty input, BOM/CRLF, and duplicates. Seven heatshrink streams are
constructed from the documented MSB-first wire format, including binary literal
bytes, overlapping backreferences and the initial zero window. Expected output
is the original payload, not another MinZ execution.

GCC first checks the expected results and output using the same upstream sources
and adapter configuration. The target runner then checks raw MIR VM, optimized
MIR VM, and assembled Z80 in default, Size and Grace profiles. Each VM stage runs
even if another fails. Target checks cover every declared input/output byte,
heatshrink API status/count trace, the result, and native return PC/SP. They do
not compare the entire 64-KB address space or every internal decoder byte.

From the repository root:

```sh
GOCACHE=/tmp/minz-grace-gocache python3 research/system-c-validation/validate.py \
  --out /path/to/persistent/artifacts
```

Use `--go /path/to/go` to select the toolchain. GCC and the Go toolchain are
research/validation dependencies; this adds no dependency to normal compilation.
