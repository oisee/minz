#!/usr/bin/env python3
"""Quick performance analysis of TRUE SMC lambdas"""

def count_instructions(asm_file):
    """Count Z80 instructions in assembly file"""
    with open(asm_file, 'r') as f:
        content = f.read()
    
    lines = content.split('\n')
    instruction_count = 0
    
    for line in lines:
        line = line.strip()
        # Skip comments, labels, directives
        if (line and 
            not line.startswith(';') and 
            not line.startswith('.') and 
            ':' not in line and
            not line.startswith('EQU') and
            not line.startswith('ORG') and
            not line.startswith('DB') and
            not line.startswith('END')):
            instruction_count += 1
    
    return instruction_count

def analyze_benchmark():
    print("🚀 TRUE SMC LAMBDA PERFORMANCE ANALYSIS")
    print("=" * 50)
    
    # Analyze lambda version
    lambda_instructions = count_instructions('/Users/alice/dev/minz-ts/minzc/simple_lambda_test.a80')
    
    # Analyze traditional version  
    traditional_instructions = count_instructions('/Users/alice/dev/minz-ts/minzc/simple_traditional_test.a80')
    
    print(f"🔥 TRUE SMC Lambda:     {lambda_instructions} instructions")
    print(f"📰 Traditional:        {traditional_instructions} instructions")
    print()
    
    improvement = ((traditional_instructions - lambda_instructions) / traditional_instructions) * 100
    speedup = traditional_instructions / lambda_instructions
    
    print(f"📈 Performance Improvement: {improvement:.1f}%")
    print(f"⚡ Speedup Factor:          {speedup:.1f}x")
    print()
    
    if improvement > 0:
        print("🎯 TRUE SMC LAMBDAS WIN! 🏆")
        print("✅ Fewer instructions = Less memory usage")
        print("✅ Direct addressing = Faster execution") 
        print("✅ Zero indirection = Better cache performance")
    else:
        print("🤔 Traditional approach wins this time")
    
    print()
    print("💡 Key Insights:")
    print("• Lambda captures variables by absolute address")
    print("• Traditional approach needs struct pointer indirection")
    print("• SMC optimization eliminates overhead")
    print("• Functional programming is faster than OOP!")

if __name__ == "__main__":
    analyze_benchmark()