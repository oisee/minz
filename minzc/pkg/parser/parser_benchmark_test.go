package parser

import (
	"testing"
	"os"
)

// Test comparing ANTLR vs Native parser performance
func BenchmarkParserComparison(b *testing.B) {
	source := `
fun fibonacci(n: u16) -> u16 {
    if n <= 1 {
        return n;
    }
    return fibonacci(n - 1) + fibonacci(n - 2);
}

struct Point {
    x: u8,
    y: u8,
}

enum Color {
    Red,
    Green,
    Blue,
}

fun complex_function(points: [Point; 10], color: Color) -> u16 {
    let mut sum: u16 = 0;
    for i in 0..10 {
        sum = sum + (points[i].x as u16) + (points[i].y as u16);
    }
    return sum;
}

fun main() -> u8 {
    let points: [Point; 10];
    let result = complex_function(points, Color.Red);
    return result as u8;
}`

	b.Run("Native", func(b *testing.B) {
		parser := NewNativeParser()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_, err := parser.ParseString(source, "bench_native.minz")
			if err != nil {
				b.Fatalf("Native parse error: %v", err)
			}
		}
	})

	// Only run ANTLR benchmark if ANTLR is available
	if os.Getenv("MINZ_TEST_ANTLR") == "1" {
		b.Run("ANTLR", func(b *testing.B) {
			parser := NewAntlrParser()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				_, err := parser.ParseString(source, "bench_antlr.minz")
				if err != nil {
					b.Fatalf("ANTLR parse error: %v", err)
				}
			}
		})
	}
}

// Test memory usage comparison
func BenchmarkParserMemory(b *testing.B) {
	// Large source file to test memory usage
	source := `
// Large file with many declarations for memory testing
fun test_function_1() -> u8 { return 1; }
fun test_function_2() -> u8 { return 2; }
fun test_function_3() -> u8 { return 3; }
fun test_function_4() -> u8 { return 4; }
fun test_function_5() -> u8 { return 5; }

struct TestStruct1 { field1: u8, field2: u16, }
struct TestStruct2 { field1: u8, field2: u16, }
struct TestStruct3 { field1: u8, field2: u16, }

enum TestEnum1 { Variant1, Variant2, Variant3, }
enum TestEnum2 { Variant1, Variant2, Variant3, }
enum TestEnum3 { Variant1, Variant2, Variant3, }

const CONST1: u8 = 1;
const CONST2: u8 = 2;
const CONST3: u8 = 3;

global GLOBAL1: u8 = 1;
global GLOBAL2: u8 = 2;
global GLOBAL3: u8 = 3;

fun large_function_with_many_statements() -> u8 {
    let a: u8 = 1;
    let b: u8 = 2;
    let c: u8 = 3;
    let d: u8 = 4;
    let e: u8 = 5;
    
    if a > 0 {
        b = b + 1;
    }
    
    if b > 1 {
        c = c + 2;
    }
    
    if c > 2 {
        d = d + 3;
    }
    
    if d > 3 {
        e = e + 4;
    }
    
    return a + b + c + d + e;
}`

	b.Run("Native", func(b *testing.B) {
		parser := NewNativeParser()
		
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_, err := parser.ParseString(source, "bench_memory_native.minz")
			if err != nil {
				b.Fatalf("Native parse error: %v", err)
			}
		}
	})

	if os.Getenv("MINZ_TEST_ANTLR") == "1" {
		b.Run("ANTLR", func(b *testing.B) {
			parser := NewAntlrParser()
			
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				_, err := parser.ParseString(source, "bench_memory_antlr.minz")
				if err != nil {
					b.Fatalf("ANTLR parse error: %v", err)
				}
			}
		})
	}
}

// Test error handling performance
func BenchmarkParserErrorHandling(b *testing.B) {
	// Deliberately malformed source
	invalidSource := `
fun invalid_function( -> u8 {
    return 1
}
struct InvalidStruct {
    field1 u8,
    field2: 
}
enum InvalidEnum {
    Variant1
    Variant2,
`

	b.Run("Native-Errors", func(b *testing.B) {
		parser := NewNativeParser()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			// We expect this to fail, we're testing error handling performance
			parser.ParseString(invalidSource, "bench_error_native.minz")
		}
	})

	if os.Getenv("MINZ_TEST_ANTLR") == "1" {
		b.Run("ANTLR-Errors", func(b *testing.B) {
			parser := NewAntlrParser()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				// We expect this to fail, we're testing error handling performance
				parser.ParseString(invalidSource, "bench_error_antlr.minz")
			}
		})
	}
}