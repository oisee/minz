def twice(x)
  x &+ x
end

def flavor(x : UInt8)
  x &+ 3_u8
end

def flavor(x : UInt16)
  x &+ 300_u16
end

def byte_demo(x : UInt8)
  twice(flavor(x))
end

def word_demo(x : UInt16)
  twice(flavor(x))
end

byte_demo(10_u8)
word_demo(1000_u16)
