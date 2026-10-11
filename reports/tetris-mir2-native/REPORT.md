# Original tetris_mir2_test geometry and persistent board proof

Exact unchanged examples/zx/tetris_mir2_test.nanz, frozen before execution in919fa65c. All original seven callables including main and18source assertions retained. No source/game-algorithm rewrite or native host substitute.

Geometry expectation uses human-authored ASCII grids for standard I,O,T,S,Z,L,J cell sets, frozen independently of generated HIR/native LUT. Each of28published orientation states is a four-cell raster; block c is row-major order in that canonical state. I repeats horizontal/vertical and O repeats square as source convention. This is an independently specified canonical raster model, not general SRS rotation/pivot mechanics or a model derived by reading compiled global bytes. All t0..6,r0..3,c0..3 dx/dy=224queries. Color bytes use the original published ZX attribute enum values, clearly source data authority rather than a universal tetromino color standard:7piececolors+8cellcolors.

Board independent model is offset10*y+x in200byte array, x0..9,y0..19. Persistent model/VM/native instances receive board_set then board_get for every one of200cells×256bytevalues:102400calls per mode, including store values equal to existing bytes. Exact data-event observer requires one correct byte write/read at source cell, and full64KiB compares every call with only explicitly expected cell allowed to change. All other199boardcells, adjacent guards, LUT/color data, image/code and nonprivateRAM protected. Native only exact observed private stack extent/declared spill homes masked. Symbol addresses normalize the same independent array model for VM/native layouts.

Source assertions execute before board mutations, not silently omitted; original empty main actually ends XOR A/DI/HALT withA0/SPeff0. Each returning source function uses actual allocated parameter/return ABI and requires PC7000/SPeff2. Main is not inaccurately treated as a returning function.

Ordinary/GraceSize/GraceSpeed each102658calls=307974actualnative invocations plus matching raw/final VM. Final ordered board effect proof PASS5.567s. No compiler edits or broad/full pipeline run. Pure native computational Tetris subset only: no complete game/rendering/input/ZX device runtime assertion.

All source/freeze, full actual raw/final MIR/ASM/BIN/allocation/cost logs tracked and manifest hashes verified. Root owns corpus projection. Initial proof log retained separately; final code adds exact board access sequence checks.
