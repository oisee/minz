-- MinZ Metafunction Benchmarking Framework
-- Measures and compares metafunction performance vs traditional approaches

local benchmark = {}

-- Benchmark registry
benchmark.tests = {}
benchmark.results = {}

-- Register a benchmark test
function benchmark.register(name, test_func)
    benchmark.tests[name] = test_func
end

-- Run all benchmarks and generate report
function benchmark.run_all()
    print("=== MinZ Metafunction Benchmark Report ===")
    print(string.format("Generated: %s", os.date()))
    print("")
    
    for name, test_func in pairs(benchmark.tests) do
        print(string.format("Running benchmark: %s", name))
        local result = test_func()
        benchmark.results[name] = result
        benchmark.print_result(name, result)
        print("")
    end
    
    benchmark.generate_summary()
end

-- Print individual benchmark result
function benchmark.print_result(name, result)
    print(string.format("  Cycles: %d", result.cycles))
    print(string.format("  Bytes: %d", result.bytes))
    print(string.format("  Memory: %d", result.memory))
    print(string.format("  Efficiency: %.2f cycles/byte", result.cycles / result.bytes))
    
    if result.baseline then
        local speedup = result.baseline.cycles / result.cycles
        local size_ratio = result.baseline.bytes / result.bytes
        print(string.format("  Speedup: %.2fx", speedup))
        print(string.format("  Size reduction: %.2fx", size_ratio))
    end
end

-- Generate summary report
function benchmark.generate_summary()
    print("=== SUMMARY ===")
    
    local total_cycles = 0
    local total_bytes = 0
    local total_speedup = 0
    local count = 0
    
    for name, result in pairs(benchmark.results) do
        total_cycles = total_cycles + result.cycles
        total_bytes = total_bytes + result.bytes
        if result.baseline then
            total_speedup = total_speedup + (result.baseline.cycles / result.cycles)
            count = count + 1
        end
    end
    
    print(string.format("Total cycles: %d", total_cycles))
    print(string.format("Total bytes: %d", total_bytes))
    print(string.format("Average efficiency: %.2f cycles/byte", total_cycles / total_bytes))
    
    if count > 0 then
        print(string.format("Average speedup: %.2fx", total_speedup / count))
    end
end

