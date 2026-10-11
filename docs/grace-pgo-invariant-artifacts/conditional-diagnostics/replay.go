package main
import("fmt";"os";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
func main(){source,e:=os.ReadFile(os.Args[1]);if e!=nil {panic(e)};dir:=os.Args[2];os.MkdirAll(dir,0700)
 for mask:=0;mask<4;mask++ {m,e:=nanz.Parse(string(source),"six-param.nanz");if e!=nil {panic(e)};sites,e:=hir.BuildLoopSiteMap(m);if e!=nil {panic(e)};if e=hir.SeedLoopLineage(m);e!=nil {panic(e)}
 m.PointerWalkPolicy=map[string]bool{};m.GraceRulePolicy=map[string]bool{"hir-invariant-base":true,"hir-constant-if":false};m.InvariantBasePolicy=map[string]bool{};for i,s:=range sites.Sites {m.InvariantBasePolicy[s.ID]=mask&(1<<i)!=0}
 steps,e:=pipeline.CompileHIRSteps(m,pipeline.Options{AssertMode:"none",UseHIRGrace:true});if e!=nil {panic(e)}
 for suffix,data:=range map[string]string{"hir":steps.HIR,"raw.mir":steps.MIR2Raw,"opt.mir":steps.MIR2Opt,"a80":steps.Assembly}{if e=os.WriteFile(fmt.Sprintf("%s/mask%d.%s",dir,mask,suffix),[]byte(data),0600);e!=nil {panic(e)}}
 program,e:=z80asm.NewAssembler().AssembleString("ORG 0x8000\n"+steps.Assembly);if e!=nil || len(program.Errors)!=0 {panic("assemble")};if e=os.WriteFile(fmt.Sprintf("%s/mask%d.bin",dir,mask),program.Binary,0600);e!=nil {panic(e)}
 w:=hir.ProfileWorkload{Schema:"minz.profile-workload/v1",TargetModel:"cpm-z80-v1",MaxSteps:200000,MaxMemory:65536}
 for _,n:=range []int64{0,1,5,255}{for _,f:=range []int64{0,1}{args:=[]hir.ProfileArgument{};for _,v:=range []int64{65535,2,60000,10000,n,f}{v:=v;args=append(args,hir.ProfileArgument{Value:&v})};w.Calls=append(w.Calls,hir.ProfileInvocation{Function:"scalar",Arguments:args})}}
 rawM,e:=nanz.Parse(string(source),"six-param.nanz");if e!=nil {panic(e)};raw,e:=hir.RunLoopProfile(rawM,w);if e!=nil {panic(e)};for i,c:=range w.Calls {a:=c.Arguments;want:=int64(uint16((uint64(*a[0].Value)+uint64(*a[1].Value)+(uint64(*a[2].Value)+uint64(*a[3].Value))*uint64(*a[5].Value))*uint64(*a[4].Value)));if raw.Results[i][0].I!=want {panic("independent arithmetic")}}
 _,e=pipeline.RunZ80ProfileWorkload(steps,rawM,w,*raw);fmt.Printf("mask%d: %v\n",mask,e)
 }
}
