// Bounded admission experiment: reuse historical instruction implementation,
// never call its search main. Output only modeled A/B/carry, not a full CPU.
#define main historical_search_main
#include "source/z80_divmod.cu"
#undef main
#include <chrono>

__global__ void smoke(uint8_t *out) {
 unsigned input=threadIdx.x;
 uint8_t sequences[2][3]={{OP_LD_BA,OP_ADD_AA,OP_ADD_AB},{OP_SRL_A,OP_RRA,OP_ADC_AB}};
 for(unsigned test=0;test<2;test++) {
  uint8_t a=input,b=0,aS=17;bool carry=true,carryS=true;
  for(int i=0;i<3;i++) exec_op(sequences[test][i],a,b,carry,aS,carryS);
  unsigned pos=(test*256+input)*3;out[pos]=a;out[pos+1]=b;out[pos+2]=carry;
 }
}
static void check(cudaError_t e) {if(e!=cudaSuccess){fprintf(stderr,"CUDA error: %s\n",cudaGetErrorString(e));exit(1);}}
int main() {
 uint8_t *d,h[1536]; auto start=std::chrono::steady_clock::now();
 check(cudaMalloc(&d,sizeof(h)));
 cudaEvent_t begin,end;check(cudaEventCreate(&begin));check(cudaEventCreate(&end));
 check(cudaEventRecord(begin));smoke<<<1,256>>>(d);check(cudaGetLastError());check(cudaEventRecord(end));check(cudaEventSynchronize(end));
 float ms;check(cudaEventElapsedTime(&ms,begin,end));check(cudaMemcpy(h,d,sizeof(h),cudaMemcpyDeviceToHost));
 auto elapsed=std::chrono::duration<double,std::milli>(std::chrono::steady_clock::now()-start).count();
 fprintf(stderr,"kernel_ms=%.6f cuda_context_alloc_kernel_copy_ms=%.6f threads=256 device_output_bytes=%zu\n",ms,elapsed,sizeof(h));
 for(unsigned test=0;test<2;test++) for(unsigned i=0;i<256;i++){unsigned p=(test*256+i)*3;printf("%u,%u,%u,%u,%u\n",test,i,h[p],h[p+1],h[p+2]);}
 check(cudaFree(d));check(cudaEventDestroy(begin));check(cudaEventDestroy(end));
}
