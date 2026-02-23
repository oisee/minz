package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// Project represents the serializable state of an analysis session.
type Project struct {
	Version     string            `json:"version"`
	Filename    string            `json:"filename"`
	Origin      string            `json:"origin"`
	Platform    string            `json:"platform"`
	Labels      map[string]string `json:"labels"`       // "XXXX": "name"
	Comments    map[string]string `json:"comments"`      // "XXXX": "text"
	Overrides   map[string]string `json:"overrides"`     // "XXXX": "code" or "data"
	EntryPoints []string          `json:"entry_points"`
}

// SaveProject serializes the analysis state to a JSON project file.
func (a *Analysis) SaveProject(path string) error {
	p := &Project{
		Version:  "1",
		Origin:   fmt.Sprintf("%04X", a.Origin),
		Platform: a.Platform,
		Labels:   make(map[string]string),
		Comments: make(map[string]string),
		Overrides: make(map[string]string),
	}

	// Save labels (only user and platform — auto are regenerated)
	for addr, lbl := range a.Labels {
		if lbl.Source == "user" || lbl.Source == "platform" {
			p.Labels[fmt.Sprintf("%04X", addr)] = lbl.Name
		}
	}

	// Save comments
	for addr, comment := range a.Comments {
		p.Comments[fmt.Sprintf("%04X", addr)] = comment
	}

	// Save overrides
	for start, end := range a.CodeOverrides {
		p.Overrides[fmt.Sprintf("%04X-%04X", start, end)] = "code"
	}
	for start, end := range a.DataOverrides {
		p.Overrides[fmt.Sprintf("%04X-%04X", start, end)] = "data"
	}

	// Save entry points
	for _, ep := range a.EntryPoints {
		p.EntryPoints = append(p.EntryPoints, fmt.Sprintf("%04X", ep))
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadProject reads a project file and applies its state to an analysis.
func LoadProject(path string, binData []byte) (*Analysis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid project file: %w", err)
	}

	origin, err := strconv.ParseUint(p.Origin, 16, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid origin %q: %w", p.Origin, err)
	}

	a := NewAnalysis(binData, uint16(origin))
	a.Platform = p.Platform

	// Restore labels
	for addrStr, name := range p.Labels {
		addr, err := strconv.ParseUint(addrStr, 16, 16)
		if err != nil {
			continue
		}
		a.SetLabel(uint16(addr), name, "user")
	}

	// Restore comments
	for addrStr, comment := range p.Comments {
		addr, err := strconv.ParseUint(addrStr, 16, 16)
		if err != nil {
			continue
		}
		a.Comments[uint16(addr)] = comment
	}

	// Restore overrides
	for rangeStr, kind := range p.Overrides {
		var start, end uint16
		if _, err := fmt.Sscanf(rangeStr, "%04X-%04X", &start, &end); err != nil {
			continue
		}
		switch kind {
		case "code":
			a.CodeOverrides[start] = end
		case "data":
			a.DataOverrides[start] = end
		}
	}

	// Restore entry points
	for _, epStr := range p.EntryPoints {
		ep, err := strconv.ParseUint(epStr, 16, 16)
		if err != nil {
			continue
		}
		a.AddEntryPoint(uint16(ep))
	}

	return a, nil
}
