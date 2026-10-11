package main
import("bytes";"compress/gzip";"crypto/sha256";"encoding/json";"fmt";"io";"os";"path/filepath";"github.com/minz/minzc/pkg/pipeline")
func must(e error){if e!=nil{panic(e)}}
func read(p string)[]byte{b,e:=os.ReadFile(p);if e==nil{return b};f,e:=os.Open(p+".gz");must(e);defer f.Close();z,e:=gzip.NewReader(f);must(e);defer z.Close();b,e=io.ReadAll(z);must(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
func main(){root:=os.Args[1];var e pipeline.MemoryHostBaselineEvidence;must(json.Unmarshal(read(filepath.Join(root,"factory-baseline.json")),&e));t:=e.Target;if t==nil||e.Raw==nil||e.Schema!="minz.memory-host-baseline/v1"{panic("schema/target/raw")}
 frames,e0:=json.Marshal(t.CallFrames);must(e0);if hash(frames)!=t.CallFramesSHA256{panic("typed call-frame SHA")};
 side,e2:=json.Marshal(t.Accesses);must(e2);if hash(side)!=t.SidecarSHA256{panic("typed sidecar SHA")}
 if hash([]byte(t.FinalMIRDump))!=t.MIRSHA256||!bytes.Equal(read(filepath.Join(root,"final.mir2")),[]byte(t.FinalMIRDump)){panic("MIR dump identity")}
 allocation:=read(filepath.Join(root,"allocation.json"));compact:=new(bytes.Buffer);must(json.Compact(compact,t.AllocationJSON));if !bytes.Equal(allocation,compact.Bytes())||hash(allocation)!=t.AllocationSHA256{panic("allocation raw bytes SHA")}
 if hash(t.Binary)!=t.BinarySHA256||!bytes.Equal(read(filepath.Join(root,"target.bin")),t.Binary)||!bytes.Equal(read(filepath.Join(root,"target.a80")),[]byte(t.Assembly)){panic("native BIN/ASM identity")}
 sum:=int64(0);for _,n:=range t.InvocationTStates{sum+=n};if sum!=t.TotalTStates||len(t.Invocations)!=30||len(e.Raw.Invocations)!=30||len(t.InvocationTStates)!=30{panic("calls/cost")}
 for _,r:=range t.Invocations{if r.Error!=""||r.Failure!=nil{panic("unexpected failed invocation")}}
 fmt.Printf("PASS typed sidecar SHA%s records%d; allocation SHA%s; MIR SHA%s; embedded/savedBIN SHA%s; savedASM identity; calls30 cost%dT/%dB\n",t.SidecarSHA256,len(t.Accesses),t.AllocationSHA256,t.MIRSHA256,t.BinarySHA256,t.TotalTStates,len(t.Binary))
}
