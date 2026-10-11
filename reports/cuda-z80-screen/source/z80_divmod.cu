// z80_divmod.cu — GPU brute-force optimal division/modulo by constant for Z80
//
// Finds shortest instruction sequences for:
//   --div  K : A_out = floor(A_in / K)
//   --mod  K : A_out = A_in % K
//   --divmod K: A_out = A_in / K, B_out = A_in % K (both at once)
//
// Uses same instruction pool as multiply bruteforce.
// Initial state: A=input, B=0, carry=0, A'=0, carry'=0
//
// Build: nvcc -O3 -o z80_divmod z80_divmod.cu
// Usage: z80_divmod --div 10 [--max-len 8]
//        z80_divmod --mod 10 [--max-len 8]
//        z80_divmod --divmod 10 [--max-len 8]

#include <cstdint>
#include <cstdio>
#include <cstring>
#include <cstdlib>

// 21 instructions (same pool as mulopt)
#define OP_ADD_AA    0   // ADD A,A   (4T)
#define OP_ADD_AB    1   // ADD A,B   (4T)
#define OP_SUB_B     2   // SUB B     (4T)
#define OP_LD_BA     3   // LD B,A    (4T)
#define OP_ADC_AB    4   // ADC A,B   (4T)
#define OP_ADC_AA    5   // ADC A,A   (4T)
#define OP_SBC_AB    6   // SBC A,B   (4T)
#define OP_SBC_AA    7   // SBC A,A   (4T)
#define OP_SLA_A     8   // SLA A     (8T)
#define OP_SRA_A     9   // SRA A     (8T)
#define OP_SRL_A    10   // SRL A     (8T)
#define OP_RLA      11   // RLA       (4T)
#define OP_RRA      12   // RRA       (4T)
#define OP_RLCA     13   // RLCA      (4T)
#define OP_RRCA     14   // RRCA      (4T)
#define OP_RLC_A    15   // RLC A     (8T)
#define OP_RRC_A    16   // RRC A     (8T)
#define OP_OR_A     17   // OR A      (4T)
#define OP_NEG      18   // NEG       (8T)
#define OP_SCF      19   // SCF       (4T)
#define OP_EX_AF    20   // EX AF,AF' (4T)
#define NUM_OPS     21

__constant__ uint8_t opCost[NUM_OPS] = {
    4,4,4,4,4,4,4,4,   // ADD/ADC/SBC/LD
    8,8,8,              // SLA/SRA/SRL
    4,4,4,4,            // RLA/RRA/RLCA/RRCA
    8,8,                // RLC/RRC
    4,8,4,4             // OR A/NEG/SCF/EX AF
};

__device__ void exec_op(uint8_t op, uint8_t &a, uint8_t &b, bool &carry,
                        uint8_t &aS, bool &carryS) {
    uint16_t r, c;
    uint8_t bit;
    switch (op) {
    case OP_ADD_AA:
        r = (uint16_t)a + a; carry = r > 0xFF; a = (uint8_t)r; break;
    case OP_ADD_AB:
        r = (uint16_t)a + b; carry = r > 0xFF; a = (uint8_t)r; break;
    case OP_SUB_B:
        carry = a < b; a = a - b; break;
    case OP_LD_BA:
        b = a; break;
    case OP_ADC_AB:
        c = carry ? 1 : 0; r = (uint16_t)a + b + c; carry = r > 0xFF; a = (uint8_t)r; break;
    case OP_ADC_AA:
        c = carry ? 1 : 0; r = (uint16_t)a + a + c; carry = r > 0xFF; a = (uint8_t)r; break;
    case OP_SBC_AB:
        c = carry ? 1 : 0; carry = ((int16_t)a - (int16_t)b - (int16_t)c) < 0;
        a = a - b - (uint8_t)c; break;
    case OP_SBC_AA:
        c = carry ? 1 : 0; carry = c > 0; a = -(uint8_t)c; break;
    case OP_SLA_A:
        carry = (a & 0x80) != 0; a = a << 1; break;
    case OP_SRA_A:
        carry = (a & 0x01) != 0; a = (uint8_t)((int8_t)a >> 1); break;
    case OP_SRL_A:
        carry = (a & 0x01) != 0; a = a >> 1; break;
    case OP_RLA:
        bit = carry ? 1 : 0; carry = (a & 0x80) != 0; a = (a << 1) | bit; break;
    case OP_RRA:
        bit = carry ? 0x80 : 0; carry = (a & 0x01) != 0; a = (a >> 1) | bit; break;
    case OP_RLCA:
        carry = (a & 0x80) != 0; a = (a << 1) | (a >> 7); break;
    case OP_RRCA:
        carry = (a & 0x01) != 0; a = (a >> 1) | (a << 7); break;
    case OP_RLC_A:
        carry = (a & 0x80) != 0; a = (a << 1) | (a >> 7); break;
    case OP_RRC_A:
        carry = (a & 0x01) != 0; a = (a >> 1) | (a << 7); break;
    case OP_OR_A:
        carry = false; break;
    case OP_NEG:
        carry = (a != 0); a = (uint8_t)(0 - a); break;
    case OP_SCF:
        carry = true; break;
    case OP_EX_AF:
        { uint8_t ta = a; a = aS; aS = ta;
          bool tc = carry; carry = carryS; carryS = tc; }
        break;
    }
}

