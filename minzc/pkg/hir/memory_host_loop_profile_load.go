package hir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// LoadMemoryHostLoopProfile accepts only this closed family, with all typed
// fields present, exact current source/workload context and complete calls.
// Measurements must additionally compare it with a newly observed run; JSON
// identity alone is not provenance for caller-supplied counts.
func LoadMemoryHostLoopProfile(r io.Reader, source *Module, w MemoryHostWorkload) (*MemoryHostLoopProfileRun, error) {
	data, e := io.ReadAll(io.LimitReader(r, maxLoopProfileBytes+1))
	if e != nil {
		return nil, e
	}
	if len(data) > maxLoopProfileBytes {
		return nil, fmt.Errorf("memory loop profile: size limit")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if e = checkJSONValue(d); e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, fmt.Errorf("memory loop profile: trailing JSON")
	}
	var p MemoryHostLoopProfileRun
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return nil, e
	}
	// Canonical typed shape equality also refuses omitted zero-valued fields and
	// null count arrays instead of silently synthesizing a complete profile.
	var got, want any
	shapeDecoder := json.NewDecoder(bytes.NewReader(data))
	shapeDecoder.UseNumber()
	if e = shapeDecoder.Decode(&got); e != nil {
		return nil, e
	}
	canonical, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	shapeDecoder = json.NewDecoder(bytes.NewReader(canonical))
	shapeDecoder.UseNumber()
	if e = shapeDecoder.Decode(&want); e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("memory loop profile: incomplete typed fields")
	}
	owned, ctx, e := PrepareMemoryHostWorkload(source, w)
	if e != nil {
		return nil, e
	}
	if _, e = BuildFunctionSiteMap(source); e != nil {
		return nil, e
	}
	ctx.SemanticsID += ":" + MemoryHostLoopProfileSemantics
	key, e := BuildProfileCompatibility(source, ctx)
	if e != nil {
		return nil, e
	}
	sites, e := BuildLoopSiteMap(source)
	if e != nil {
		return nil, e
	}
	if sites.UnmappedLoops != 0 || p.Schema != "minz.memory-host-loop-observations/v1" || p.Stage != "input-hir-instrumented-mir2-vm" || !reflect.DeepEqual(p.Workload, owned) || !reflect.DeepEqual(p.Context, ctx) || !reflect.DeepEqual(p.Compatibility, key) || !reflect.DeepEqual(p.Sites, sites) {
		return nil, fmt.Errorf("memory loop profile: incompatible family/source/workload")
	}
	if p.FailedCalls != 0 || p.CompletedCalls == 0 || p.CompletedCalls != uint64(len(owned.Calls)) || len(p.Invocations) != len(owned.Calls) || len(p.Counts) != len(sites.Sites) {
		return nil, fmt.Errorf("memory loop profile: incomplete or failed sample")
	}
	sums := make([]LoopCounts, len(sites.Sites))
	for i, s := range sites.Sites {
		sums[i].SiteID = s.ID
	}
	for _, inv := range p.Invocations {
		if !inv.Completed || inv.Observation.Error != "" || inv.Observation.Failure != nil || len(inv.Counts) != len(sums) {
			return nil, fmt.Errorf("memory loop profile: failed invocation")
		}
		for i, c := range inv.Counts {
			if c.SiteID != sums[i].SiteID || c.Entries == 0 && c.BodyEntries != 0 || c.Entries > ^uint64(0)-sums[i].Entries || c.BodyEntries > ^uint64(0)-sums[i].BodyEntries {
				return nil, fmt.Errorf("memory loop profile: invalid count delta")
			}
			sums[i].Entries += c.Entries
			sums[i].BodyEntries += c.BodyEntries
		}
	}
	if !reflect.DeepEqual(sums, p.Counts) {
		return nil, fmt.Errorf("memory loop profile: cumulative count mismatch")
	}
	return &p, nil
}
