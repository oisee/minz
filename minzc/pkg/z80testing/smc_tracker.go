package z80testing

import (
	"fmt"
	"sort"
)

// SMCEvent represents a self-modifying code event
type SMCEvent struct {
	Cycle    int     // CPU cycle when modification occurred
	PC       uint16  // Program counter where write instruction executed
	Address  uint16  // Memory address that was modified
	OldValue byte    // Previous value at address
	NewValue byte    // New value written to address
	InCode   bool    // True if address is within code segment
}

// SMCTracker tracks self-modifying code events
type SMCTracker struct {
	events    []SMCEvent
	codeStart uint16
	codeEnd   uint16
	enabled   bool
}

// NewSMCTracker creates a new SMC tracker
func NewSMCTracker(codeStart, codeEnd uint16) *SMCTracker {
	return &SMCTracker{
		events:    make([]SMCEvent, 0),
		codeStart: codeStart,
		codeEnd:   codeEnd,
		enabled:   true,
	}
}

// Enable enables SMC tracking
func (t *SMCTracker) Enable() {
	t.enabled = true
}

// Disable disables SMC tracking
func (t *SMCTracker) Disable() {
	t.enabled = false
}

// Clear clears all tracked events
func (t *SMCTracker) Clear() {
	t.events = t.events[:0]
}

// TrackWrite tracks a memory write that might be SMC
func (t *SMCTracker) TrackWrite(pc uint16, address uint16, oldValue, newValue byte, cycle int) {
	if !t.enabled {
		return
	}

	// Always track writes, but mark if they're in code segment
	inCode := address >= t.codeStart && address <= t.codeEnd
	
	// Only track if value actually changed
	if oldValue != newValue {
		t.events = append(t.events, SMCEvent{
			Cycle:    cycle,
			PC:       pc,
			Address:  address,
			OldValue: oldValue,
			NewValue: newValue,
			InCode:   inCode,
		})
	}
}

// GetEvents returns all tracked SMC events
func (t *SMCTracker) GetEvents() []SMCEvent {
	return t.events
}

// GetCodeEvents returns only SMC events that modified code
func (t *SMCTracker) GetCodeEvents() []SMCEvent {
	var codeEvents []SMCEvent
	for _, event := range t.events {
		if event.InCode {
			codeEvents = append(codeEvents, event)
		}
	}
	return codeEvents
}

// EventCount returns the total number of tracked events
func (t *SMCTracker) EventCount() int {
	return len(t.events)
}

// CodeEventCount returns the number of code-modifying events
func (t *SMCTracker) CodeEventCount() int {
	count := 0
	for _, event := range t.events {
		if event.InCode {
			count++
		}
	}
	return count
}

// Summary returns a human-readable summary of SMC events
func (t *SMCTracker) Summary() string {
	if len(t.events) == 0 {
		return "No SMC events detected"
	}

	codeEvents := t.GetCodeEvents()
	
	summary := fmt.Sprintf("Total SMC Events: %d (Code: %d, Data: %d)\n",
		len(t.events), len(codeEvents), len(t.events)-len(codeEvents))
	
	if len(codeEvents) > 0 {
		summary += "\nCode Modifications:\n"
		for i, event := range codeEvents {
			summary += fmt.Sprintf("  [%d] Cycle %d: PC=%04X modified %04X from %02X to %02X\n",
				i+1, event.Cycle, event.PC, event.Address, event.OldValue, event.NewValue)
		}
	}
	
	return summary
}

// GetModifiedAddresses returns unique addresses that were modified
func (t *SMCTracker) GetModifiedAddresses() []uint16 {
	addressMap := make(map[uint16]bool)
	for _, event := range t.events {
		if event.InCode {
			addressMap[event.Address] = true
		}
	}
	
	addresses := make([]uint16, 0, len(addressMap))
	for addr := range addressMap {
		addresses = append(addresses, addr)
	}
	
	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i] < addresses[j]
	})
	
	return addresses
}

// GetEventsByAddress returns all events for a specific address
func (t *SMCTracker) GetEventsByAddress(address uint16) []SMCEvent {
	var events []SMCEvent
	for _, event := range t.events {
		if event.Address == address {
			events = append(events, event)
		}
	}
	return events
}

// SMCPattern represents a pattern of self-modifying code
type SMCPattern struct {
	Name        string
	Description string
	Matches     []SMCEvent
}

// DetectPatterns analyzes events to detect common SMC patterns
func (t *SMCTracker) DetectPatterns() []SMCPattern {
	patterns := []SMCPattern{}
	
	// Pattern 1: Parameter patching (modifying immediate values)
	immediateMods := []SMCEvent{}
	for _, event := range t.GetCodeEvents() {
		// Check if previous byte looks like an instruction that uses immediates
		if event.Address > t.codeStart {
			// This is simplified - real detection would check instruction bytes
			immediateMods = append(immediateMods, event)
		}
	}
	
	if len(immediateMods) > 0 {
		patterns = append(patterns, SMCPattern{
			Name:        "Immediate Parameter Patching",
			Description: "Modifying immediate values in instructions",
			Matches:     immediateMods,
		})
	}
	
	// Pattern 2: Jump target modification
	jumpMods := []SMCEvent{}
	for _, event := range t.GetCodeEvents() {
		// Check for modifications after jump instructions (0xC3, 0xC2, etc.)
		if event.Address > t.codeStart+1 {
			// Simplified check
			jumpMods = append(jumpMods, event)
		}
	}
	
	if len(jumpMods) > 0 {
		patterns = append(patterns, SMCPattern{
			Name:        "Jump Target Modification",
			Description: "Modifying jump/call addresses",
			Matches:     jumpMods,
		})
	}
	
	return patterns
}

// SMCStats provides statistics about SMC events
type SMCStats struct {
	TotalEvents      int
	CodeEvents       int
	DataEvents       int
	UniqueAddresses  int
	FirstEventCycle  int
	LastEventCycle   int
	CycleRange       int
	ModificationsPerAddress map[uint16]int
}

// GetStats returns statistics about SMC events
func (t *SMCTracker) GetStats() SMCStats {
	stats := SMCStats{
		TotalEvents:             len(t.events),
		ModificationsPerAddress: make(map[uint16]int),
	}
	
	if len(t.events) == 0 {
		return stats
	}
	
	stats.FirstEventCycle = t.events[0].Cycle
	stats.LastEventCycle = t.events[len(t.events)-1].Cycle
	stats.CycleRange = stats.LastEventCycle - stats.FirstEventCycle
	
	addressSet := make(map[uint16]bool)
	
	for _, event := range t.events {
		if event.InCode {
			stats.CodeEvents++
			addressSet[event.Address] = true
			stats.ModificationsPerAddress[event.Address]++
		} else {
			stats.DataEvents++
		}
	}
	
	stats.UniqueAddresses = len(addressSet)
	
	return stats
}