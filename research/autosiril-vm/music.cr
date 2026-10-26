# Musical kernels exported by the real native semantic Crystal frontend.
# Nanz calls these through ordinary scalar HIR ABI, not host callbacks.
# Inputs are nonnegative; ppqn/per_beat are checked by the caller.
def crystal_quantize(tick : UInt32, ppqn : UInt32, per_beat : UInt32) : UInt32
  whole = (tick // ppqn) &* per_beat
  fraction = ((tick % ppqn) &* per_beat) &+ (ppqn // 2_u32)
  whole &+ (fraction // ppqn)
end

# FNote cell mask: start=1, continue=2, release=4. Ruby drops continue-only
# cells, releases the lowest note for release-only cells, otherwise retriggers.
def crystal_cell_action(mask : UInt8) : UInt8
  return 0_u8 if mask == 0_u8
  return 0_u8 if mask == 2_u8
  return 4_u8 if mask == 4_u8
  1_u8
end

def crystal_prefer(current : UInt8, candidate : UInt8, mode : UInt8) : UInt8
  if mode == 109_u8 # 'm': highest note. Drums/release: lowest note.
    candidate > current ? candidate : current
  else
    candidate < current ? candidate : current
  end
end

# Semantic inference roots, never executed by the exporter.
crystal_quantize(0_u32, 480_u32, 8_u32)
crystal_cell_action(0_u8)
crystal_prefer(60_u8, 64_u8, 109_u8)

# VortexTracker octave numbering, preserving Autosiril's low-octave convention.
def crystal_octave(note : UInt16) : UInt16
  q = note // 12_u16
  octave = q < 2_u16 ? q : q &- 1_u16
  octave > 8_u16 ? 8_u16 : octave
end

def crystal_ornament_shift(note : UInt8, ornament : UInt16) : UInt8
  return note if ornament == 0_u16
  note = note &- 12_u8 if note >= 72_u8
  note = note &+ 12_u8 if note <= 48_u8
  note
end

def crystal_keep_offset(offset : UInt8, median : UInt8, limit : UInt8) : Bool
  distance = offset > median ? offset &- median : median &- offset
  distance <= limit
end

# The original starts every note at volume 15. Its two echoes therefore have
# only these values: 10.5 and 7.35. Centi-volume preserves ordering and display
# truncation without introducing a general floating-point frontend claim.
def crystal_echo_volume(bounce : UInt8) : UInt16
  bounce == 1_u8 ? 1050_u16 : 735_u16
end
crystal_octave(60_u16)
crystal_ornament_shift(60_u8, 0_u16)
crystal_keep_offset(0_u8, 0_u8, 24_u8)
crystal_echo_volume(1_u8)

# Preserve the reference converter's historical boundary rule for byte-identical
# VT output. A corrected pattern policy can be added separately after parity.
def crystal_pattern_count(rows : UInt32, size : UInt32) : UInt32
  count = rows // size
  count = count &+ 1_u32 if (rows % size) == 0_u32
  count
end
crystal_pattern_count(128_u32, 64_u32)
