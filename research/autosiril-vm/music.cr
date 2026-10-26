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
