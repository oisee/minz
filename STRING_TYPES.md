# MinZ String Types

MinZ provides three string types for different use cases:

## Native MinZ String Types

### `String` - Pascal-style string (recommended)
- **Structure**: Length byte (u8) + string data
- **Max length**: 255 characters
- **Memory layout**: `[length][char1][char2]...[charN]`
- **Use case**: General string handling in MinZ programs
- **Example**:
```minz
let name: String = "Alice";  // Stored as: [5, 'A', 'l', 'i', 'c', 'e']
```

### `LString` - Long Pascal-style string
- **Structure**: Length word (u16) + string data  
- **Max length**: 65,535 characters
- **Memory layout**: `[length_lo][length_hi][char1][char2]...[charN]`
- **Use case**: Large text buffers, file contents
- **Example**:
```minz
let document: LString = load_file("readme.txt");
```

## C Interoperability Type

### `cstr` - C-style string pointer
- **Type**: Alias for `*u8` (pointer to u8)
- **Structure**: Pointer to null-terminated character array
- **Memory layout**: `[char1][char2]...[charN][0]`
- **Use case**: 
  - Interfacing with ROM routines expecting C-style strings
  - Interoperability with C libraries
  - Lower memory overhead when length tracking isn't needed
- **Example**:
```minz
// For C interop or ROM calls
let rom_msg: cstr = "HELLO\0";  // Explicitly C-style
rom_print_routine(rom_msg);     // ROM expects null-terminated string
```

## Recommendations

1. **Use `String` for most MinZ code** - It's safer and more idiomatic
2. **Use `LString` for large text** - When you need more than 255 characters
3. **Use `cstr` only for C/ROM interop** - When interfacing with external code that expects C-style strings

## String Literals

String literals in MinZ are flexible and adapt to their context:
```minz
let s1: String = "Hello";  // Pascal-style with length prefix
let s2: cstr = "Hello";    // C-style pointer to string data
```

The compiler handles the conversion automatically based on the target type.