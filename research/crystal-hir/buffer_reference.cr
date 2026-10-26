# Native semantic reference for mem.bytes workloads. Uses stock Crystal Slice
# for descriptors and bounds, not the target implementation under test.
def check_buffer(seed : UInt8, capacity : UInt16)
  storage = StaticArray(UInt8, 258).new(197_u8)
  data = storage.to_unsafe + 1
  length = 0
  capacity.times do |i|
    data[length] = seed &+ i.to_u8!
    length += 1
  end
  # A bounded append rejects another byte when full, including capacity zero.
  if length < capacity
    data[length] = 99_u8
    raise "unexpected capacity"
  end
  saved = Slice.new(data, length)
  258.times do |i|
    expected = i > 0 && i <= capacity ? seed &+ (i - 1).to_u8! : 197_u8
    raise "backing byte #{seed}/#{capacity}/#{i}" unless storage[i] == expected
  end
  raise "read bounds" unless saved[capacity]?.nil?
  before = storage.dup
  begin
    saved[capacity] = 99_u8
    raise "write bounds admitted"
  rescue IndexError
  end
  raise "failed write changed storage" unless storage == before
  # Crystal's API raises, whereas our bounded API returns zero and keeps out.
  piece = Slice.new(storage.to_unsafe, 7)
  [{65535, 2}, {capacity.to_i, 1}].each do |start, count|
    begin
      piece = saved[start, count]
      raise "subview bounds admitted"
    rescue IndexError
    end
    raise "failed subview changed descriptor" unless piece.size == 7 && piece.to_unsafe == storage.to_unsafe
  end
  piece = saved[capacity.to_i, 0]
  raise "empty end view" unless piece.empty? && piece.to_unsafe == data + capacity
  if capacity > 0
    piece = saved[0, capacity.to_i]
    piece = piece[capacity.to_i - 1, 1]
    raise "last byte" unless piece[0] == seed &+ (capacity - 1).to_u8!
    piece[0] = 42_u8
    raise "alias write" unless storage[capacity] == 42_u8
  end
  length = 0
  raise "saved size" unless saved.size == capacity
  if capacity > 0
    data[length] = 91_u8
    length += 1
    raise "shared backing" unless saved[0] == 91_u8
  end
  raise "guards" unless storage[0] == 197 && storage[257] == 197
end

{0_u16, 1_u16, 3_u16, 255_u16, 256_u16}.each do |capacity|
  {0_u8, 1_u8, 127_u8, 255_u8}.each do |seed|
    check_buffer(seed, capacity)
  end
end

storage = StaticArray(UInt8, 1).new(0_u8)
huge = Slice.new(storage.to_unsafe, 65535)
begin
  huge[65534, 3]
  raise "overflowing subrange admitted"
rescue IndexError
end
empty = Slice.new(Pointer(UInt8).null, 0)
raise "empty null read" unless empty[0]?.nil?
begin
  empty[0] = 42_u8
  raise "empty null write admitted"
rescue IndexError
end
end_view = empty[0, 0]
raise "empty null subview" unless end_view.empty? && end_view.to_unsafe.null?
puts "20 native Crystal buffer/view reference workloads + 2 boundary scenarios passed"
