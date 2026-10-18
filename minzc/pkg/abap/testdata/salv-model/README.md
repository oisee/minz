Independent project grid model created by the coordinator before native SALV output
was available. Input: declared80x24 layout/cell/scroll contract, not compiler/runtime
or native golden output. This is not SAP UI equivalence. fixtures.json contains
8exact24x80 screens plus input metadata; grid_model.py produces them independently.
ABAP semantic data excerpts use the pinned runtime separately.
all256-bytes.json is a separate independent fixture added after the original8,
without changing them. all256.py lays out16rows of16consecutive byte values plus
an i32offset; it covers each byte0..255 exactly once. This is explicitly a typed
storage fixture, not an admission claim for high-byte ABAP source literals.
