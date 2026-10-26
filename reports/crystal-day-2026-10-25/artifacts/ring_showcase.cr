require "./bytes"

def ring_trace(seed : UInt8, capacity : UInt16) : UInt16
  storage = StaticArray(UInt8, 256).new(197_u8)
  data = storage.to_unsafe
  ring = MinzBytes::Ring.new(data + 1_u16, capacity)
  first = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  second = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  byte = 197_u8
  return 1_u16 unless ring.try_pop(pointerof(byte)) == 0_u8
  return 2_u16 unless byte == 197_u8
  i = 0_u16
  while i < capacity
    return 3_u16 unless ring.try_push(seed &+ i.to_u8!) == 1_u8
    i = i &+ 1_u16
  end
  return 4_u16 unless ring.try_push(99_u8) == 0_u8
  return 5_u16 unless ring.size == capacity
  i = 0_u16
  while i < (capacity &* 2_u16) &+ 1_u16
    return 6_u16 unless ring.try_pop(pointerof(byte)) == 1_u8
    return 7_u16 unless byte == seed &+ i.to_u8!
    return 8_u16 unless ring.try_push(seed &+ (capacity &+ i).to_u8!) == 1_u8
    i = i &+ 1_u16
  end
  ring.segments(pointerof(first), pointerof(second))
  return 9_u16 unless first.size &+ second.size == capacity
  if capacity == 1_u16
    return 10_u16 unless first.size == 1_u16
    return 11_u16 unless second.size == 0_u16
  else
    return 12_u16 unless first.size == capacity &- 1_u16
    return 13_u16 unless second.size == 1_u16
  end
  at = (capacity &* 2_u16) &+ 1_u16
  Slice.new(first.to_unsafe, first.size).each do |value|
    return 14_u16 unless value == seed &+ at.to_u8!
    at = at &+ 1_u16
  end
  Slice.new(second.to_unsafe, second.size).each do |value|
    return 15_u16 unless value == seed &+ at.to_u8!
    at = at &+ 1_u16
  end
  # Views share bytes, so a write through the first readable range changes pop.
  return 16_u16 unless first.try_set(0_u16, 42_u8) == 1_u8
  return 17_u16 unless ring.try_pop(pointerof(byte)) == 1_u8
  return 18_u16 unless byte == 42_u8
  i = capacity &+ 1_u16
  while i < 256_u16
    return 19_u16 unless data[i] == 197_u8
    i = i &+ 1_u16
  end
  return 20_u16 unless data[0_u16] == 197_u8
  ring.clear
  return 21_u16 unless ring.size == 0_u16
  ring.try_push(seed)
  ring.segments(pointerof(first), pointerof(second))
  ring.clear
  ring.try_push(91_u8)
  return 22_u16 unless first.size == 1_u16
  return 23_u16 unless first.try_get(0_u16, pointerof(byte)) == 1_u8
  return 24_u16 unless byte == 91_u8
  0_u16
end

def ring_wide(seed : UInt8) : UInt16
  storage = StaticArray(UInt8, 256).new(0_u8)
  ring = MinzBytes::Ring.new(storage.to_unsafe, 256_u16)
  first = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  second = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  byte = 197_u8
  i = 0_u16
  while i < 256_u16
    return 1_u16 unless ring.try_push(seed &+ i.to_u8!) == 1_u8
    i = i &+ 1_u16
  end
  return 2_u16 unless ring.size == 256_u16
  return 3_u16 unless ring.try_push(99_u8) == 0_u8
  return 4_u16 unless ring.try_pop(pointerof(byte)) == 1_u8
  return 5_u16 unless byte == seed
  return 6_u16 unless ring.try_push(seed) == 1_u8
  ring.segments(pointerof(first), pointerof(second))
  return 7_u16 unless first.size == 255_u16
  return 8_u16 unless second.size == 1_u16
  i = 0_u16
  while i < 256_u16
    return 9_u16 unless ring.try_pop(pointerof(byte)) == 1_u8
    return 10_u16 unless byte == seed &+ (i &+ 1_u16).to_u8!
    i = i &+ 1_u16
  end
  byte = 197_u8
  return 11_u16 unless ring.try_pop(pointerof(byte)) == 0_u8
  return 12_u16 unless byte == 197_u8
  256_u16
end

def ring_null : UInt16
  null = Pointer(UInt8).null
  ring = MinzBytes::Ring.new(null, 0_u16)
  first = MinzBytes::View.new(null, 7_u16)
  second = MinzBytes::View.new(null, 9_u16)
  byte = 197_u8
  return 1_u16 unless ring.try_pop(pointerof(byte)) == 0_u8
  return 2_u16 unless byte == 197_u8
  return 3_u16 unless ring.try_push(42_u8) == 0_u8
  ring.segments(pointerof(first), pointerof(second))
  return 4_u16 unless first.size == 0_u16
  return 5_u16 unless second.size == 0_u16
  return 6_u16 unless first.to_unsafe.null?
  return 7_u16 unless second.to_unsafe.null?
  ring.clear
  return 8_u16 unless ring.size == 0_u16
  0_u16
end

def ring_values(first : Pointer(UInt8), count : UInt16, second : Pointer(UInt8), rest : UInt16, reads : Pointer(UInt8))
  i = 0_u16
  while i < count
    reads[0_u16] = reads[0_u16] &+ 1_u8
    yield first[i]
    i = i &+ 1_u16
  end
  i = 0_u16
  while i < rest
    reads[0_u16] = reads[0_u16] &+ 1_u8
    yield second[i]
    i = i &+ 1_u16
  end
  nil
end

def ring_early(seed : UInt8) : UInt16
  storage = StaticArray(UInt8, 3).new(0_u8)
  ring = MinzBytes::Ring.new(storage.to_unsafe, 3_u16)
  first = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  second = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  byte = 0_u8
  ring.try_push(seed)
  ring.try_push(seed &+ 1_u8)
  ring.try_push(seed &+ 2_u8)
  ring.try_pop(pointerof(byte))
  ring.try_push(seed &+ 3_u8)
  ring.segments(pointerof(first), pointerof(second))
  reads = 0_u8
  sum = 0_u16
  count = 0_u8
  ring_values(first.to_unsafe, first.size, second.to_unsafe, second.size, pointerof(reads)) do |value|
    sum = sum &+ value.to_u16
    count = count &+ 1_u8
    break if count == 2_u8
  end
  sum &+ (reads.to_u16 &* 512_u16)
end

ring_trace(0_u8, 3_u16)
ring_wide(0_u8)
ring_null
ring_early(0_u8)
