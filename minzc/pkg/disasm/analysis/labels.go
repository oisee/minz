package analysis

import (
	"fmt"
	"sort"
	"strings"
)

// AutoLabel generates labels for all known addresses from analysis results.
// Priority: user > platform > auto-generated.
func (a *Analysis) AutoLabel() {
	// Generate auto-labels for functions
	for addr, fn := range a.Functions {
		if _, exists := a.Labels[addr]; !exists {
			a.Labels[addr] = &Label{
				Addr:   addr,
				Name:   fn.Name,
				Source: "auto",
			}
		}
	}

	// Generate auto-labels for jump targets that aren't function entries
	for addr, refs := range a.XRefsTo {
		if _, exists := a.Labels[addr]; exists {
			continue
		}
		for _, ref := range refs {
			if ref.Type == XRefJump || ref.Type == XRefCondJump {
				a.Labels[addr] = &Label{
					Addr:   addr,
					Name:   fmt.Sprintf("loc_%04X", addr),
					Source: "auto",
				}
				break
			}
		}
	}

	// Generate auto-labels for detected strings
	for addr := range a.Strings {
		if _, exists := a.Labels[addr]; exists {
			continue
		}
		a.Labels[addr] = &Label{
			Addr:   addr,
			Name:   fmt.Sprintf("str_%04X", addr),
			Source: "auto",
		}
	}

	// Generate auto-labels for data references
	for addr, refs := range a.XRefsTo {
		if _, exists := a.Labels[addr]; exists {
			continue
		}
		for _, ref := range refs {
			if ref.Type == XRefRead || ref.Type == XRefWrite {
				a.Labels[addr] = &Label{
					Addr:   addr,
					Name:   fmt.Sprintf("dat_%04X", addr),
					Source: "auto",
				}
				break
			}
		}
	}

	// RST/interrupt vectors
	vectorNames := map[uint16]string{
		0x0000: "vec_RST00",
		0x0008: "vec_RST08",
		0x0010: "vec_RST10",
		0x0018: "vec_RST18",
		0x0020: "vec_RST20",
		0x0028: "vec_RST28",
		0x0030: "vec_RST30",
		0x0038: "vec_RST38",
		0x0066: "vec_NMI",
	}
	for addr, name := range vectorNames {
		lbl, exists := a.Labels[addr]
		if !exists && a.IsCode(addr) {
			a.Labels[addr] = &Label{
				Addr:   addr,
				Name:   name,
				Source: "auto",
			}
		} else if exists && lbl.Source == "auto" {
			// Upgrade auto label to vector name if it's just sub_XXXX
			if strings.HasPrefix(lbl.Name, "sub_") {
				lbl.Name = name
			}
		}
	}
}

// SetLabel sets a label at the given address with the given source priority.
// Higher priority sources override lower: user > platform > auto.
func (a *Analysis) SetLabel(addr uint16, name, source string) {
	existing, ok := a.Labels[addr]
	if ok {
		if sourcePriority(source) >= sourcePriority(existing.Source) {
			existing.Name = name
			existing.Source = source
		}
		return
	}
	a.Labels[addr] = &Label{
		Addr:   addr,
		Name:   name,
		Source: source,
	}
}

// GetLabel returns the label name for an address, or empty string.
func (a *Analysis) GetLabel(addr uint16) string {
	if lbl, ok := a.Labels[addr]; ok {
		return lbl.Name
	}
	return ""
}

// FormatOperand returns a label name if one exists, otherwise $XXXX.
func (a *Analysis) FormatOperand(addr uint16) string {
	if name := a.GetLabel(addr); name != "" {
		return name
	}
	return fmt.Sprintf("$%04X", addr)
}

// GetSortedLabels returns all labels sorted by address.
func (a *Analysis) GetSortedLabels() []*Label {
	labels := make([]*Label, 0, len(a.Labels))
	for _, lbl := range a.Labels {
		labels = append(labels, lbl)
	}
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Addr < labels[j].Addr
	})
	return labels
}

func sourcePriority(source string) int {
	switch source {
	case "user":
		return 3
	case "platform":
		return 2
	case "auto":
		return 1
	default:
		return 0
	}
}
