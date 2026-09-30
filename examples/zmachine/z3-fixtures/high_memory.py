#!/usr/bin/env python3
"""Generate a minimal 131070-byte Z3 story with code and text near 128 KiB.

SPDX-License-Identifier: MIT
"""

from pathlib import Path
import sys

story = bytearray(131070)
story[0] = 3
story[4:8] = bytes.fromhex('00400040')  # high memory and initial PC
story[14:16] = bytes.fromhex('0040')     # static-memory base
story[26:28] = bytes.fromhex('ffff')     # file length / 2

# call packed $ff00 (byte $1fe00), store on stack, quit
story[0x40:0x46] = bytes.fromhex('e03fff0000ba')

# 0 locals; print "H" as a packed Z-string; newline; return true
story[0x1FE00:0x1FE05] = bytes.fromhex('00b291a5bb')
story[0x1FE05] = 0xB0

checksum = sum(story[64:]) & 0xFFFF
story[28:30] = checksum.to_bytes(2, 'big')
Path(sys.argv[1]).write_bytes(story)
