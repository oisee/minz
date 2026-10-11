# Imported globals and the ZX font reference

Imported globals/constants now preserve local-to-mangled symbol aliases
for both glob and selected/renamed imports. Lexical locals still shadow
imported names. Five independent read/write/alias/shadow expectations pass
MIR2 VM and actual assembled Z80. sqlite_test now assembles instead of
failing on unknown SQLITE_ROW.

zsql_zx referenced nonexistent _zx_font although the font bytes were already
emitted under _font_anchor. It now uses that existing data symbol and
assembles. No empty font or dummy function was supplied.

Other 192-file showcase known skips remain unchanged; Agon hello_world
legacy MOS syntax is still under investigation. The broader goal includes
verified table admission, measured PFCCO/Grace benefit and full LIR removal;
these are not complete merely because compile failures are nearly closed.
