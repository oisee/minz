package nanz

import (
	"fmt"
	"sort"
	"strings"
)

// MetaVariant is a generator-owned finite set of semantically equivalent
// implementations. Declaration alone is not a proof of that equivalence.
type MetaVariant struct {
	Key          string `json:"key"`
	Alternatives uint8  `json:"alternatives"`
	Default      uint8  `json:"default"`
}

// MetaVariantContext belongs to one parse, including imported and generated
// code. Selection is copied on construction; declarations are returned by value.
// Use a fresh context for every independent candidate parse.
type MetaVariantContext struct {
	selection map[string]uint8
	declared  map[string]MetaVariant
}

func NewMetaVariantContext(selection map[string]uint8) *MetaVariantContext {
	c := &MetaVariantContext{selection: map[string]uint8{}, declared: map[string]MetaVariant{}}
	for k, v := range selection {
		c.selection[k] = v
	}
	return c
}

func (c *MetaVariantContext) Variants() []MetaVariant {
	out := make([]MetaVariant, 0, len(c.declared))
	for _, v := range c.declared {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Validate rejects selections that the actual parse never requested.
func (c *MetaVariantContext) Validate() error {
	for k := range c.selection {
		if _, ok := c.declared[k]; !ok {
			return fmt.Errorf("meta variant: unused selection %q", k)
		}
	}
	return nil
}

func (c *MetaVariantContext) choose(key string, alternatives, fallback int64) (uint8, error) {
	if len(key) == 0 || len(key) > 128 || strings.ContainsAny(key, " \t\r\n") {
		return 0, fmt.Errorf("meta variant: invalid key %q", key)
	}
	if alternatives < 2 || alternatives > 32 || fallback < 0 || fallback >= alternatives {
		return 0, fmt.Errorf("meta variant %q: need 2..32 alternatives and an in-range default", key)
	}
	v := MetaVariant{key, uint8(alternatives), uint8(fallback)}
	if c == nil {
		return v.Default, nil
	}
	if old, ok := c.declared[key]; ok && old != v {
		return 0, fmt.Errorf("meta variant %q: conflicting declaration", key)
	}
	c.declared[key] = v
	selected, ok := c.selection[key]
	if !ok {
		selected = v.Default
	}
	if selected >= v.Alternatives {
		return 0, fmt.Errorf("meta variant %q: selection %d out of range", key, selected)
	}
	return selected, nil
}
