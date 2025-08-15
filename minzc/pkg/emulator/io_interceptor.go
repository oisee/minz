package emulator

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// IOInterceptor handles file I/O interception for various platforms
type IOInterceptor struct {
	enabled     bool
	tapDir      string
	fddDir      string
	cpmDir      string
	logging     bool
	fileHandles map[byte]*os.File
	nextHandle  byte
}

// NewIOInterceptor creates a new I/O interceptor
func NewIOInterceptor() *IOInterceptor {
	return &IOInterceptor{
		enabled:     true,
		tapDir:      "./tap",
		fddDir:      "./fdd",
		cpmDir:      "./cpm",
		fileHandles: make(map[byte]*os.File),
		nextHandle:  1,
	}
}

// InterceptCall checks if a CALL instruction should be intercepted
func (io *IOInterceptor) InterceptCall(cpu *Z80, addr uint16) bool {
	if !io.enabled {
		return false
	}

	switch addr {
	// ZX Spectrum ROM calls
	case 0x04C2: // SA-BYTES (tape save)
		return io.interceptTapeSave(cpu)
	case 0x0556: // LD-BYTES (tape load)
		return io.interceptTapeLoad(cpu)
	case 0x3D13: // TR-DOS entry point
		return io.interceptTRDOS(cpu)
		
	// CP/M and MSX-DOS BDOS entry
	case 0x0005:
		return io.interceptBDOS(cpu)
	}
	
	return false
}

// interceptTapeSave handles ZX Spectrum tape save
func (io *IOInterceptor) interceptTapeSave(cpu *Z80) bool {
	// Get parameters from registers
	// IX = header address
	// DE = length
	// A = block type (0=header, 0xFF=data)
	
	headerAddr := cpu.GetIX()
	length := cpu.GetDE()
	blockType := cpu.A
	
	if io.logging {
		fmt.Printf("TAPE SAVE: addr=%04X len=%04X type=%02X\n", 
			headerAddr, length, blockType)
	}
	
	// Extract filename from header (bytes 1-10)
	filename := io.extractTapeName(cpu, headerAddr)
	
	// Get data to save
	data := make([]byte, length)
	for i := uint16(0); i < length; i++ {
		data[i] = cpu.ReadMemory(headerAddr+i)
	}
	
	// Save to host filesystem
	tapPath := filepath.Join(io.tapDir, filename+".tap")
	if err := os.MkdirAll(io.tapDir, 0755); err != nil {
		cpu.SetCarryFlag(true) // Error
		return true
	}
	
	if err := ioutil.WriteFile(tapPath, data, 0644); err != nil {
		cpu.SetCarryFlag(true) // Error
		return true
	}
	
	if io.logging {
		fmt.Printf("Saved %d bytes to %s\n", length, tapPath)
	}
	
	cpu.SetCarryFlag(false) // Success
	return true
}

// interceptTapeLoad handles ZX Spectrum tape load
func (io *IOInterceptor) interceptTapeLoad(cpu *Z80) bool {
	// Get parameters
	// IX = destination address
	// DE = expected length
	// A = block type
	// Carry flag = LOAD (set) or VERIFY (clear)
	
	destAddr := cpu.GetIX()
	expectedLen := cpu.GetDE()
	blockType := cpu.A
	isLoad := cpu.GetCarryFlag()
	
	if io.logging {
		fmt.Printf("TAPE LOAD: addr=%04X len=%04X type=%02X load=%v\n",
			destAddr, expectedLen, blockType, isLoad)
	}
	
	// For simplicity, load the first .tap file found
	// In real implementation, would show a file selector
	files, err := ioutil.ReadDir(io.tapDir)
	if err != nil {
		cpu.SetCarryFlag(true) // Error
		return true
	}
	
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".tap") {
			tapPath := filepath.Join(io.tapDir, file.Name())
			data, err := ioutil.ReadFile(tapPath)
			if err != nil {
				continue
			}
			
			// Load data to memory
			if isLoad {
				for i := 0; i < len(data) && i < int(expectedLen); i++ {
					cpu.WriteMemory(destAddr+uint16(i), data[i])
				}
			}
			
			if io.logging {
				fmt.Printf("Loaded %d bytes from %s\n", len(data), tapPath)
			}
			
			cpu.SetCarryFlag(false) // Success
			de := uint16(len(data)) // Actual length loaded
			cpu.D = uint8(de >> 8)
			cpu.E = uint8(de & 0xFF)
			return true
		}
	}
	
	cpu.SetCarryFlag(true) // Error - no file found
	return true
}

