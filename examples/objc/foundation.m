// MinZ Foundation — Cocoa-style core types for Z80
//
// Static dispatch, zero-cost abstractions, no runtime.
// Looks like macOS/early iOS — compiles to Z80.

// ═══════════════════════════════════════════════════════════════════════
// NSNumber — boxed integer value
// ═══════════════════════════════════════════════════════════════════════

@interface NSNumber {
    int _value;
}
-(int)intValue;
-(int)isEqualToNumber:(int)other;
-(NSNumber*)add:(int)n;
-(NSNumber*)sub:(int)n;
-(NSNumber*)mul:(int)n;
-(int)compare:(int)other;
@end

@implementation NSNumber
-(int)intValue { return self->_value; }

-(int)isEqualToNumber:(int)other {
    if (self->_value == other) return 1;
    return 0;
}

-(NSNumber*)add:(int)n {
    self->_value = self->_value + n;
    return self;
}

-(NSNumber*)sub:(int)n {
    self->_value = self->_value - n;
    return self;
}

-(NSNumber*)mul:(int)n {
    self->_value = self->_value * n;
    return self;
}

-(int)compare:(int)other {
    if (self->_value < other) return -1;
    if (self->_value > other) return 1;
    return 0;
}
@end

// ═══════════════════════════════════════════════════════════════════════
// NSPoint / NSSize / NSRect — geometry (classic Cocoa)
// ═══════════════════════════════════════════════════════════════════════

@interface NSPoint {
    int x;
    int y;
}
-(int)x;
-(int)y;
-(NSPoint*)offset:(int)dx dy:(int)dy;
-(int)distanceSquaredTo:(int)px py:(int)py;
-(int)isEqual:(int)ox oy:(int)oy;
@end

@implementation NSPoint
-(int)x { return self->x; }
-(int)y { return self->y; }

-(NSPoint*)offset:(int)dx dy:(int)dy {
    self->x = self->x + dx;
    self->y = self->y + dy;
    return self;
}

-(int)distanceSquaredTo:(int)px py:(int)py {
    int dx = self->x - px;
    int dy = self->y - py;
    return dx * dx + dy * dy;
}

-(int)isEqual:(int)ox oy:(int)oy {
    if (self->x == ox)
        if (self->y == oy)
            return 1;
    return 0;
}
@end

@interface NSSize {
    int width;
    int height;
}
-(int)width;
-(int)height;
-(int)area;
-(NSSize*)scale:(int)factor;
@end

@implementation NSSize
-(int)width  { return self->width; }
-(int)height { return self->height; }
-(int)area   { return self->width * self->height; }

-(NSSize*)scale:(int)factor {
    self->width  = self->width * factor;
    self->height = self->height * factor;
    return self;
}
@end

@interface NSRect {
    int x;
    int y;
    int width;
    int height;
}
-(int)x;
-(int)y;
-(int)width;
-(int)height;
-(int)area;
-(int)maxX;
-(int)maxY;
-(int)containsX:(int)px y:(int)py;
-(NSRect*)inset:(int)dx dy:(int)dy;
-(NSRect*)offset:(int)dx dy:(int)dy;
@end

@implementation NSRect
-(int)x      { return self->x; }
-(int)y      { return self->y; }
-(int)width  { return self->width; }
-(int)height { return self->height; }
-(int)area   { return self->width * self->height; }
-(int)maxX   { return self->x + self->width; }
-(int)maxY   { return self->y + self->height; }

-(int)containsX:(int)px y:(int)py {
    if (px >= self->x)
        if (px < self->x + self->width)
            if (py >= self->y)
                if (py < self->y + self->height)
                    return 1;
    return 0;
}

-(NSRect*)inset:(int)dx dy:(int)dy {
    self->x = self->x + dx;
    self->y = self->y + dy;
    self->width  = self->width  - dx - dx;
    self->height = self->height - dy - dy;
    return self;
}

