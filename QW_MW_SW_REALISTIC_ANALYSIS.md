# QW-MW-SW Realistic Analysis

## Current State: 72% (42/58 examples)
**Goal: 80% (47/58) - Need 5 more examples**

## Quick Wins (< 1 hour each)

### QW1: Add Missing Built-in Functions ⚡
**Time:** 30 minutes
**Impact:** +1 example (partial fixes for others)
**Functions to add:**
- `hex(n: u8) -> str` - Convert to hex string
- `set_paper(color: u8)` - ZX Spectrum function
- `set_ink(color: u8)` - ZX Spectrum function
**Implementation:** Simple stubs in semantic analyzer

### QW2: Fix Simple Cast Errors ⚡
**Time:** 45 minutes
**Impact:** +1 example
**Fix:** Add u8 to *u8 cast support in semantic analyzer
**Affected:** std.mem.find errors

## Medium Wins (2-4 hours each)

### MW1: Fix Pattern Matching Nil Body ⚠️
**Time:** 2-3 hours
**Impact:** +2 examples (game_state_machine, traffic_light_fsm)
**Issue:** Parser generates nil body for case arms
**Fix:** Debug parser and fix case statement body handling

### MW2: Basic Method Call Syntax ⚠️
**Time:** 3-4 hours
**Impact:** +2-3 examples (zero_cost_interfaces*)
**Issue:** p.method() doesn't resolve
**Fix:** Add dot notation method resolution

### MW3: Simple Type Casts ⚠️
**Time:** 2 hours
**Impact:** +1-2 examples
**Fix:** Implement basic cast operations between numeric types

## Slow Wins (> 4 hours each)

### SW1: Complete Method Calls with Self ❌
**Time:** 6-8 hours
**Impact:** +3-4 examples
**Complexity:** Full OOP support with self parameter injection

### SW2: Lambda Closures ❌
**Time:** 8+ hours
**Impact:** +2 examples
**Complexity:** Capture outer variables in nested functions

### SW3: Advanced Pattern Matching ❌
**Time:** 8+ hours
**Impact:** +2-3 examples
**Complexity:** Full pattern matching with guards and destructuring

## Recommended Action Plan

### Phase 1: Quick Wins (1 hour total) → 74% (44/58)
1. ✅ QW1: Add missing built-ins (30 min) → +1
2. ✅ QW2: Fix simple casts (30 min) → +1

### Phase 2: Key Medium Win (3 hours) → 78% (45/58)
3. ⚠️ MW1: Fix pattern matching nil body → +2

### Phase 3: Reach 80% (2 hours) → 80% (47/58)
4. ⚠️ MW3: Simple type casts → +1

**Total Time: 6 hours**

## Let's Start with Quick Wins!