// interceptTRDOS handles TR-DOS disk operations
func (io *IOInterceptor) interceptTRDOS(cpu *Z80) bool {
	function := cpu.C
	
	if io.logging {
		fmt.Printf("TR-DOS: function=%02X\n", function)
	}
	
	switch function {
	case 0x00: // Initialize/Reset
		cpu.A = 0 // Success
		return true
		
	case 0x0A: // Open file
		return io.trdosOpen(cpu)
		
	case 0x0B: // Read byte
		return io.trdosRead(cpu)
		
	case 0x0C: // Write byte
		return io.trdosWrite(cpu)
		
	case 0x0D: // Close file
		return io.trdosClose(cpu)
		
	case 0x0E: // Find file
		return io.trdosFind(cpu)
	}
	
	return false
}

// interceptBDOS handles CP/M and MSX-DOS BDOS calls
func (io *IOInterceptor) interceptBDOS(cpu *Z80) bool {
	function := cpu.C
	
	if io.logging {
		fmt.Printf("BDOS: function=%02X\n", function)
	}
	
	switch function {
	case 0x0F: // Open file
		return io.bdosOpen(cpu)
		
	case 0x10: // Close file
		return io.bdosClose(cpu)
		
	case 0x14: // Read sequential
		return io.bdosRead(cpu)
		
	case 0x15: // Write sequential
		return io.bdosWrite(cpu)
		
	case 0x16: // Make file
		return io.bdosMake(cpu)
		
	case 0x11: // Search first
		return io.bdosSearchFirst(cpu)
		
	case 0x12: // Search next
		return io.bdosSearchNext(cpu)
	}
	
	return false
}

// Helper functions

func (io *IOInterceptor) extractTapeName(cpu *Z80, addr uint16) string {
	name := make([]byte, 10)
	for i := 0; i < 10; i++ {
		b := cpu.ReadMemory(addr+1+uint16(i))
		if b == 0 || b == 0x20 {
			break
		}
		name[i] = b
	}
	return strings.TrimSpace(string(name))
}

func (io *IOInterceptor) getFCBName(cpu *Z80, fcbAddr uint16) string {
	// FCB structure:
	// +0: Drive (0=default, 1=A:, 2=B:, etc)
	// +1: Filename (8 bytes, space padded)
	// +9: Extension (3 bytes, space padded)
	
	filename := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b := cpu.ReadMemory(fcbAddr+1+uint16(i))
		if b == 0x20 {
			break
		}
		filename[i] = b
	}
	
	extension := make([]byte, 3)
	for i := 0; i < 3; i++ {
		b := cpu.ReadMemory(fcbAddr+9+uint16(i))
		if b == 0x20 {
			break
		}
		extension[i] = b
	}
	
	name := strings.TrimSpace(string(filename))
	ext := strings.TrimSpace(string(extension))
	
	if ext != "" {
		return name + "." + ext
	}
	return name
}

// BDOS file operations

func (io *IOInterceptor) bdosOpen(cpu *Z80) bool {
	fcbAddr := cpu.GetDE()
	filename := io.getFCBName(cpu, fcbAddr)
	
	filepath := filepath.Join(io.cpmDir, filename)
	file, err := os.Open(filepath)
	if err != nil {
		cpu.A = 0xFF // Error
		return true
	}
	
	handle := io.nextHandle
	io.fileHandles[handle] = file
	io.nextHandle++
	
	// Store handle in FCB
	cpu.WriteMemory(fcbAddr+32, handle)
	cpu.A = 0 // Success
	
	if io.logging {
		fmt.Printf("Opened %s as handle %d\n", filename, handle)
	}
	
	return true
}

