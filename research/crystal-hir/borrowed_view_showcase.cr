def local_view(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  p = raw.to_unsafe
  p[0_u16] = seed
  p[1_u16] = seed &+ 1_u8
  p[2_u16] = seed &+ 2_u8
  view = Slice.new(p, 3_u8)
  sum = 0_u8
  view.each do |value|
    sum = sum &+ value
  end
  sum.to_u16 ^ view.size.to_u16
end

def copied_view(seed : UInt8)
  a = StaticArray(UInt8, 4).new(0_u8)
  b = StaticArray(UInt8, 4).new(0_u8)
  a.to_unsafe[0_u16] = seed
  b.to_unsafe[0_u16] = seed &+ 17_u8
  view = Slice.new(a.to_unsafe, 2_u8)
  second = view
  view = Slice.new(b.to_unsafe, 1_u8)
  second.to_unsafe[1_u16] = seed &+ 1_u8
  left = 0_u8
  right = 0_u8
  second.each do |x|
    left = left &+ x
  end
  view.each do |x|
    right = right &+ x
  end
  left ^ right
end

def view_shrink(seed : UInt8)
  a = StaticArray(UInt8, 4).new(0_u8)
  b = StaticArray(UInt8, 4).new(9_u8)
  a.to_unsafe[0_u16] = seed
  view = Slice.new(a.to_unsafe, 3_u8)
  sum = 0_u8
  visits = 0_u8
  view.each do |x|
    sum = sum &+ x
    visits = visits &+ 1_u8
    if visits == 1_u8
      view = Slice.new(b.to_unsafe, 1_u8)
    end
  end
  sum ^ visits
end

def view_grow_next(seed : UInt8)
  a = StaticArray(UInt8, 4).new(0_u8)
  b = StaticArray(UInt8, 4).new(0_u8)
  a.to_unsafe[0_u16] = seed
  b.to_unsafe[1_u16] = seed &+ 7_u8
  b.to_unsafe[2_u16] = seed &+ 11_u8
  view = Slice.new(a.to_unsafe, 1_u8)
  sum = 0_u8
  visits = 0_u8
  view.each do |x|
    visits = visits &+ 1_u8
    if visits == 1_u8
      view = Slice.new(b.to_unsafe, 3_u8)
      next
    end
    sum = sum &+ x
  end
  sum ^ visits
end

def view_break(seed : UInt8)
  a = StaticArray(UInt8, 4).new(0_u8)
  b = StaticArray(UInt8, 4).new(0_u8)
  a.to_unsafe[0_u16] = seed
  view = Slice.new(a.to_unsafe, 3_u8)
  visits = 0_u8
  sum = 0_u8
  view.each do |x|
    visits = visits &+ 1_u8
    sum = sum &+ x
    view = Slice.new(b.to_unsafe, 4_u8)
    break
  end
  sum ^ visits
end

def view_word(seed : UInt16)
  raw = StaticArray(UInt16, 3).new(0_u16)
  raw.to_unsafe[0_u16] = seed
  raw.to_unsafe[1_u16] = seed &+ 1000_u16
  view = Slice.new(raw.to_unsafe, 2_u8)
  sum = 0_u16
  view.each do |word|
    sum = sum ^ word
  end
  sum ^ view.size.to_u16
end

def view_wide(seed : UInt8, length : UInt16)
  raw = StaticArray(UInt8, 256).new(1_u8)
  raw.to_unsafe[255_u16] = seed
  view = Slice.new(raw.to_unsafe, length)
  sum = 0_u16
  view.each do |value|
    sum = sum &+ value.to_u16
  end
  sum ^ view.size.to_u16
end

local_view(250_u8)
copied_view(250_u8)
view_shrink(250_u8)
view_grow_next(250_u8)
view_break(250_u8)
view_word(65535_u16)
view_wide(250_u8,256_u16)

struct ProjectionState
  @base : Pointer(UInt8)
  @hits : UInt8

  def initialize(@base : Pointer(UInt8))
    @hits = 0_u8
  end

  def data
    @hits = @hits &+ 1_u8
    @base
  end

  def length
    @hits = (@hits &* 2_u8) &+ 1_u8
    2_u8
  end

  def hits
    @hits
  end
end

def view_projection_effects(seed : UInt8)
  raw = StaticArray(UInt8, 4).new(0_u8)
  raw.to_unsafe[0_u16] = seed
  raw.to_unsafe[1_u16] = seed &+ 1_u8
  state = ProjectionState.new(raw.to_unsafe)
  length = Slice.new(state.data, state.length).size.to_u16
  pointer = Slice.new(state.data, state.length).to_unsafe
  length ^ state.hits.to_u16 ^ pointer[0_u16].to_u16 ^ pointer[1_u16].to_u16
end

view_projection_effects(250_u8)
