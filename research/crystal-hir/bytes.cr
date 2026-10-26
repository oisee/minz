# Native Crystal facade for stdlib/mem/bytes.nanz. Only the exact methods in
# this file are admitted as library bindings by the research frontend.
module MinzBytes
  struct Ring
    def initialize(@data : Pointer(UInt8), cap : UInt16)
      @head = 0_u16
      @len = 0_u16
      @cap = cap
      nil
    end

    def size : UInt16
      @len
    end

    def clear : Nil
      @head = 0_u16
      @len = 0_u16
      nil
    end

    def try_push(value : UInt8) : UInt8
      return 0_u8 if @len >= @cap
      remaining = @cap - @head
      index = @len < remaining ? @head + @len : @len - remaining
      @data[index] = value
      @len += 1_u16
      1_u8
    end

    def try_pop(dest : Pointer(UInt8)) : UInt8
      return 0_u8 if @len == 0_u16
      dest.value = @data[@head]
      @head += 1_u16
      @head = 0_u16 if @head == @cap
      @len -= 1_u16
      1_u8
    end

    def segments(first : Pointer(View), second : Pointer(View)) : Nil
      start = 0_u16
      count = 0_u16
      if @len != 0_u16
        start = @head
        count = @cap - @head
        count = @len if count > @len
      end
      first.value = View.new(@data + start, count)
      second.value = View.new(@data, @len - count)
      nil
    end
  end

  struct View
    def initialize(@data : Pointer(UInt8), @len : UInt16)
      nil
    end

    def size : UInt16
      @len
    end

    def to_unsafe : Pointer(UInt8)
      @data
    end

    def try_get(index : UInt16, dest : Pointer(UInt8)) : UInt8
      return 0_u8 if index >= @len
      dest.value = @data[index]
      1_u8
    end

    def try_set(index : UInt16, value : UInt8) : UInt8
      return 0_u8 if index >= @len
      @data[index] = value
      1_u8
    end

    def subview(start : UInt16, count : UInt16, dest : Pointer(View)) : UInt8
      return 0_u8 if start > @len
      return 0_u8 if count > @len - start
      data = @data
      dest.value = View.new(data + start, count)
      1_u8
    end
  end

  struct Buffer
    def initialize(@data : Pointer(UInt8), cap : UInt16)
      @len = 0_u16
      @cap = cap
      nil
    end

    def size : UInt16
      @len
    end

    def clear : Nil
      @len = 0_u16
      nil
    end

    def try_push(value : UInt8) : UInt8
      return 0_u8 if @len >= @cap
      @data[@len] = value
      @len += 1_u16
      1_u8
    end

    def view(dest : Pointer(View)) : Nil
      dest.value = View.new(@data, @len)
      nil
    end
  end
end
