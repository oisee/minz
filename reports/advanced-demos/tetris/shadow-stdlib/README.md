# Historical Tetris sprite generator

The shadow-candidate source and these two changed sprite-library files are
pinned to `df6f67b0`. The historical main/scene/shadow tests use an isolated
stdlib copy with these files overlaid. Other stdlib files and the compiler
remain current. This keeps new sprite helpers from growing the old binary
into its reserved E000..F7FF bitmap buffer. Current direct-renderer tests
always use the current stdlib.

- `zx/sprite.nanz`: SHA256 `b28327c3cc126d2f023f39baf265a10f21c0213bf8521b7c48a8c4e2a832f2f7`
- `zx/sprite_address.nanz`: SHA256 `ad4b56f2eb3f478afa3ada331e104f00a0ba451795363772c661bc1a22cea2f8`
