require "./bytes"

def library_buffer(seed : UInt8, capacity : UInt16) : UInt16
  storage = StaticArray(UInt8, 256).new(197_u8)
  data = storage.to_unsafe
  buffer = MinzBytes::Buffer.new(data + 1_u16, capacity)
  saved = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  piece = MinzBytes::View.new(data, 7_u16)
  byte = 197_u8
  i = 0_u16
  while i < capacity
    return 1_u16 unless buffer.try_push(seed &+ i.to_u8!) == 1_u8
    i = i &+ 1_u16
  end
  return 2_u16 unless buffer.try_push(99_u8) == 0_u8
  return 3_u16 unless buffer.size == capacity
  buffer.view(pointerof(saved))
  return 4_u16 unless saved.size == capacity
  i = 0_u16
  while i < 256_u16
    expected = 197_u8
    if i > 0_u16
      if i <= capacity
        expected = seed &+ (i &- 1_u16).to_u8!
      end
    end
    return 5_u16 unless data[i] == expected
    i = i &+ 1_u16
  end
  return 6_u16 unless saved.try_get(capacity, pointerof(byte)) == 0_u8
  return 7_u16 unless byte == 197_u8
  return 8_u16 unless saved.try_set(capacity, 99_u8) == 0_u8
  return 9_u16 unless data[capacity &+ 1_u16] == 197_u8
  return 10_u16 unless saved.subview(65535_u16, 2_u16, pointerof(piece)) == 0_u8
  return 11_u16 unless piece.size == 7_u16
  return 12_u16 unless piece.to_unsafe == data
  return 13_u16 unless saved.subview(capacity, 1_u16, pointerof(piece)) == 0_u8
  return 14_u16 unless piece.size == 7_u16
  return 15_u16 unless saved.subview(capacity, 0_u16, pointerof(piece)) == 1_u8
  return 16_u16 unless piece.size == 0_u16
  return 17_u16 unless piece.to_unsafe == data + 1_u16 + capacity
  if capacity > 0_u16
    return 18_u16 unless saved.subview(0_u16, capacity, pointerof(piece)) == 1_u8
    return 19_u16 unless piece.subview(capacity &- 1_u16, 1_u16, pointerof(piece)) == 1_u8
    return 20_u16 unless piece.try_get(0_u16, pointerof(byte)) == 1_u8
    return 21_u16 unless byte == seed &+ (capacity &- 1_u16).to_u8!
    return 22_u16 unless piece.try_set(0_u16, 42_u8) == 1_u8
    return 23_u16 unless data[capacity] == 42_u8
  end
  buffer.clear
  return 24_u16 unless buffer.size == 0_u16
  return 25_u16 unless saved.size == capacity
  if capacity > 0_u16
    return 26_u16 unless buffer.try_push(91_u8) == 1_u8
    return 27_u16 unless saved.try_get(0_u16, pointerof(byte)) == 1_u8
    return 28_u16 unless byte == 91_u8
  end
  return 29_u16 unless data[0_u16] == 197_u8
  return 30_u16 unless data[255_u16] == 197_u8
  0_u16
end

def library_wide(seed : UInt8) : UInt16
  storage = StaticArray(UInt8, 256).new(0_u8)
  buffer = MinzBytes::Buffer.new(storage.to_unsafe, 256_u16)
  i = 0_u16
  while i < 256_u16
    return 1_u16 unless buffer.try_push(seed &+ i.to_u8!) == 1_u8
    i = i &+ 1_u16
  end
  return 2_u16 unless buffer.size == 256_u16
  return 3_u16 unless buffer.try_push(91_u8) == 0_u8
  i = 0_u16
  while i < 256_u16
    return 4_u16 unless storage.to_unsafe[i] == seed &+ i.to_u8!
    i = i &+ 1_u16
  end
  buffer.size
end

