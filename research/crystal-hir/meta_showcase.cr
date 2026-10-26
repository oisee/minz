# Native reference implementation: its body is deliberately NOT imported into
# target HIR. The typed exporter recognizes this explicit opt-in marker.
def minz_meta_repeat(value : UInt8, count : UInt8, delta : UInt8) : UInt8
  i = 0_u8
  while i < count
    value = value &+ delta
    i = i &+ 1_u8
  end
  value
end

macro meta_repeat(value, count, delta)
  minz_meta_repeat({{value}}, {{count}}, {{delta}})
end

def no_iterations(x : UInt8)
  meta_repeat(x, 0_u8, 255_u8)
end

def one_iteration(x : UInt8)
  meta_repeat(x, 1_u8, 7_u8)
end

def three_iterations(x : UInt8)
  meta_repeat(x, 3_u8, 7_u8)
end

def palette_stride(x : UInt8)
  meta_repeat(x, 8_u8, 32_u8)
end

def thirty_two_iterations(x : UInt8)
  meta_repeat(x, 32_u8, 255_u8)
end

def repeated_specialization(x : UInt8)
  first = meta_repeat(x, 3_u8, 7_u8)
  meta_repeat(first, 3_u8, 7_u8)
end

def caller_hygiene(meta_result : UInt8)
  generated = meta_repeat(meta_result, 3_u8, 7_u8)
  generated ^ meta_result
end

no_iterations(250_u8)
one_iteration(250_u8)
three_iterations(250_u8)
palette_stride(250_u8)
thirty_two_iterations(250_u8)
repeated_specialization(250_u8)
caller_hygiene(250_u8)
