# Borrowed Slice iteration: no slice allocation, iterator object or GC on target.
def each_sum(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  sum = 0_u8
  Slice.new(p, 3_u8).each do |value|
    sum = sum &+ value
  end
  sum
end

def each_break_alias(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = 165_u8
  p[3_u16] = 90_u8
  sum = 0_u8
  visited = 0_u8
  Slice.new(p, 3_u8).each do |value|
    visited = visited &+ 1_u8
    sum = sum &+ value
    if visited == 1_u8
      p[1_u16] = seed &+ 7_u8
    end
    break if visited == 2_u8
    p[3_u16] = 60_u8
  end
  sum ^ visited ^ p[2_u16] ^ p[3_u16]
end

def each_next(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  seen = 0_u8
  sum = 0_u8
  Slice.new(p, 3_u8).each do |value|
    seen = seen &+ 1_u8
    next if seen == 2_u8
    sum = sum &+ value
  end
  sum ^ seen
end

def each_nonlocal_return(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  seen = 0_u8
  Slice.new(p, 3_u8).each do |value|
    seen = seen &+ 1_u8
    return value if seen == 2_u8
  end
  255_u8
end

def each_capture_bounds(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  length = 3_u8
  sum = 0_u8
  Slice.new(p, length).each do |value|
    length = 0_u8
    p = Pointer(UInt8).null
    sum = sum &+ value
  end
  sum ^ length
end

def each_nested(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  sum = 0_u8
  Slice.new(p, 2_u8).each do |left|
    Slice.new(p, 2_u8).each do |right|
      sum = sum &+ left &+ right
    end
  end
  sum
end

def each_word(seed : UInt16)
  raw = StaticArray(UInt16, 3).new(0_u16)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1000_u16
  p[2_u16] = 23130_u16
  sum = 0_u16
  Slice.new(p, 2_u8).each do |word|
    sum = sum ^ word
  end
  sum ^ p[2_u16]
end

def each_wide(seed : UInt8, length : UInt16)
  raw = StaticArray(UInt8, 256).new(1_u8)
  raw.to_unsafe[255_u16] = seed
  sum = 0_u16
  Slice.new(raw.to_unsafe, length).each do |value|
    sum = sum &+ value.to_u16
  end
  sum
end

def each_empty(seed : UInt8)
  sum = seed
  Slice.new(Pointer(UInt8).null, 0_u16).each do |value|
    sum = sum &+ value
  end
  sum
end

each_sum(250_u8)
each_break_alias(250_u8)
each_next(250_u8)
each_nonlocal_return(250_u8)
each_capture_bounds(250_u8)
each_nested(250_u8)
each_word(65535_u16)
each_wide(250_u8, 256_u16)
each_empty(250_u8)
