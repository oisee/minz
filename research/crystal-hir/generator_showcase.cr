# Ordinary Crystal yielding methods. The bridge imports their one-way yield
# bodies as shared HIR generators; immediate block calls use the pull protocol.
def stream_peek(src : Pointer(UInt8), trace : Pointer(UInt8)) : UInt8
  trace[0_u16] = trace[0_u16] &+ 1_u8
  src[0_u16]
end

def byte_stream(src : Pointer(UInt8), n : UInt8, trace : Pointer(UInt8), &)
  while n != 0_u8
    trace[1_u16] = trace[1_u16] &+ 1_u8
    yield stream_peek(src, trace)
    trace[2_u16] = trace[2_u16] &+ 1_u8
    src = src + 1_u16
    n = n &- 1_u8
  end
  trace[3_u16] = 1_u8
  nil
end

def positive_stream(src : Pointer(UInt8), n : UInt8, trace : Pointer(UInt8), &)
  while n != 0_u8
    value = stream_peek(src, trace)
    src = src + 1_u16
    n = n &- 1_u8
    if value > 0_u8
      trace[1_u16] = trace[1_u16] &+ 1_u8
      yield value
      trace[2_u16] = trace[2_u16] &+ 1_u8
    end
  end
  trace[3_u16] = 1_u8
  nil
end

def rle_stream(src : Pointer(UInt8), pairs : UInt8, trace : Pointer(UInt8), &)
  while pairs != 0_u8
    left = stream_peek(src, trace)
    value = stream_peek(src + 1_u16, trace)
    src = src + 2_u16
    pairs = pairs &- 1_u8
    while left != 0_u8
      trace[1_u16] = trace[1_u16] &+ 1_u8
      yield value
      trace[2_u16] = trace[2_u16] &+ 1_u8
      left = left &- 1_u8
    end
  end
  trace[3_u16] = 1_u8
  nil
end

def stream_digest(sum : UInt8, trace : Pointer(UInt8))
  sum.to_u16 | (trace[0_u16].to_u16 &* 256_u16) |
    (trace[1_u16].to_u16 &* 2048_u16) |
    (trace[2_u16].to_u16 &* 8192_u16) |
    (trace[3_u16].to_u16 &* 32768_u16)
end

def generator_sum(seed : UInt8)
  data = StaticArray(UInt8, 4).new(0_u8)
  log = StaticArray(UInt8, 4).new(0_u8)
  p = data.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  sum = 0_u8
  byte_stream(p, 3_u8, log.to_unsafe) do |value|
    sum = sum &+ value
  end
  stream_digest(sum, log.to_unsafe)
end

def generator_break_alias(seed : UInt8)
  data = StaticArray(UInt8, 4).new(0_u8)
  log = StaticArray(UInt8, 4).new(0_u8)
  p = data.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = 165_u8
  sum = 0_u8
  seen = 0_u8
  byte_stream(p, 3_u8, log.to_unsafe) do |value|
    sum = sum &+ value
    seen = seen &+ 1_u8
    p[1_u16] = seed &+ 7_u8
    break if seen == 2_u8
  end
  stream_digest(sum, log.to_unsafe)
end

def generator_next(seed : UInt8)
  data = StaticArray(UInt8, 4).new(0_u8)
  log = StaticArray(UInt8, 4).new(0_u8)
  p = data.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  sum = 0_u8
  seen = 0_u8
  byte_stream(p, 3_u8, log.to_unsafe) do |value|
    seen = seen &+ 1_u8
    next if seen == 2_u8
    sum = sum &+ value
  end
  stream_digest(sum, log.to_unsafe)
end

def generator_return(seed : UInt8)
  data = StaticArray(UInt8, 4).new(0_u8)
  log = StaticArray(UInt8, 4).new(0_u8)
  p = data.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  sum = 0_u8
  seen = 0_u8
  byte_stream(p, 3_u8, log.to_unsafe) do |value|
    sum = sum &+ value
    seen = seen &+ 1_u8
    return stream_digest(sum, log.to_unsafe) if seen == 2_u8
  end
  65535_u16
end

def generator_filter_take(seed : UInt8)
  data = StaticArray(UInt8, 8).new(0_u8)
  log = StaticArray(UInt8, 4).new(0_u8)
  p = data.to_unsafe
  p[2_u16] = seed
  p[3_u16] = seed &+ 1_u8
  p[4_u16] = seed &+ 2_u8
  sum = 0_u8
  seen = 0_u8
  positive_stream(p, 5_u8, log.to_unsafe) do |value|
    sum = sum &+ value
    seen = seen &+ 1_u8
    break if seen == 2_u8
  end
  stream_digest(sum, log.to_unsafe)
end

def generator_rle(seed : UInt8)
  data = StaticArray(UInt8, 8).new(0_u8)
  log = StaticArray(UInt8, 4).new(0_u8)
  p = data.to_unsafe
  p[0_u16] = 0_u8
  p[1_u16] = 165_u8
  p[2_u16] = 2_u8
  p[3_u16] = seed
  p[4_u16] = 1_u8
  p[5_u16] = seed &+ 1_u8
  sum = 0_u8
  rle_stream(p, 3_u8, log.to_unsafe) do |value|
    sum = sum &+ value
  end
  stream_digest(sum, log.to_unsafe)
end

def infinite_stream(seed : UInt8, &)
  value = seed
  while true
    yield value
    value = value &+ 1_u8
  end
end

def generator_infinite_prefix(seed : UInt8)
  sum = 0_u8
  seen = 0_u8
  infinite_stream(seed) do |value|
    sum = sum &+ value
    seen = seen &+ 1_u8
    break if seen == 2_u8
  end
  sum ^ seen
end

def generator_empty(seed : UInt8)
  log = StaticArray(UInt8, 4).new(0_u8)
  sum = seed
  byte_stream(Pointer(UInt8).null, 0_u8, log.to_unsafe) do |value|
    sum = sum &+ value
  end
  stream_digest(sum, log.to_unsafe)
end

generator_sum(250_u8)
generator_break_alias(250_u8)
generator_next(250_u8)
generator_return(250_u8)
generator_filter_take(250_u8)
generator_rle(250_u8)
generator_infinite_prefix(250_u8)
generator_empty(250_u8)
