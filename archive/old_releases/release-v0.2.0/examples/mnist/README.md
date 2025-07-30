# MNIST Digit Editor for ZX Spectrum

A MinZ implementation of an interactive digit editor for the ZX Spectrum, designed for creating and editing 16x16 pixel digits for MNIST-style recognition.

## Features

- **Attribute-based cursor**: Uses XOR on attribute bytes for cursor visibility
- **16x16 digit canvas**: Draw digits in the top-left corner of the screen
- **Efficient screen handling**: Fills screen with 0xFF pattern, attributes with 0x00
- **Simple controls**: Navigate with QAOP keys, toggle pixels with SPACE/M

## Controls

- **Q/A**: Move cursor up/down
- **O/P**: Move cursor left/right  
- **SPACE or M**: Toggle pixel at cursor position
- **Symbol Shift + SPACE**: Exit editor

## Technical Details

### Screen Layout
- Screen bitmap filled with 0xFF (all pixels on)
- Attributes set to 0x00 (black ink on black paper)
- Border set to black (0)
- Cursor visibility achieved by XORing attribute byte with 0x04

### Memory Map
- Screen bitmap: 0x4000-0x57FF (6KB)
- Screen attributes: 0x5800-0x5AFF (768 bytes)
- Code starts at: 0x8000 (standard for ZX Spectrum programs)

### 16x16 Digit Representation
The cursor position (0-31, 0-23) maps to a 16x16 pixel grid:
- X coordinate: Lower 4 bits (0-15) 
- Y coordinate: Lower 4 bits (0-15)
- Pixels are toggled in the top-left corner of the screen

## Building

```bash
cd minzc
./minzc ../examples/mnist/editor.minz -o editor.a80
```

## Files

- `editor.minz` - Main consolidated editor implementation
- `mnist_attr_editor.minz` - Original attribute-based implementation
- `mnist_editor_simple.minz` - Simplified version
- `mnist_editor_minimal.minz` - Minimal test version
- `mnist_editor.minz` - Initial implementation attempt
- `mnist_editor.asm` - Assembly language reference

## Future Enhancements

- Add neural network inference for digit recognition
- Implement save/load functionality
- Add multiple digit storage
- Port fuzzy pattern matching from the Go implementation