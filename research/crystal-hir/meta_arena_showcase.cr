# Explicit opt-in bridge markers. Their bodies are native reference behavior;
# the record importer replaces each marker with Nanz-generated target HIR.
def minz_meta_repeat(value : UInt8, count : UInt8, delta : UInt8) : UInt8
  i = 0_u8
  while i < count
    value = value &+ delta
    i = i &+ 1_u8
  end
  value
end

def minz_meta_layout_byte(value : Pointer(T), field : String) : UInt8 forall T
  value.value.marker
end

macro meta_repeat(value, count, delta)
  minz_meta_repeat({{value}}, {{count}}, {{delta}})
end

macro meta_byte(record, field)
  minz_meta_layout_byte(pointerof({{record}}), {{field}})
end

struct MetaArena
  @tag : UInt8
  @base : Pointer(UInt8)
  @cursor : UInt16
  @capacity : UInt16
  @marker : UInt8

  def initialize(@base : Pointer(UInt8), @capacity : UInt16)
    @tag = 90_u8
    @cursor = 0_u16
    @marker = 0_u8
  end

  def alloc(size : UInt16) : Pointer(UInt8)
    if size > (@capacity &- @cursor)
      return Pointer(UInt8).null
    end
    result = @base + @cursor
    @cursor = @cursor &+ size
    result
  end

  def marker
    @marker
  end

  def set_marker(value : UInt8)
    @marker = value
  end
end

struct HeaderProbe
  def initialize(@marker : UInt8, @word : UInt16)
  end

  def marker
    @marker
  end
end

def meta_arena_memory(seed : UInt8)
  storage = StaticArray(UInt8, 16).new(0_u8)
  arena = MetaArena.new(storage.to_unsafe, 16_u16)
  p = arena.alloc(3_u16)
  p[0_u16] = meta_repeat(seed, 3_u8, 7_u8)
  arena.set_marker(p[0_u16])
  meta_byte(arena, "marker") ^ seed
end

def meta_arena_copy(seed : UInt8)
  storage = StaticArray(UInt8, 16).new(0_u8)
  arena = MetaArena.new(storage.to_unsafe, 16_u16)
  arena.set_marker(seed)
  second = arena
  arena.set_marker(seed &+ 1_u8)
  old = meta_byte(second, "marker")
  fresh = meta_byte(arena, "marker")
  old ^ fresh
end

def meta_two_layouts(seed : UInt8)
  storage = StaticArray(UInt8, 16).new(0_u8)
  arena = MetaArena.new(storage.to_unsafe, 16_u16)
  arena.set_marker(seed)
  header = HeaderProbe.new(seed &+ 17_u8, 23130_u16)
  meta_byte(arena, "marker") ^ meta_byte(header, "marker")
end

meta_arena_memory(250_u8)
meta_arena_copy(250_u8)
meta_two_layouts(250_u8)
