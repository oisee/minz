package z80testing

import (
	"testing"
)

func TestSMCTracker(t *testing.T) {
	tracker := NewSMCTracker(0x8000, 0x9000)

	// Test basic tracking
	t.Run("BasicTracking", func(t *testing.T) {
		tracker.Clear()
		
		// Track a write to code area
		tracker.TrackWrite(0x8000, 0x8010, 0x00, 0x3E, 100)
		
		events := tracker.GetEvents()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		
		if events[0].Address != 0x8010 {
			t.Errorf("Wrong address: got %04X, want 8010", events[0].Address)
		}
		
		if !events[0].InCode {
			t.Error("Event should be marked as in code")
		}
	})

	// Test filtering code events
	t.Run("CodeEventFiltering", func(t *testing.T) {
		tracker.Clear()
		
		// Track writes to code and data areas
		tracker.TrackWrite(0x8000, 0x8100, 0x00, 0xFF, 100) // In code
		tracker.TrackWrite(0x8001, 0x4000, 0x00, 0xFF, 101) // In data
		tracker.TrackWrite(0x8002, 0x8200, 0x00, 0xFF, 102) // In code
		
		allEvents := tracker.GetEvents()
		codeEvents := tracker.GetCodeEvents()
		
		if len(allEvents) != 3 {
			t.Errorf("Expected 3 total events, got %d", len(allEvents))
		}
		
		if len(codeEvents) != 2 {
			t.Errorf("Expected 2 code events, got %d", len(codeEvents))
		}
	})

	// Test no-change filtering
	t.Run("NoChangeFiltering", func(t *testing.T) {
		tracker.Clear()
		
		// Track write that doesn't change value
		tracker.TrackWrite(0x8000, 0x8100, 0xFF, 0xFF, 100)
		
		events := tracker.GetEvents()
		if len(events) != 0 {
			t.Error("Should not track writes that don't change value")
		}
	})

	// Test address tracking
	t.Run("AddressTracking", func(t *testing.T) {
		tracker.Clear()
		
		// Multiple writes to same address
		tracker.TrackWrite(0x8000, 0x8100, 0x00, 0x01, 100)
		tracker.TrackWrite(0x8001, 0x8100, 0x01, 0x02, 200)
		tracker.TrackWrite(0x8002, 0x8200, 0x00, 0x03, 300)
		
		addresses := tracker.GetModifiedAddresses()
		if len(addresses) != 2 {
			t.Errorf("Expected 2 unique addresses, got %d", len(addresses))
		}
		
		events := tracker.GetEventsByAddress(0x8100)
		if len(events) != 2 {
			t.Errorf("Expected 2 events for address 8100, got %d", len(events))
		}
	})
}

func TestSMCMemory(t *testing.T) {
	// Create SMC memory
	smcMem := NewSMCMemory(0x8000, 0x9000)
	
	// Mock CPU state
	mockCPU := &MockCPUState{pc: 0x8000, tstates: 100}
	smcMem.SetCPU(mockCPU)

	t.Run("TrackingWrites", func(t *testing.T) {
		smcMem.ClearSMCEvents()
		
		// Write to code area
		smcMem.WriteByte(0x8100, 0x3E) // LD A, immediate
		mockCPU.tstates = 104
		smcMem.WriteByte(0x8101, 0x42) // immediate value 42
		
		events := smcMem.GetSMCEvents()
		if len(events) != 2 {
			t.Errorf("Expected 2 events, got %d", len(events))
		}
		
		// Check the summary
		summary := smcMem.GetSMCSummary()
		if summary == "No SMC events detected" {
			t.Error("Should have SMC events in summary")
		}
	})

	t.Run("DisableTracking", func(t *testing.T) {
		smcMem.ClearSMCEvents()
		smcMem.DisableSMCTracking()
		
		// Write should not be tracked
		smcMem.WriteByte(0x8200, 0xFF)
		
		events := smcMem.GetSMCEvents()
		if len(events) != 0 {
			t.Error("Should not track when disabled")
		}
		
		// Re-enable and verify tracking works
		smcMem.EnableSMCTracking()
		smcMem.WriteByte(0x8201, 0xFE)
		
		events = smcMem.GetSMCEvents()
		if len(events) != 1 {
			t.Error("Should track when re-enabled")
		}
	})
}

// MockCPUState implements CPUState for testing
type MockCPUState struct {
	pc      uint16
	tstates int
}

func (m *MockCPUState) PC() uint16 {
	return m.pc
}

func (m *MockCPUState) Tstates() int {
	return m.tstates
}