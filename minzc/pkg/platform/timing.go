package platform

// FrameTiming contains platform-specific interrupt and frame timing information
type FrameTiming struct {
	CyclesPerFrame   int     // T-states between interrupts
	FrameRate        float64 // Hz (frames per second)
	UsableCycles     int     // Cycles available after display overhead
	HasContention    bool    // Whether memory contention affects timing
	ContentionLoss   float64 // Percentage of cycles lost to contention (0.0-1.0)
}

// PlatformTimings defines frame timing for each supported platform
var PlatformTimings = map[string]FrameTiming{
	// Original Spectrum with ULA contention
	"spectrum": {
		CyclesPerFrame:   69888,
		FrameRate:        50.0,
		UsableCycles:     42000, // ~40% lost to contention
		HasContention:    true,
		ContentionLoss:   0.4,
	},
	"zxspectrum": { // Alias
		CyclesPerFrame:   69888,
		FrameRate:        50.0,
		UsableCycles:     42000,
		HasContention:    true,
		ContentionLoss:   0.4,
	},
	
	// Pentagon - slightly more cycles, no contention!
	"pentagon": {
		CyclesPerFrame:   71680,
		FrameRate:        48.828125,
		UsableCycles:     71680, // All cycles usable!
		HasContention:    false,
		ContentionLoss:   0.0,
	},
	
	// Scorpion - switchable timing
	"scorpion": {
		CyclesPerFrame:   69888, // Can also be 139776 in turbo
		FrameRate:        50.0,
		UsableCycles:     69888, // No contention
		HasContention:    false,
		ContentionLoss:   0.0,
	},
	
	// ATM Turbo - multiple speeds
	"atm": {
		CyclesPerFrame:   71680, // Base mode, up to 286720 at 14MHz
		FrameRate:        48.828125,
		UsableCycles:     71680,
		HasContention:    false,
		ContentionLoss:   0.0,
	},
	
	// Kay-1024 - Pentagon compatible timing
	"kay": {
		CyclesPerFrame:   71680,
		FrameRate:        48.828125,
		UsableCycles:     71680,
		HasContention:    false,
		ContentionLoss:   0.0,
	},
	
	// Profi - Spectrum timing but no contention
	"profi": {
		CyclesPerFrame:   69888,
		FrameRate:        50.0,
		UsableCycles:     69888,
		HasContention:    false,
		ContentionLoss:   0.0,
	},
	
	// Timex - PAL version
	"timex": {
		CyclesPerFrame:   69888,
		FrameRate:        50.0,
		UsableCycles:     65000, // Some overhead for extended modes
		HasContention:    false,
		ContentionLoss:   0.07,
	},
	
	// Timex TC2068 - NTSC version
	"timex_ntsc": {
		CyclesPerFrame:   59736,
		FrameRate:        60.0,
		UsableCycles:     55000,
		HasContention:    false,
		ContentionLoss:   0.08,
	},
	
	// SAM Coupé - 6MHz Z80B
	"sam": {
		CyclesPerFrame:   125334,
		FrameRate:        50.08,
		UsableCycles:     113000, // Mode-dependent contention
		HasContention:    true,    // In some graphics modes
		ContentionLoss:   0.1,
	},
	
	// MSX machines (rough average)
	"msx": {
		CyclesPerFrame:   71364, // PAL MSX
		FrameRate:        50.0,
		UsableCycles:     65000,
		HasContention:    false,
		ContentionLoss:   0.09, // VDP access overhead
	},
	
	// MSX2 with faster VDP
	"msx2": {
		CyclesPerFrame:   71364,
		FrameRate:        50.0,
		UsableCycles:     68000,
		HasContention:    false,
		ContentionLoss:   0.05,
	},
	
	// Amstrad CPC
	"amstrad": {
		CyclesPerFrame:   79872, // 4MHz Z80
		FrameRate:        50.0,
		UsableCycles:     72000,
		HasContention:    true, // Gate Array wait states
		ContentionLoss:   0.1,
	},
	
	// CP/M systems (generic, no video)
	"cpm": {
		CyclesPerFrame:   70000, // Approximate, no fixed frame rate
		FrameRate:        0.0,    // No video interrupts
		UsableCycles:     70000,
		HasContention:    false,
		ContentionLoss:   0.0,
	},
}

// GetFrameBudget returns the recommended cycle budget for smooth animation
func GetFrameBudget(platform string, targetFPS int) int {
	timing, exists := PlatformTimings[platform]
	if !exists {
		// Default to Spectrum timing
		timing = PlatformTimings["spectrum"]
	}
	
	// Calculate how many frames we can skip
	framesPerUpdate := int(timing.FrameRate) / targetFPS
	if framesPerUpdate < 1 {
		framesPerUpdate = 1
	}
	
	// Return total cycles available
	return timing.UsableCycles * framesPerUpdate
}

// GetScanlineCycles returns T-states per scanline for the platform
func GetScanlineCycles(platform string) int {
	timing, exists := PlatformTimings[platform]
	if !exists {
		return 224 // Default Spectrum value
	}
	
	// Calculate from frame timing
	switch platform {
	case "spectrum", "zxspectrum":
		return 224 // 69888 / 312 scanlines
	case "pentagon", "kay", "atm":
		return 224 // 71680 / 320 scanlines  
	case "sam":
		return 384 // Wider scanlines
	default:
		return 224
	}
}

// IsTurboCapable returns whether platform supports turbo modes
func IsTurboCapable(platform string) bool {
	switch platform {
	case "scorpion", "atm", "profi":
		return true
	default:
		return false
	}
}

// GetTurboMultiplier returns the speed multiplier in turbo mode
func GetTurboMultiplier(platform string) float64 {
	switch platform {
	case "scorpion", "profi":
		return 2.0 // 7MHz
	case "atm":
		return 4.0 // Up to 14MHz on ATM2
	default:
		return 1.0
	}
}