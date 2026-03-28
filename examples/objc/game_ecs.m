// ObjC Entity-Component-System on Z80
//
// A tiny ECS game engine: entities with health, position, velocity.
// All methods compile to direct CALL — zero vtable overhead.
// Shows ObjC as a game scripting language on 8-bit hardware.

@interface Entity {
    int hp;
    int maxHp;
    int x;
    int y;
    int vx;
    int vy;
    int type;
}
-(int)hp;
-(int)x;
-(int)y;
-(int)isAlive;
-(int)isDead;
-(int)hpPercent;
-(Entity*)damage:(int)amount;
-(Entity*)heal:(int)amount;
-(Entity*)tick;
-(int)distanceTo:(int)tx y:(int)ty;
@end

@implementation Entity

-(int)hp { return self->hp; }
-(int)x { return self->x; }
-(int)y { return self->y; }

-(int)isAlive {
    if (self->hp > 0) return 1;
    return 0;
}

-(int)isDead {
    if (self->hp == 0) return 1;
    return 0;
}

-(int)hpPercent {
    return self->hp * 100 / self->maxHp;
}

-(Entity*)damage:(int)amount {
    if (amount > self->hp)
        self->hp = 0;
    else
        self->hp = self->hp - amount;
    return self;
}

-(Entity*)heal:(int)amount {
    self->hp = self->hp + amount;
    if (self->hp > self->maxHp)
        self->hp = self->maxHp;
    return self;
}

-(Entity*)tick {
    self->x = self->x + self->vx;
    self->y = self->y + self->vy;
    return self;
}

-(int)distanceTo:(int)tx y:(int)ty {
    int dx;
    int dy;
    if (self->x > tx)
        dx = self->x - tx;
    else
        dx = tx - self->x;
    if (self->y > ty)
        dy = self->y - ty;
    else
        dy = ty - self->y;
    return dx + dy;
}

@end

// C helpers
int add(int a, int b) { return a + b; }

// assert add(1, 2) == 3 via mir2

// --- Entity getters ---
// assert-objc Entity{hp:100, maxHp:100, x:5, y:10, vx:1, vy:2, type:0}.hp() == 100
// assert-objc Entity{hp:100, maxHp:100, x:5, y:10, vx:1, vy:2, type:0}.x() == 5
// assert-objc Entity{hp:100, maxHp:100, x:5, y:10, vx:1, vy:2, type:0}.y() == 10

// --- Alive/Dead ---
// assert-objc Entity{hp:100, maxHp:100, x:0, y:0, vx:0, vy:0, type:0}.isAlive() == 1
// assert-objc Entity{hp:0, maxHp:100, x:0, y:0, vx:0, vy:0, type:0}.isAlive() == 0
// assert-objc Entity{hp:0, maxHp:100, x:0, y:0, vx:0, vy:0, type:0}.isDead() == 1

// --- HP percent ---
// assert-objc Entity{hp:50, maxHp:100, x:0, y:0, vx:0, vy:0, type:0}.hpPercent() == 50
// assert-objc Entity{hp:100, maxHp:100, x:0, y:0, vx:0, vy:0, type:0}.hpPercent() == 100

// --- Manhattan distance ---
// assert-objc Entity{hp:0, maxHp:0, x:3, y:4, vx:0, vy:0, type:0}.distanceTo(0, 0) == 7
// assert-objc Entity{hp:0, maxHp:0, x:10, y:10, vx:0, vy:0, type:0}.distanceTo(10, 10) == 0
