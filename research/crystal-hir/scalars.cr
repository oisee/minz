def add_byte(a : UInt8, b : UInt8) : UInt8
  a &+ b
end

def sum_to(n : UInt8) : UInt8
  acc = 0_u8
  while n > 0_u8
    acc = add_byte(acc, n)
    n = n &- 1_u8
  end
  acc
end

def abs_diff(a : UInt8, b : UInt8) : UInt8
  if a > b
    a &- b
  else
    b &- a
  end
end

def add_word(a : UInt16, b : UInt16) : UInt16
  a &+ b
end

def negative(a : Int8) : Bool
  a < 0_i8
end

# Deliberate forward call: signature stubs must preserve UInt16 return type.
def forward_word(a : UInt16) : UInt16
  later_word(a)
end

def later_word(a : UInt16) : UInt16
  a &+ 300_u16
end
