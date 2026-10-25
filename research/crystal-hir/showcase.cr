# The same untyped helper is specialized by native Crystal for UInt8/UInt16.
def twice(x)
  x &+ x
end

def identity(x)
  x
end

def identity_twin(x)
  x
end

def flavor(x : UInt8)
  x &+ 3_u8
end

def flavor(x : UInt16)
  x &+ 300_u16
end

def byte_demo(x : UInt8)
  twice(flavor(identity(x)))
end

def word_demo(x : UInt16)
  twice(flavor(x))
end

def twin_demo(x : UInt8)
  identity_twin(x)
end

macro bias
  7_u8
end

def macro_demo(x : UInt8)
  x &+ bias
end

# A 0..254 triangle wave: a tiny palette / animation building block.
def triangle(phase : UInt8)
  if phase < 128_u8
    phase &* 2_u8
  else
    (255_u8 &- phase) &* 2_u8
  end
end

# RGB888 -> an RGB332 packed palette index, no allocation.
def rgb332(r : UInt8, g : UInt8, b : UInt8)
  (r & 224_u8) | ((g >> 3_u8) & 28_u8) | (b >> 6_u8)
end

# CRC-8/ATM, polynomial 0x07. This is a real byte-at-a-time kernel.
def crc8_step(crc : UInt8, byte : UInt8)
  crc = crc ^ byte
  i = 0_u8
  while i < 8_u8
    if (crc & 128_u8) != 0_u8
      crc = (crc << 1_u8) ^ 7_u8
    else
      crc = crc << 1_u8
    end
    i = i &+ 1_u8
  end
  crc
end

def hex_nibble(c : UInt8)
  if c >= 48_u8
    if c <= 57_u8
      return c &- 48_u8
    end
  end
  if c >= 65_u8
    if c <= 70_u8
      return (c &- 65_u8) &+ 10_u8
    end
  end
  raise "bad hex digit"
end

def hex_byte(hi : UInt8, lo : UInt8)
  (hex_nibble(hi) &* 16_u8) | hex_nibble(lo)
end

# Rescue is lowered to a local error branch, with no stack unwinder.
def hex_or_marker(hi : UInt8, lo : UInt8)
  hex_byte(hi, lo)
rescue
  238_u8
end

# Arena reservation in a 32-byte region: offsets are explicit; no GC/object
# allocation. A real memory-owning Arena struct is a subsequent layout task.
def reserve(cursor : UInt8, size : UInt8, capacity : UInt8)
  if cursor > capacity
    raise "invalid arena cursor"
  end
  if size > (capacity &- cursor)
    raise "arena exhausted"
  end
  cursor &+ size
end

def arena_frame(first : UInt8, second : UInt8)
  cursor = reserve(0_u8, first, 32_u8)
  cursor = reserve(cursor, second, 32_u8)
  cursor
rescue
  255_u8
end

def arena_reset_demo(size : UInt8)
  # A new frame starts again at offset zero.
  previous_frame = arena_frame(16_u8, 16_u8)
  next_frame = arena_frame(size, 0_u8)
  next_frame
end

# Reachability roots for native inference. The exporter does not execute them.
byte_demo(10_u8)
word_demo(1000_u16)
twin_demo(10_u8)
macro_demo(10_u8)
triangle(0_u8)
rgb332(255_u8, 128_u8, 64_u8)
crc8_step(0_u8, 49_u8)
hex_or_marker(65_u8, 70_u8)
arena_frame(16_u8, 16_u8)
arena_reset_demo(4_u8)
