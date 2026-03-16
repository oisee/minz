// ObjC fluent transform pipeline — method chaining on Z80
//
// Demonstrates:
//   1. Methods returning self (ClassName*) for chaining
//   2. Nested message sends [[obj methodA] methodB]
//   3. Multi-keyword selectors with chaining
//   4. Transform pipeline pattern (scale, translate, clamp)

@interface Vec3 {
    int x;
    int y;
    int z;
}
-(int)x;
-(int)y;
-(int)z;
-(int)sum;
-(Vec3*)scale:(int)factor;
-(Vec3*)addX:(int)dx y:(int)dy z:(int)dz;
-(Vec3*)clampMax:(int)limit;
// Chaining wrappers (exercise [[self method] method])
-(int)scaleAndSum:(int)factor;
-(int)translateAndSum:(int)dx;
@end

@implementation Vec3

-(int)x { return self->x; }
-(int)y { return self->y; }
-(int)z { return self->z; }
-(int)sum { return self->x + self->y + self->z; }

-(Vec3*)scale:(int)factor {
    self->x = self->x * factor;
    self->y = self->y * factor;
    self->z = self->z * factor;
    return self;
}

-(Vec3*)addX:(int)dx y:(int)dy z:(int)dz {
    self->x = self->x + dx;
    self->y = self->y + dy;
    self->z = self->z + dz;
    return self;
}

-(Vec3*)clampMax:(int)limit {
    if (self->x > limit) self->x = limit;
    if (self->y > limit) self->y = limit;
    if (self->z > limit) self->z = limit;
    return self;
}

// [[self scale:factor] sum]  — chain: scale then read
-(int)scaleAndSum:(int)factor {
    return [[self scale:factor] sum];
}

// [[self addX:dx y:0 z:0] sum]  — chain with multi-keyword selector
-(int)translateAndSum:(int)dx {
    return [[self addX:dx y:0 z:0] sum];
}

@end

// --- Builder pattern: fluent config ---

@interface Config {
    int width;
    int height;
    int color;
}
-(Config*)width:(int)w;
-(Config*)height:(int)h;
-(Config*)color:(int)c;
-(int)area;
-(int)getColor;
// Chaining wrapper
-(int)setAndArea:(int)w;
@end

@implementation Config

-(Config*)width:(int)w {
    self->width = w;
    return self;
}

-(Config*)height:(int)h {
    self->height = h;
    return self;
}

-(Config*)color:(int)c {
    self->color = c;
    return self;
}

-(int)area { return self->width * self->height; }
-(int)getColor { return self->color; }

// [[[self width:w] height:10] area]  — triple chain
-(int)setAndArea:(int)w {
    return [[[self width:w] height:10] area];
}

@end

// C identity for basic sanity
int identity(int x) { return x; }

// assert identity(1) == 1 via mir2

// --- Vec3 transforms ---
// assert-objc Vec3{x:2, y:3, z:4}.sum() == 9
// assert-objc Vec3{x:1, y:2, z:3}.x() == 1

// Method chaining: scale(2) then sum → (1*2 + 2*2 + 3*2) = 12
// assert-objc Vec3{x:1, y:2, z:3}.scaleAndSum(2) == 12

// Method chaining: translate x by 10 then sum → (1+10 + 2 + 3) = 16
// assert-objc Vec3{x:1, y:2, z:3}.translateAndSum(10) == 16

// --- Config builder ---
// assert-objc Config{width:0, height:0, color:0}.area() == 0

// Triple chain: width(8), height(10) → area = 80
// assert-objc Config{width:0, height:0, color:0}.setAndArea(8) == 80