// Initial B value (set via --init-b, searched 0-255 by default)
__constant__ uint8_t d_initB;

// Run sequence, return (A, B) packed as uint16
__device__ uint16_t run_seq(const uint8_t *ops, int len, uint8_t input) {
    uint8_t a = input, b = d_initB, aS = 0;
    bool carry = false, carryS = false;
    for (int i = 0; i < len; i++) {
        exec_op(ops[i], a, b, carry, aS, carryS);
    }
    return ((uint16_t)a << 8) | b;
}

__device__ void decode_seq(uint64_t idx, int len, uint8_t *ops) {
    for (int i = len - 1; i >= 0; i--) {
        ops[i] = (uint8_t)(idx % NUM_OPS);
        idx /= NUM_OPS;
    }
}

__device__ uint16_t seq_cost(const uint8_t *ops, int len) {
    uint16_t cost = 0;
    for (int i = 0; i < len; i++) cost += opCost[ops[i]];
    return cost;
}

// Mode flags
#define MODE_DIV    1  // verify A = input / K
#define MODE_MOD    2  // verify A = input % K
#define MODE_DIVMOD 3  // verify A = input / K AND B = input % K

__device__ bool verify_one(uint16_t ab, uint8_t input, uint8_t k, int mode) {
    uint8_t a = (uint8_t)(ab >> 8);
    uint8_t b = (uint8_t)ab;
    uint8_t quot = input / k;
    uint8_t rem  = input % k;
    switch (mode) {
    case MODE_DIV:    return a == quot;
    case MODE_MOD:    return a == rem;
    case MODE_DIVMOD: return a == quot && b == rem;
    }
    return false;
}

__global__ void divmod_kernel(uint8_t k, int mode, int seqLen,
                               uint64_t offset, uint64_t count,
                               uint32_t *d_bestScore, uint64_t *d_bestIdx) {
    uint64_t tid = blockIdx.x * (uint64_t)blockDim.x + threadIdx.x;
    if (tid >= count) return;

    uint64_t seqIdx = offset + tid;
    uint8_t ops[12];
    decode_seq(seqIdx, seqLen, ops);

    // QuickCheck: 4 discriminating inputs
    if (!verify_one(run_seq(ops, seqLen, 0), 0, k, mode)) return;
    if (!verify_one(run_seq(ops, seqLen, 9), 9, k, mode)) return;
    if (!verify_one(run_seq(ops, seqLen, 10), 10, k, mode)) return;
    if (!verify_one(run_seq(ops, seqLen, 255), 255, k, mode)) return;

    // Full verification
    for (int input = 0; input < 256; input++) {
        if (!verify_one(run_seq(ops, seqLen, (uint8_t)input), (uint8_t)input, k, mode))
            return;
    }

    uint16_t cost = seq_cost(ops, seqLen);
    uint32_t score = ((uint32_t)seqLen << 16) | cost;
    uint32_t old = atomicMin(d_bestScore, score);
    if (score <= old) {
        atomicExch((unsigned long long *)d_bestIdx, (unsigned long long)seqIdx);
    }
}

static uint64_t ipow(uint64_t base, int exp) {
    uint64_t result = 1;
    for (int i = 0; i < exp; i++) result *= base;
    return result;
}