def library_empty : UInt16
  null = Pointer(UInt8).null
  view = MinzBytes::View.new(null, 0_u16)
  dest = MinzBytes::View.new(null, 7_u16)
  buffer = MinzBytes::Buffer.new(null, 0_u16)
  byte = 197_u8
  return 1_u16 unless view.try_get(0_u16, pointerof(byte)) == 0_u8
  return 2_u16 unless byte == 197_u8
  return 3_u16 unless view.try_set(0_u16, 42_u8) == 0_u8
  return 4_u16 unless buffer.try_push(42_u8) == 0_u8
  return 5_u16 unless view.subview(0_u16, 0_u16, pointerof(dest)) == 1_u8
  return 6_u16 unless dest.to_unsafe.null?
  return 7_u16 unless dest.size == 0_u16
  0_u16
end

def library_overflow : UInt16
  storage = StaticArray(UInt8, 1).new(0_u8)
  view = MinzBytes::View.new(storage.to_unsafe, 65535_u16)
  dest = MinzBytes::View.new(storage.to_unsafe, 7_u16)
  return 1_u16 unless view.subview(65534_u16, 3_u16, pointerof(dest)) == 0_u8
  return 2_u16 unless dest.size == 7_u16
  0_u16
end

def byte_effect(trace : Pointer(UInt8), tag : UInt8, value : UInt8) : UInt8
  trace[0_u16] = (trace[0_u16] &* 10_u8) &+ tag
  value
end

def index_effect(trace : Pointer(UInt8), tag : UInt8, value : UInt16) : UInt16
  trace[0_u16] = (trace[0_u16] &* 10_u8) &+ tag
  value
end

def library_effects(seed : UInt8) : UInt16
  storage = StaticArray(UInt8, 1).new(197_u8)
  buffer = MinzBytes::Buffer.new(storage.to_unsafe, 0_u16)
  view = MinzBytes::View.new(storage.to_unsafe, 0_u16)
  trace = 0_u8
  return 300_u16 unless buffer.try_push(byte_effect(pointerof(trace), 1_u8, seed)) == 0_u8
  return 301_u16 unless view.try_set(index_effect(pointerof(trace), 2_u8, 0_u16), byte_effect(pointerof(trace), 3_u8, seed)) == 0_u8
  return 302_u16 unless storage.to_unsafe[0_u16] == 197_u8
  trace.to_u16
end

def library_iteration(seed : UInt8) : UInt16
  storage = StaticArray(UInt8, 3).new(0_u8)
  buffer = MinzBytes::Buffer.new(storage.to_unsafe, 3_u16)
  view = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  buffer.try_push(seed)
  buffer.try_push(seed &+ 1_u8)
  buffer.try_push(seed &+ 2_u8)
  buffer.view(pointerof(view))
  sum = 0_u16
  steps = 0_u8
  Slice.new(view.to_unsafe, view.size).each do |value|
    sum = sum &+ value.to_u16
    steps = steps &+ 1_u8
    buffer.clear
    buffer.try_push(91_u8)
    break if steps == 2_u8
  end
  sum
end

def library_values(data : Pointer(UInt8), length : UInt16)
  index = 0_u16
  while index < length
    yield data[index]
    index = index &+ 1_u16
  end
  nil
end

def library_pull(seed : UInt8) : UInt16
  storage = StaticArray(UInt8, 3).new(0_u8)
  buffer = MinzBytes::Buffer.new(storage.to_unsafe, 3_u16)
  view = MinzBytes::View.new(Pointer(UInt8).null, 0_u16)
  buffer.try_push(seed)
  buffer.try_push(seed &+ 1_u8)
  buffer.view(pointerof(view))
  result = 0_u16
  library_values(view.to_unsafe, view.size) do |value|
    result = value.to_u16
    break
  end
  result
end

library_buffer(0_u8, 3_u16)
library_wide(0_u8)
library_empty
library_overflow
library_effects(0_u8)
library_iteration(0_u8)
library_pull(0_u8)