-(NSRect*)offset:(int)dx dy:(int)dy {
    self->x = self->x + dx;
    self->y = self->y + dy;
    return self;
}
@end

// ═══════════════════════════════════════════════════════════════════════
// NSColor — RGB color (8-bit per channel, packed)
// ═══════════════════════════════════════════════════════════════════════

@interface NSColor {
    int red;
    int green;
    int blue;
}
-(int)red;
-(int)green;
-(int)blue;
-(int)brightness;
-(int)isEqualToColor:(int)r g:(int)g b:(int)b;
-(NSColor*)blendWith:(int)r g:(int)g b:(int)b;
@end

@implementation NSColor
-(int)red   { return self->red; }
-(int)green { return self->green; }
-(int)blue  { return self->blue; }

-(int)brightness {
    // Approximate: (r + g + b) / 3
    return (self->red + self->green + self->blue) / 3;
}

-(int)isEqualToColor:(int)r g:(int)g b:(int)b {
    if (self->red == r)
        if (self->green == g)
            if (self->blue == b)
                return 1;
    return 0;
}

-(NSColor*)blendWith:(int)r g:(int)g b:(int)b {
    self->red   = (self->red + r) / 2;
    self->green = (self->green + g) / 2;
    self->blue  = (self->blue + b) / 2;
    return self;
}
@end

// ═══════════════════════════════════════════════════════════════════════
// NSRange — location + length (classic Foundation)
// ═══════════════════════════════════════════════════════════════════════

@interface NSRange {
    int location;
    int length;
}
-(int)location;
-(int)length;
-(int)maxRange;
-(int)containsIndex:(int)idx;
@end

@implementation NSRange
-(int)location { return self->location; }
-(int)length   { return self->length; }
-(int)maxRange { return self->location + self->length; }

-(int)containsIndex:(int)idx {
    if (idx >= self->location)
        if (idx < self->location + self->length)
            return 1;
    return 0;
}
@end

// ═══════════════════════════════════════════════════════════════════════
// Tests — feel like real Cocoa code
// ═══════════════════════════════════════════════════════════════════════

int identity(int x) { return x; }
// assert identity(1) == 1 via mir2

// --- NSNumber ---
// assert-objc NSNumber{_value:42}.intValue() == 42
// assert-objc NSNumber{_value:10}.isEqualToNumber(10) == 1
// assert-objc NSNumber{_value:10}.isEqualToNumber(20) == 0
// assert-objc NSNumber{_value:5}.compare(10) == 0xFFFF

// --- NSPoint ---
// assert-objc NSPoint{x:10, y:20}.x() == 10
// assert-objc NSPoint{x:10, y:20}.y() == 20
// assert-objc NSPoint{x:3, y:4}.distanceSquaredTo(0, 0) == 25
// assert-objc NSPoint{x:5, y:5}.isEqual(5, 5) == 1
// assert-objc NSPoint{x:5, y:5}.isEqual(5, 6) == 0

// --- NSSize ---
// assert-objc NSSize{width:10, height:20}.area() == 200
// assert-objc NSSize{width:3, height:7}.width() == 3

// --- NSRect ---
// assert-objc NSRect{x:0, y:0, width:100, height:50}.area() == 5000
// assert-objc NSRect{x:10, y:20, width:100, height:50}.maxX() == 110
// assert-objc NSRect{x:10, y:20, width:100, height:50}.maxY() == 70
// assert-objc NSRect{x:0, y:0, width:100, height:100}.containsX(50, 50) == 1
// assert-objc NSRect{x:0, y:0, width:100, height:100}.containsX(150, 50) == 0

// --- NSColor ---
// assert-objc NSColor{red:255, green:0, blue:0}.red() == 255
// assert-objc NSColor{red:60, green:120, blue:180}.brightness() == 120

// --- NSRange ---
// assert-objc NSRange{location:5, length:10}.maxRange() == 15
// assert-objc NSRange{location:5, length:10}.containsIndex(7) == 1
// assert-objc NSRange{location:5, length:10}.containsIndex(20) == 0