static const char *opName(uint8_t op) {
    static const char *names[] = {
        "ADD A,A", "ADD A,B", "SUB B", "LD B,A",
        "ADC A,B", "ADC A,A", "SBC A,B", "SBC A,A",
        "SLA A", "SRA A", "SRL A",
        "RLA", "RRA", "RLCA", "RRCA", "RLC A", "RRC A",
        "OR A", "NEG", "SCF", "EX AF,AF'"
    };
    return op < NUM_OPS ? names[op] : "?";
}

struct DivResult {
    int length;
    int tstates;
    uint8_t ops[12];
    uint8_t initB;
    bool found;
};

// Solve with a specific initial B value
static DivResult solve_with_b(int k, int mode, int maxLen, uint8_t bVal) {
    DivResult result;
    result.found = false;
    result.initB = bVal;

    cudaMemcpyToSymbol(d_initB, &bVal, sizeof(uint8_t));

    uint32_t *d_bestScore;
    uint64_t *d_bestIdx;
    cudaMalloc(&d_bestScore, sizeof(uint32_t));
    cudaMalloc(&d_bestIdx, sizeof(uint64_t));

    uint32_t initScore = 0xFFFFFFFF;
    uint64_t initIdx = 0;
    int blockSize = 256;

    for (int len = 1; len <= maxLen; len++) {
        uint64_t total = ipow(NUM_OPS, len);
        if (total > 500000000000ULL) break;

        cudaMemcpy(d_bestScore, &initScore, sizeof(uint32_t), cudaMemcpyHostToDevice);
        cudaMemcpy(d_bestIdx, &initIdx, sizeof(uint64_t), cudaMemcpyHostToDevice);

        uint64_t batchSize = (uint64_t)blockSize * 65535;
        uint64_t offset = 0;
        while (offset < total) {
            uint64_t count = total - offset;
            if (count > batchSize) count = batchSize;
            uint64_t grid = (count + blockSize - 1) / blockSize;
            divmod_kernel<<<(unsigned int)grid, blockSize>>>(
                (uint8_t)k, mode, len, offset, count, d_bestScore, d_bestIdx);
            cudaDeviceSynchronize();
            offset += count;
        }

        uint32_t bestScore;
        cudaMemcpy(&bestScore, d_bestScore, sizeof(uint32_t), cudaMemcpyDeviceToHost);

        if (bestScore != 0xFFFFFFFF) {
            uint64_t bestIdx;
            cudaMemcpy(&bestIdx, d_bestIdx, sizeof(uint64_t), cudaMemcpyDeviceToHost);
            result.found = true;
            result.length = len;
            result.tstates = bestScore & 0xFFFF;
            uint64_t idx = bestIdx;
            for (int i = len - 1; i >= 0; i--) {
                result.ops[i] = (uint8_t)(idx % NUM_OPS);
                idx /= NUM_OPS;
            }
            break;
        }
    }

    cudaFree(d_bestScore);
    cudaFree(d_bestIdx);
    return result;
}

// Solve: either with fixed B or try all 256 B values
static DivResult solve(int k, int mode, int maxLen, int fixedB) {
    if (fixedB >= 0) {
        fprintf(stderr, "  B=%d fixed\n", fixedB);
        DivResult r = solve_with_b(k, mode, maxLen, (uint8_t)fixedB);
        if (r.found) fprintf(stderr, "  FOUND with B=%d!\n", fixedB);
        else fprintf(stderr, "  not found with B=%d\n", fixedB);
        return r;
    }

    // Try all 256 B values, keep best result
    DivResult best;
    best.found = false;
    int bestScore = 0x7FFFFFFF;

    for (int bv = 0; bv < 256; bv++) {
        if (bv % 16 == 0) fprintf(stderr, "\r  trying B=%d..%d (best so far: %s)  ",
                                    bv, bv+15, best.found ? "yes" : "none");
        DivResult r = solve_with_b(k, mode, maxLen, (uint8_t)bv);
        if (r.found) {
            // Score: length * 1000 + tstates + 7 (for the LD B,n preamble)
            int score = r.length * 1000 + r.tstates;
            if (score < bestScore) {
                bestScore = score;
                best = r;
            }
        }
    }
    if (best.found) fprintf(stderr, "\r  BEST: B=%d, %d insts + LD B,%d, %dT + 7T      \n",
                             best.initB, best.length, best.initB, best.tstates);
    else fprintf(stderr, "\r  not found for any B value                    \n");
    return best;
}

