struct ByteArena
  @tag : UInt8
  @base : Pointer(UInt8)
  @cursor : UInt16
  @capacity : UInt16

  def initialize(@base : Pointer(UInt8), @capacity : UInt16)
    @tag = 90_u8
    @cursor = 0_u16
  end

  def alloc(size : UInt16) : Pointer(UInt8)
    if size > (@capacity &- @cursor)
      return Pointer(UInt8).null
    end
    result = @base + @cursor
    @cursor = @cursor &+ size
    result
  end

  def reset
    @cursor = 0_u16
  end

  def used
    @cursor
  end

  def tag
    @tag
  end

  def allocate_echo(value : UInt8)
    p = alloc(1_u16)
    p[0_u16] = value &+ 1_u8
    value
  end

  def overwrite(p : Pointer(UInt8), value : UInt8)
    p[0_u16] = value
    value
  end
end

def arena_memory(seed : UInt8)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  arena = ByteArena.new(raw + 2_u16, 28_u16)
  p = arena.alloc(2_u16)
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  if guards_ok(raw) == false
    return 60001_u16
  end
  if arena.tag != 90_u8
    return 60002_u16
  end
  (p[0_u16] ^ p[1_u16]).to_u16
end

def set_guards(p : Pointer(UInt8))
  p[0_u16] = 165_u8
  p[1_u16] = 90_u8
  p[30_u16] = 195_u8
  p[31_u16] = 60_u8
end

def guards_ok(p : Pointer(UInt8))
  if p[0_u16] != 165_u8
    return false
  end
  if p[1_u16] != 90_u8
    return false
  end
  if p[30_u16] != 195_u8
    return false
  end
  if p[31_u16] != 60_u8
    return false
  end
  true
end

def arena_try(size : UInt16, seed : UInt8)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  arena = ByteArena.new(raw + 2_u16, 28_u16)
  p = arena.alloc(size)
  if p.null?
    if arena.used != 0_u16
      return 60003_u16
    end
    return 65535_u16
  end
  if size > 0_u16
    p[0_u16] = seed
    p[size &- 1_u16] = seed &+ 1_u8
  end
  if guards_ok(raw) == false
    return 60004_u16
  end
  arena.used
end

def arena_reset(seed : UInt8)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  arena = ByteArena.new(raw + 2_u16, 28_u16)
  old = arena.alloc(2_u16)
  old[0_u16] = seed
  old[1_u16] = seed &+ 1_u8
  arena.reset
  fresh = arena.alloc(2_u16)
  if fresh != old
    return 60005_u16
  end
  fresh[0_u16] = seed &+ 7_u8
  if old[0_u16] != (seed &+ 7_u8)
    return 60006_u16
  end
  if guards_ok(raw) == false
    return 60007_u16
  end
  arena.used
end

def arena_copy(seed : UInt8)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  first = ByteArena.new(raw + 2_u16, 28_u16)
  a = first.alloc(2_u16)
  a[0_u16] = seed
  second = first
  b = second.alloc(3_u16)
  b[0_u16] = seed &+ 1_u8
  if first.used != 2_u16
    return 60008_u16
  end
  if second.used != 5_u16
    return 60009_u16
  end
  if a[0_u16] != seed
    return 60010_u16
  end
  if b[0_u16] != (seed &+ 1_u8)
    return 60011_u16
  end
  if guards_ok(raw) == false
    return 60012_u16
  end
  0_u16
end

def word_pointer(seed : UInt16)
  buffer = StaticArray(UInt16, 8).new(0_u16)
  p = buffer.to_unsafe
  p[0_u16] = seed
  q = p + 1_u16
  q[0_u16] = seed &+ 1000_u16
  if p[7_u16] != 0_u16
    return 60013_u16
  end
  p[0_u16] ^ p[1_u16]
end

def zero_then_write(seed : UInt8)
  buffer = StaticArray(UInt8, 4).new(0_u8)
  p = buffer.to_unsafe
  initial = p[0_u16]
  p[0_u16] = seed
  initial
end

def repeated_storage(seed : UInt8)
  first = zero_then_write(seed)
  second = zero_then_write(seed &+ 1_u8)
  first | second
end

def evaluation_order(seed : UInt8)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  arena = ByteArena.new(raw + 2_u16, 28_u16)
  # LHS alloc must happen before RHS alloc. The two stores differ.
  arena.alloc(1_u16)[0_u16] = arena.allocate_echo(seed)
  if raw[2_u16] != seed
    return 60014_u16
  end
  if raw[3_u16] != (seed &+ 1_u8)
    return 60015_u16
  end
  # The first load must be captured before overwrite mutates that byte.
  sum = raw[2_u16] &+ arena.overwrite(raw + 2_u16, seed &+ 2_u8)
  if guards_ok(raw) == false
    return 60016_u16
  end
  sum.to_u16
end

def integer_truthiness(value : UInt8)
  if value
    1_u8
  else
    0_u8
  end
end

def pointer_truthiness(seed : UInt8)
  if Pointer(UInt8).null
    seed
  else
    seed &+ 1_u8
  end
end

def condition_effect(seed : UInt8)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  arena = ByteArena.new(raw + 2_u16, 28_u16)
  if arena.overwrite(raw + 2_u16, seed)
    if guards_ok(raw) == false
      return 60018_u16
    end
    raw[2_u16].to_u16
  else
    60017_u16
  end
end

def allocation_truthiness(size : UInt16)
  buffer = StaticArray(UInt8, 32).new(0_u8)
  raw = buffer.to_unsafe
  set_guards(raw)
  arena = ByteArena.new(raw + 2_u16, 28_u16)
  if arena.alloc(size)
    if guards_ok(raw) == false
      return 60019_u16
    end
    arena.used
  else
    if arena.used != 0_u16
      return 60020_u16
    end
    65535_u16
  end
end

arena_memory(5_u8)
arena_try(28_u16, 5_u8)
arena_reset(5_u8)
arena_copy(5_u8)
word_pointer(1234_u16)
repeated_storage(5_u8)
evaluation_order(5_u8)
integer_truthiness(0_u8)
pointer_truthiness(5_u8)
condition_effect(5_u8)
allocation_truthiness(28_u16)
