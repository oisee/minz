# Original native captures

These four files are byte-for-byte copies of the retained native captures.
[manifest.json](manifest.json) records their size, SHA256 and original archive path.

| Capture | Native product | Product SHA256 |
|---|---|---|
| [CP/M still](cpm-grid.png) / [session](cpm-session.gif) | FLEET.COM, 40,257 bytes | `2de170d533831fe6dc8a6a4db273a200eed4d61f4ef51f45be6bfc3f908a6617` |
| [Agon still](agon-grid.png) / [session](agon-session.gif) | FLEET.BIN, 45,652 bytes | `02512a48a64143224b44a5d4021b19569224d4ce3f42a4bc51d0f393e7abd302` |

Both use the same ABAP source: SHA256
`50a48e0726154b3d37115bfdcd09d4f6222c7c2e3380293128cb489c75fb7979`.
Chain: ABAP → MinZ HIR → MIR2 → Z80. Agon executes Z80-mode code on
eZ80 with native 24-bit MOS bridges.

CP/M: genuine X11/MZE terminal capture, 80 retained frames over about 8 seconds
at nominal 10 fps, with native F4/selection/display/Back and OS return.
Agon: genuine Fab eZ80 SDL capture with pinned MOS/full VDP and raw SD SPI;
host filesystem interception is disabled. There are 156 retained samples from
SDL updates 6..936 at stride 6, observed over 15.756140259 seconds. Identical
adjacent frames merge losslessly into 31 encoded GIF frames with summed duration.
The clip includes real debugger pauses. Its last frame has a minimal 10 ms hold
because the end timestamp is unknown. These are not performance measurements.

No intentional speed-up, interpolation, cropping or compositing was applied.
The Agon 80×24 grid occupies the top 640×192 of the original 640×480 framebuffer.
Its two rows clamp the viewport at zero; this clip does not demonstrate nonzero
scrolling. Both show a project retro grid, not SAP GUI pixel equivalence.
These are emulator captures, not physical-hardware or physical-SD certificates.

For the verification scope, reproduction commands and local raw archive paths,
see the [end-to-end report](../../2026-10-18-f4-csv-alv-e2e.md).