static void print_result(const char *label, int k, DivResult &r, bool json) {
    if (!r.found) {
        if (json)
            printf("{\"op\": \"%s\", \"k\": %d, \"found\": false}\n", label, k);
        else
            printf("%s %d: NOT FOUND\n", label, k);
        return;
    }
    int totalInsts = r.length + (r.initB != 0 ? 1 : 0); // +1 for LD B,n
    int totalT = r.tstates + (r.initB != 0 ? 7 : 0);     // +7T for LD B,n
    if (json) {
        printf("{\"op\": \"%s\", \"k\": %d, \"initB\": %d, \"ops\": [", label, k, r.initB);
        if (r.initB != 0) printf("\"LD B,%d\", ", r.initB);
        for (int i = 0; i < r.length; i++) {
            if (i > 0 || r.initB != 0) printf(", ");
            printf("\"%s\"", opName(r.ops[i]));
        }
        printf("], \"length\": %d, \"tstates\": %d}\n", totalInsts, totalT);
    } else {
        printf("%s %d: ", label, k);
        if (r.initB != 0) printf("LD B,%d / ", r.initB);
        for (int i = 0; i < r.length; i++) {
            if (i > 0) printf(" / ");
            printf("%s", opName(r.ops[i]));
        }
        printf("  (%d insts, %dT)\n", totalInsts, totalT);
    }
    fflush(stdout);
}

int main(int argc, char *argv[]) {
    cudaSetDeviceFlags(cudaDeviceScheduleBlockingSync);

    int maxLen = 8;
    int divK = 0, modK = 0, divmodK = 0;
    int initB = -1; // -1 = try all 256 values, 0-255 = fixed
    bool jsonMode = false;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--max-len") == 0 && i+1 < argc) maxLen = atoi(argv[++i]);
        else if (strcmp(argv[i], "--div") == 0 && i+1 < argc) divK = atoi(argv[++i]);
        else if (strcmp(argv[i], "--mod") == 0 && i+1 < argc) modK = atoi(argv[++i]);
        else if (strcmp(argv[i], "--divmod") == 0 && i+1 < argc) divmodK = atoi(argv[++i]);
        else if (strcmp(argv[i], "--init-b") == 0 && i+1 < argc) initB = atoi(argv[++i]);
        else if (strcmp(argv[i], "--json") == 0) jsonMode = true;
        else if (strcmp(argv[i], "--help") == 0) {
            fprintf(stderr, "z80_divmod — GPU brute-force division/modulo by constant\n");
            fprintf(stderr, "Usage:\n");
            fprintf(stderr, "  z80_divmod --div 10     Find A/10 → A\n");
            fprintf(stderr, "  z80_divmod --mod 10     Find A%%10 → A\n");
            fprintf(stderr, "  z80_divmod --divmod 10  Find A/10 → A, A%%10 → B\n");
            fprintf(stderr, "  --max-len N  Maximum sequence length (default 8)\n");
            fprintf(stderr, "  --json       JSON output\n");
            return 0;
        }
    }

    if (divK > 0) {
        fprintf(stderr, "Searching for A/%d → A (max length %d, %d ops)\n", divK, maxLen, NUM_OPS);
        DivResult r = solve(divK, MODE_DIV, maxLen, initB);
        print_result("div", divK, r, jsonMode);
    }

    if (modK > 0) {
        fprintf(stderr, "Searching for A%%%d → A (max length %d, %d ops)\n", modK, maxLen, NUM_OPS);
        DivResult r = solve(modK, MODE_MOD, maxLen, initB);
        print_result("mod", modK, r, jsonMode);
    }

    if (divmodK > 0) {
        fprintf(stderr, "Searching for divmod %d: A→A/K, B→A%%K (max length %d, %d ops)\n",
                divmodK, maxLen, NUM_OPS);
        DivResult r = solve(divmodK, MODE_DIVMOD, maxLen, initB);
        print_result("divmod", divmodK, r, jsonMode);
    }

    if (divK == 0 && modK == 0 && divmodK == 0) {
        fprintf(stderr, "No target specified. Use --div, --mod, or --divmod.\n");
        return 1;
    }

    return 0;
}
