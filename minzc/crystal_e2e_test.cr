# MinZ → Crystal E2E Test
# This file wraps the generated Crystal code to make it compilable

# Stub for print_u8_decimal 
def print_u8_decimal
  puts "42"  # Placeholder
end

# Generated functions with clean names
def fibonacci_minz(n : UInt8) : UInt8
  if n <= 1
    return n
  end
  return fibonacci_minz((n - 1).to_u8) + fibonacci_minz((n - 2).to_u8)
end

def test_arithmetic : UInt8
  a = 10_u8
  b = 20_u8
  c = (a + b).to_u8
  d = (c * 2).to_u8
  e = (d / 3).to_u8
  return (e - 5).to_u8
end

def test_control_flow(x : UInt8) : Bool
  if x > 10
    return true
  else
    return false
  end
end

def print_number(n : UInt8)
  print "Number: "
  puts n
end

def main
  # Test arithmetic
  result = test_arithmetic
  print_number(result)
  
  # Test recursion
  fib = fibonacci_minz(5_u8)
  print_number(fib)
  
  # Test control flow
  flag = test_control_flow(15_u8)
  if flag
    puts "Test passed!"
  end
end

# Run the main function
main