-- Benchmark 1: @print vs traditional printf
benchmark.register("print_comparison", function()
    -- Metafunction version
    local metafunction_code = minz.compile([[
        fun test_metafunction() -> void {
            @print("Hello, {}! You scored {} points.", "Alice", 95);
        }
    ]])
    
    -- Traditional version
    local traditional_code = minz.compile([[
        fun test_traditional() -> void {
            printf("Hello, %s! You scored %d points.", "Alice", 95);
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local trad_stats = minz.analyze_code(traditional_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = trad_stats.cycles,
            bytes = trad_stats.bytes,
            memory = trad_stats.memory
        }
    }
end)

-- Benchmark 2: Compile-time constants vs runtime conversion
benchmark.register("constant_optimization", function()
    -- Metafunction with compile-time constants
    local metafunction_code = minz.compile([[
        fun test_constants() -> void {
            @print("Values: {}, {}, {}", 42, 255, 1000);
        }
    ]])
    
    -- Runtime number-to-string conversion
    local runtime_code = minz.compile([[
        fun test_runtime() -> void {
            print_u8(42);
            print_u8(255);
            print_u16(1000);
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local runtime_stats = minz.analyze_code(runtime_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = runtime_stats.cycles,
            bytes = runtime_stats.bytes,
            memory = runtime_stats.memory
        }
    }
end)

-- Benchmark 3: @hex formatting vs manual hex conversion
benchmark.register("hex_formatting", function()
    -- Metafunction hex formatting
    local metafunction_code = minz.compile([[
        fun test_hex_meta() -> void {
            @print("Address: 0x{}", @hex(0x1234));
            @print("Byte: 0x{}", @hex(0xAB));
        }
    ]])
    
    -- Manual hex conversion
    local manual_code = minz.compile([[
        fun test_hex_manual() -> void {
            print("Address: 0x");
            print_hex_u16(0x1234);
            print("Byte: 0x");
            print_hex_u8(0xAB);
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local manual_stats = minz.analyze_code(manual_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = manual_stats.cycles,
            bytes = manual_stats.bytes,
            memory = manual_stats.memory
        }
    }
end)

-- Benchmark 4: @debug vs manual debug printing
benchmark.register("debug_printing", function()
    -- Metafunction debug
    local metafunction_code = minz.compile([[
        fun test_debug_meta() -> void {
            let x: u16 = 100;
            let y: u16 = 200;
            @debug(x);
            @debug(y);
        }
    ]])
    
    -- Manual debug printing
    local manual_code = minz.compile([[
        fun test_debug_manual() -> void {
            let x: u16 = 100;
            let y: u16 = 200;
            print("[DEBUG] x = ");
            print_u16(x);
            print("[DEBUG] y = ");
            print_u16(y);
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local manual_stats = minz.analyze_code(manual_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = manual_stats.cycles,
            bytes = manual_stats.bytes,
            memory = manual_stats.memory
        }
    }
end)

-- Benchmark 5: @format string building vs runtime concatenation
benchmark.register("string_building", function()
    -- Compile-time format
    local metafunction_code = minz.compile([[
        fun test_format_meta() -> void {
            let name = @format("User_{}", 123);
            let message = @format("Hello, {}!", name);
            @print("{}", message);
        }
    ]])
    
    -- Runtime string building
    local runtime_code = minz.compile([[
        fun test_format_runtime() -> void {
            print("User_");
            print_u16(123);
            let separator = "!";
            print("Hello, User_");
            print_u16(123);
            print(separator);
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local runtime_stats = minz.analyze_code(runtime_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = runtime_stats.cycles,
            bytes = runtime_stats.bytes,
            memory = runtime_stats.memory
        }
    }
end)

-- Benchmark 6: Complex interpolation
benchmark.register("complex_interpolation", function()
    -- Metafunction interpolation
    local metafunction_code = minz.compile([[
        fun test_complex_meta() -> void {
            let player = "Alice";
            let score: u16 = 1500;
            let level: u8 = 5;
            @print("Player {} reached level {} with {} points (0x{})!", 
                   player, level, score, @hex(score));
        }
    ]])
    
    -- Manual assembly
    local manual_code = minz.compile([[
        fun test_complex_manual() -> void {
            let player = "Alice";
            let score: u16 = 1500;
            let level: u8 = 5;
            print("Player ");
            print(player);
            print(" reached level ");
            print_u8(level);
            print(" with ");
            print_u16(score);
            print(" points (0x");
            print_hex_u16(score);
            print(")!");
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local manual_stats = minz.analyze_code(manual_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = manual_stats.cycles,
            bytes = manual_stats.bytes,
            memory = manual_stats.memory
        }
    }
end)

-- Benchmark 7: Platform-specific operations
benchmark.register("platform_operations", function()
    -- Metafunction platform code
    local metafunction_code = minz.compile([[
        fun test_platform_meta() -> void {
            @zx_cls();
            @zx_beep(50, 100);
            @print("ZX Spectrum ready!");
        }
    ]])
    
    -- Manual platform code
    local manual_code = minz.compile([[
        fun test_platform_manual() -> void {
            asm { CALL 0x0DAF }  // CLS
            asm { 
                LD HL, 50
                LD DE, 100
                CALL 0x03B5
            }
            print("ZX Spectrum ready!");
        }
    ]])
    
    local meta_stats = minz.analyze_code(metafunction_code)
    local manual_stats = minz.analyze_code(manual_code)
    
    return {
        cycles = meta_stats.cycles,
        bytes = meta_stats.bytes,
        memory = meta_stats.memory,
        baseline = {
            cycles = manual_stats.cycles,
            bytes = manual_stats.bytes,
            memory = manual_stats.memory
        }
    }
end)

-- Run the benchmarks
if minz.is_benchmark_mode() then
    benchmark.run_all()
end

return benchmark