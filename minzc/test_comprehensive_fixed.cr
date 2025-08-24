# Generated Crystal code from MinZ compiler v0.15.0
# Ruby-style interpolation maps perfectly to Crystal syntax!

def __test_crystal_comprehensive_fibonacci_u8(n : UInt8) : UInt8
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  r3 = 1
  # TODO: Unhandled instruction: LE
  if !r4
    # goto else_1
  end
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  return r5
  # label else_1:
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  # TODO: Unhandled instruction: PATCH_TEMPLATE
  # TODO: Unhandled instruction: PATCH_TARGET
  # TODO: Unhandled instruction: PATCH_PARAM
  r12 = __test_crystal_comprehensive_fibonacci_u8()
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  # TODO: Unhandled instruction: PATCH_TEMPLATE
  # TODO: Unhandled instruction: PATCH_TARGET
  # TODO: Unhandled instruction: PATCH_PARAM
  r19 = __test_crystal_comprehensive_fibonacci_u8()
  r20 = r12 + r19
  return r20
end

def __test_crystal_comprehensive_test_arithmetic : UInt8
  a = uninitialized UInt8
  b = uninitialized UInt8
  c = uninitialized UInt16
  d = uninitialized UInt16
  e = uninitialized UInt16
  r17 = e
  r18 = 5
  r19 = r17 - r18
  return r19
end

def __test_crystal_comprehensive_test_control_flow_u8(x : UInt8) : Bool
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  r3 = 10
  # TODO: Unhandled instruction: GT
  if !r4
    # goto else_3
  end
  r5 = 1
  return r5
  # label else_3:
  r6 = 0
  return r6
end

def __test_crystal_comprehensive_print_number_u8(n : UInt8) : Nil
  # TODO: Unhandled instruction: PRINT_STRING_DIRECT
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  print_u8_decimal()
  # TODO: Unhandled instruction: PRINT_STRING_DIRECT
  return
end

def __test_crystal_comprehensive_main : Nil
  result = uninitialized UInt8
  fib = uninitialized UInt8
  flag = uninitialized Bool
  # TODO: Unhandled instruction: PATCH_TEMPLATE
  # TODO: Unhandled instruction: PATCH_TARGET
  # TODO: Unhandled instruction: PRINT_STRING_DIRECT
  # TODO: Unhandled instruction: LOAD_PARAM
  print_u8_decimal()
  # TODO: Unhandled instruction: PRINT_STRING_DIRECT
  # TODO: Unhandled instruction: PATCH_TEMPLATE
  # TODO: Unhandled instruction: PATCH_TARGET
  # TODO: Unhandled instruction: PATCH_PARAM
  r9 = __test_crystal_comprehensive_fibonacci_u8()
  # TODO: Unhandled instruction: PRINT_STRING_DIRECT
  # TODO: Unhandled instruction: LOAD_PARAM
  print_u8_decimal()
  # TODO: Unhandled instruction: PRINT_STRING_DIRECT
  # TODO: Unhandled instruction: TRUE_SMC_LOAD
  r4 = 10
  # TODO: Unhandled instruction: GT
  if !r5
    # goto else_3
  end
  # TODO: Unhandled instruction: MOVE
  # label else_3:
  # TODO: Unhandled instruction: MOVE
  r17 = flag
  if !r17
    # goto else_5
  end
  # TODO: Unhandled instruction: LOAD_STRING
  print "string_0"
  # goto end_if_6
  # label else_5:
  # label end_if_6:
  return
end

