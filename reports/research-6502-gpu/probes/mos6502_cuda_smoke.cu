// Research host for the unmodified generated kernel in z80-optimizer.
// Copy next to mos6502_mulopt.cu, then: nvcc -O3 mos6502_cuda_smoke.cu -o mos6502_cuda_smoke
#include <cstdio>
#include <chrono>
#include <cuda_runtime.h>
#include "mos6502_mulopt.cu"

int main() {
    uint32_t *score, resultScore;
    uint64_t *index, resultIndex;
    if (cudaMalloc(&score,sizeof(*score))!=cudaSuccess || cudaMalloc(&index,sizeof(*index))!=cudaSuccess) return 2;
    uint64_t space=1;
    const char *names[]={"ASL A","LSR A","ROL A","ROR A","TAX","TXA","TAY","TYA","CLC","SEC","STA zp","ADC zp","SBC zp","EOR #$FF"};
    auto started=std::chrono::steady_clock::now();
    for (int len=1;len<=4;len++) {
        space*=NUM_OPS;
        resultScore=0xFFFFFFFF;resultIndex=0;
        cudaMemcpy(score,&resultScore,sizeof(resultScore),cudaMemcpyHostToDevice);
        cudaMemcpy(index,&resultIndex,sizeof(resultIndex),cudaMemcpyHostToDevice);
        mos6502_mulopt_kernel<<<(space+255)/256,256>>>(3,len,0,space,score,index);
        if (cudaDeviceSynchronize()!=cudaSuccess) return 3;
        cudaMemcpy(&resultScore,score,sizeof(resultScore),cudaMemcpyDeviceToHost);
        cudaMemcpy(&resultIndex,index,sizeof(resultIndex),cudaMemcpyDeviceToHost);
        std::printf("K=3 len=%d candidates=%llu score=%u",len,(unsigned long long)space,resultScore);
        if(resultScore!=0xFFFFFFFF) {
            uint8_t ops[12];uint64_t tmp=resultIndex;
            for(int i=len-1;i>=0;i--){ops[i]=tmp%NUM_OPS;tmp/=NUM_OPS;}
            for(int i=0;i<len;i++)std::printf(" %s;",names[ops[i]]);
        }
        std::printf("\n");
    }
    auto elapsed=std::chrono::duration<double>(std::chrono::steady_clock::now()-started).count();
    std::printf("kernel+transfer wall_seconds=%.6f (CUDA context warm)\n",elapsed);
    cudaFree(score);cudaFree(index);
}
