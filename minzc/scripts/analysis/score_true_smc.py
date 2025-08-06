#!/usr/bin/env python3
# score_true_smc.py - Score TRUE SMC assembly output

import re
import sys
import os

def score_assembly(filename):
    """Score a TRUE SMC assembly file based on key patterns."""
    
    if not os.path.exists(filename):
        print(f"Error: File {filename} not found")
        return 0, 0
    
    with open(filename, 'r') as f:
        content = f.read()
    
    score = 0
    max_score = 0
    details = []
    
    # Check for anchors (parameter$imm0:)
    anchors = re.findall(r'(\w+)\$imm0:', content)
    if anchors:
        anchor_score = min(len(anchors) * 10, 30)
        score += anchor_score
        details.append(f"✅ Found {len(anchors)} anchors: {', '.join(anchors)} (+{anchor_score} points)")
    else:
        details.append("❌ No parameter anchors found")
    max_score += 30
    
    # Check for anchor reuse patterns
    reuses = re.findall(r'LD\s+\w+,\s*\((\w+\$imm0)\)', content)
    if reuses:
        unique_reuses = set(reuses)
        reuse_score = min(len(reuses) * 5, 25)
        score += reuse_score
        details.append(f"✅ Found {len(reuses)} anchor reuses ({len(unique_reuses)} unique) (+{reuse_score} points)")
    else:
        details.append("❌ No anchor reuse patterns found")
    max_score += 25
    
    # Check for patching logic (modifications to anchors)
    patches = re.findall(r'LD\s*\((\w+\$imm0)\+1\),', content)
    if patches:
        patch_score = min(len(patches) * 10, 20)
        score += patch_score
        details.append(f"✅ Found {len(patches)} anchor patches (+{patch_score} points)")
    else:
        details.append("❌ No anchor patching found")
    max_score += 20
    
    # Check for DI/EI pairs (16-bit atomic updates)
    di_ei_pairs = len(re.findall(r'DI\s*\n.*?EI', content, re.DOTALL))
    if di_ei_pairs:
        di_score = min(di_ei_pairs * 15, 15)
        score += di_score
        details.append(f"✅ Found {di_ei_pairs} DI/EI pairs for atomic updates (+{di_score} points)")
    else:
        # Only a concern if there are 16-bit parameters
        if re.search(r'LD\s+HL,\s*0.*anchor', content):
            details.append("⚠️  16-bit anchors without DI/EI protection")
        else:
            details.append("ℹ️  No 16-bit parameters requiring DI/EI")
            score += 15  # Full points if not needed
    max_score += 15
    
    # Check for immediate loads in anchors
    imm_anchors = re.findall(r'(LD\s+\w+,\s*0\s*;.*anchor)', content)
    if imm_anchors:
        imm_score = 10
        score += imm_score
        details.append(f"✅ Found {len(imm_anchors)} immediate anchor loads (+{imm_score} points)")
    else:
        details.append("❌ No immediate loads in anchors")
    max_score += 10
    
    # Print detailed results
    print(f"\n=== Scoring {filename} ===")
    for detail in details:
        print(detail)
    
    # Calculate percentage
    percentage = (score / max_score) * 100 if max_score > 0 else 0
    print(f"\nScore: {score}/{max_score} ({percentage:.1f}%)")
    
    # Grade
    if percentage >= 90:
        grade = "A - Excellent TRUE SMC implementation"
    elif percentage >= 80:
        grade = "B - Good TRUE SMC implementation"
    elif percentage >= 70:
        grade = "C - Acceptable TRUE SMC implementation"
    elif percentage >= 60:
        grade = "D - Basic TRUE SMC implementation"
    else:
        grade = "F - Needs significant improvement"
    
    print(f"Grade: {grade}\n")
    
    # Additional analysis
    if anchors:
        print("Anchor Analysis:")
        for anchor in anchors:
            # Count how many times each anchor is reused
            reuse_count = len([r for r in reuses if r == f"{anchor}$imm0"])
            print(f"  - {anchor}: created once, reused {reuse_count} times")
    
    return score, max_score

def compare_files(true_smc_file, regular_file):
    """Compare TRUE SMC output with regular SMC output."""
    print("\n=== Comparison Mode ===")
    
    if os.path.exists(regular_file):
        with open(true_smc_file, 'r') as f1, open(regular_file, 'r') as f2:
            true_lines = len(f1.readlines())
            regular_lines = len(f2.readlines())
            
        print(f"TRUE SMC: {true_lines} lines")
        print(f"Regular SMC: {regular_lines} lines")
        print(f"Size reduction: {regular_lines - true_lines} lines ({((regular_lines - true_lines) / regular_lines * 100):.1f}%)")

def main():
    if len(sys.argv) < 2:
        print("Usage: python score_true_smc.py <assembly_file> [regular_smc_file]")
        print("Example: python score_true_smc.py test1_true_smc.a80")
        print("         python score_true_smc.py test1_true_smc.a80 test1_regular.a80")
        sys.exit(1)
    
    filename = sys.argv[1]
    score, max_score = score_assembly(filename)
    
    # Optional: compare with regular SMC
    if len(sys.argv) > 2:
        compare_files(filename, sys.argv[2])
    
    # Return exit code based on grade
    percentage = (score / max_score) * 100 if max_score > 0 else 0
    sys.exit(0 if percentage >= 70 else 1)

if __name__ == "__main__":
    main()