# Nanz chain spellings (unfrozen probe set)

This directory is **not** part of the frozen fusion suite: it has no manifest,
`run.py --suite fusion` does not read it, and its files may change. It holds
spellings of the fusion cases that use the iterator chain methods `skip`,
`take`, `zip`, `count` and `sum`, which the frozen `nanz-idiomatic/` sources
predate (they carry a second pointer or a counter by hand instead).

Each file is measured against the frozen workload of the case it names
(`../workloads/<case>.json`; a `_mapsum` / `_fold` suffix is an alternative
spelling of the same case):

    measure -compiler nanz -profile default -source <file> -out <dir> \
        -workload ../workloads/<case>.json

| File | Spelling |
|---|---|
| `fusion_filter_count.nanz` | `source.filter(\|x\| x == k).count(n)` |
| `fusion_dot_product.nanz` | `source.zip(other).fold(u16(0), \|s, x, y\| s + u16(x) * u16(y), n)` |
| `fusion_dot_product_mapsum.nanz` | `source.zip(other).map(\|x, y\| u16(x) * u16(y)).sum(n)` |
| `fusion_two_ptr_xor.nanz` | `source.zip(other).map(\|x, y\| x xor y).sum(n)` |
| `fusion_two_ptr_xor_fold.nanz` | `source.zip(other).fold(u16(0), \|s, x, y\| s + u16(x xor y), n)` |
| `fusion_window_sum.nanz` | `source.skip(4).take(limit).sum(n)` |
| `fusion_map_store.nanz` | identical to the frozen spelling: no chain method writes a second buffer, so `map` + `forEach` with a destination pointer stays |

`measured-default.json` records one run of every file here and of the frozen
`nanz-idiomatic/` spelling of the same case, both compiled by the compiler at
the commit it names (default profile): status, sample count, mismatches,
TotalT (summed over samples) and ProgramBytes.
