# MZA vs SjASMPlus Compatibility Report

## Executive Summary

| Assembler | Success Count | Success Rate | Unique Successes |
|-----------|---------------|--------------|------------------|
| MZA       | 0          | 0%         | 0              |
| SjASMPlus | 0          | 0%         | 0              |
| Both      | 0          | 0%         | -                |

## Test Categories

### 1. Supertest Z80 (Comprehensive Instruction Set)
| Category | MZA | SjASMPlus | Notes |
|----------|-----|-----------|-------|
| Comprehensive | ❌ FAIL | ❌ FAIL | Full Z80 instruction set |

## Detailed Analysis

### Files Both Assemblers Handle Successfully
0 files

### Files Only MZA Handles Successfully  
       1 files

### Files Only SjASMPlus Handles Successfully
0 files

### Files Neither Assembler Handles
      99 files

## Compatibility Recommendations

### MZA Strengths
- Modern parser handles complex expressions well
- Good support for hierarchical labels from MinZ
- Enhanced multi-arg instruction support

### SjASMPlus Strengths  
- Mature Z80 assembler with extensive validation
- Broad instruction set support
- Industry standard compatibility

### Key Gaps Identified
- MZA needs broader Z80 instruction support
- Missing edge case handling compared to mature SjASMPlus

## Test Environment
- Test Date: Sat 16 Aug 2025 20:11:32 IST
- MZA Version: Unknown
- SjASMPlus Version: Unrecognized option: version
SjASMPlus Z80 Cross-Assembler v1.07 RC8 (build 06-11-2008)
No inputfile(s)
Unknown
- Test Files: 0 sample from 2000+ corpus
