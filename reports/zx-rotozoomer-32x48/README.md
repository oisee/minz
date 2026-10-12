# ZX Spectrum half-cell rotozoomer — prototype

Alice proposed a 32×48 colour surface: each fixed 8×8 bitmap cell has four zero rows above four FF rows. PAPER colours the upper 8×4 rectangle, INK the lower one, with shared BRIGHT. Only 768 attribute bytes change per frame. The Nanz algorithm uses 8.8 coordinates and additive stepping, following antique-toy chapter7.

source.nanz compiles to Z80. Independent model.json and previews describe the intended 32 frames. Raw/final VM matches that model after exposing a 64KiB target memory map. Current native validation FAILS frame0 attribute0 (4d instead of6d) in allthree modes: intermediate top colour is lost while evaluating the second texture sample. The backend issue is under investigation. This prototype and model preview are not a working native showcase yet.

Native tests check every attribute, fixed bitmap, exact return PC/SP and code integrity. Reports preserve only the exercised scopes; ULA contention and hardware timing are not measured. Test: TestZXHalfCellRotozoomerNative; export variable MINZ_ZX_ROTO_EXPORT.

Reference book cloned at /home/alice/dev/minz-artifacts/reference/antique-toy, repository https://github.com/oisee/antique-toy, initial HEAD7313ecfaab710d1a105b7bff3be19aaf04dc5935.