func (io *IOInterceptor) bdosClose(cpu *Z80) bool {
	fcbAddr := cpu.GetDE()
	handle := cpu.ReadMemory(fcbAddr+32)
	
	if file, ok := io.fileHandles[handle]; ok {
		file.Close()
		delete(io.fileHandles, handle)
		cpu.A = 0 // Success
	} else {
		cpu.A = 0xFF // Error
	}
	
	return true
}

func (io *IOInterceptor) bdosRead(cpu *Z80) bool {
	fcbAddr := cpu.GetDE()
	handle := cpu.ReadMemory(fcbAddr+32)
	dmaAddr := uint16(0x0080) // Default DMA address
	
	if file, ok := io.fileHandles[handle]; ok {
		buffer := make([]byte, 128) // CP/M sector size
		n, err := file.Read(buffer)
		if err != nil {
			cpu.A = 1 // End of file
			return true
		}
		
		// Copy to DMA buffer
		for i := 0; i < n; i++ {
			cpu.WriteMemory(dmaAddr+uint16(i), buffer[i])
		}
		
		cpu.A = 0 // Success
	} else {
		cpu.A = 0xFF // Error
	}
	
	return true
}

func (io *IOInterceptor) bdosWrite(cpu *Z80) bool {
	fcbAddr := cpu.GetDE()
	handle := cpu.ReadMemory(fcbAddr+32)
	dmaAddr := uint16(0x0080) // Default DMA address
	
	if file, ok := io.fileHandles[handle]; ok {
		buffer := make([]byte, 128)
		for i := 0; i < 128; i++ {
			buffer[i] = cpu.ReadMemory(dmaAddr+uint16(i))
		}
		
		_, err := file.Write(buffer)
		if err != nil {
			cpu.A = 0xFF // Error
		} else {
			cpu.A = 0 // Success
		}
	} else {
		cpu.A = 0xFF // Error
	}
	
	return true
}

func (io *IOInterceptor) bdosMake(cpu *Z80) bool {
	fcbAddr := cpu.GetDE()
	filename := io.getFCBName(cpu, fcbAddr)
	
	filepath := filepath.Join(io.cpmDir, filename)
	os.MkdirAll(io.cpmDir, 0755)
	
	file, err := os.Create(filepath)
	if err != nil {
		cpu.A = 0xFF // Error
		return true
	}
	
	handle := io.nextHandle
	io.fileHandles[handle] = file
	io.nextHandle++
	
	// Store handle in FCB
	cpu.WriteMemory(fcbAddr+32, handle)
	cpu.A = 0 // Success
	
	if io.logging {
		fmt.Printf("Created %s as handle %d\n", filename, handle)
	}
	
	return true
}

// Directory search functions (simplified)
var searchIndex int
var searchFiles []os.FileInfo

func (io *IOInterceptor) bdosSearchFirst(cpu *Z80) bool {
	searchFiles, _ = ioutil.ReadDir(io.cpmDir)
	searchIndex = 0
	return io.bdosSearchNext(cpu)
}

func (io *IOInterceptor) bdosSearchNext(cpu *Z80) bool {
	if searchIndex >= len(searchFiles) {
		cpu.A = 0xFF // No more files
		return true
	}
	
	file := searchFiles[searchIndex]
	searchIndex++
	
	// Return filename in DMA buffer (simplified)
	dmaAddr := uint16(0x0080)
	name := file.Name()
	for i := 0; i < len(name) && i < 11; i++ {
		cpu.WriteMemory(dmaAddr+uint16(i), name[i])
	}
	
	cpu.A = 0 // Success
	return true
}

// TR-DOS operations (simplified stubs)

func (io *IOInterceptor) trdosOpen(cpu *Z80) bool {
	// Simplified - would need full implementation
	cpu.A = 0 // Success
	return true
}

func (io *IOInterceptor) trdosRead(cpu *Z80) bool {
	cpu.A = 0 // Byte read
	return true
}

func (io *IOInterceptor) trdosWrite(cpu *Z80) bool {
	return true
}

func (io *IOInterceptor) trdosClose(cpu *Z80) bool {
	return true
}

func (io *IOInterceptor) trdosFind(cpu *Z80) bool {
	cpu.SetCarryFlag(false) // Found
	return true
}