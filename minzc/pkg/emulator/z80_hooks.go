package emulator

// Z80Hooks provides hooks for intercepting Z80 operations
type Z80Hooks struct {
	// RST hooks - called when RST instruction is executed
	OnRST00 func()          // Reset
	OnRST08 func()          // Error
	OnRST10 func(a byte)    // Print character (RST 16)
	OnRST18 func() byte     // Collect character
	OnRST20 func() byte     // Collect next character
	OnRST28 func()          // Calculator
	OnRST30 func()          // Syntax tables
	OnRST38 func()          // Interrupt
	
	// I/O hooks
	OnIN  func(port byte) byte        // IN instruction
	OnOUT func(port byte, value byte) // OUT instruction
	
	// Memory hooks (for system variables)
	OnMemRead  func(addr uint16) byte
	OnMemWrite func(addr uint16, value byte)
	
	// Instruction hooks
	OnHalt func()
	OnStop func()
}

// Z80WithScreen extends Z80 with ZX Spectrum screen emulation
type Z80WithScreen struct {
	*Z80
	Screen *ZXScreen
	Hooks  *Z80Hooks
}

// NewZ80WithScreen creates a Z80 emulator with screen
func NewZ80WithScreen() *Z80WithScreen {
	z80 := &Z80WithScreen{
		Z80:    New(),
		Screen: NewZXScreen(),
		Hooks:  &Z80Hooks{},
	}
	
	// Set up default hooks for screen output
	z80.Hooks.OnRST10 = func(a byte) {
		z80.Screen.HandleRST16(a)
	}
	
	// Set up hook for keyboard input (RST 18)
	z80.Hooks.OnRST18 = func() byte {
		return z80.Screen.GetKey()
	}
	
	z80.Hooks.OnOUT = func(port byte, value byte) {
		z80.Screen.HandlePort(port, value, true)
	}
	
	z80.Hooks.OnMemWrite = func(addr uint16, value byte) {
		// Update screen system variables
		if addr >= SYSVAR_BASE && addr < SYSVAR_BASE+256 {
			z80.Screen.UpdateSysvar(addr, value)
		}
	}
	
	z80.Hooks.OnMemRead = func(addr uint16) byte {
		// Read screen system variables
		if addr >= SYSVAR_BASE && addr < SYSVAR_BASE+256 {
			return z80.Screen.GetSysvar(addr)
		}
		return z80.memory[addr]
	}
	
	return z80
}

// ExecuteWithHooks executes instructions with hook support
func (z *Z80WithScreen) ExecuteWithHooks(startPC uint16) (string, uint32) {
	z.PC = startPC
	cycles := uint32(0)
	
	for {
		opcode := z.memory[z.PC]
		z.PC++
		
		// Check for RST instructions
		switch opcode {
		case 0xC7: // RST 00
			if z.Hooks.OnRST00 != nil {
				z.Hooks.OnRST00()
			} else {
				z.push(z.PC)
				z.PC = 0x0000
			}
			cycles += 11
			
		case 0xCF: // RST 08
			if z.Hooks.OnRST08 != nil {
				z.Hooks.OnRST08()
			} else {
				z.push(z.PC)
				z.PC = 0x0008
			}
			cycles += 11
			
		case 0xD7: // RST 10 (RST 16)
			if z.Hooks.OnRST10 != nil {
				z.Hooks.OnRST10(z.A)
			} else {
				z.push(z.PC)
				z.PC = 0x0010
			}
			cycles += 11
			
		case 0xDF: // RST 18
			if z.Hooks.OnRST18 != nil {
				z.A = z.Hooks.OnRST18()
			} else {
				z.push(z.PC)
				z.PC = 0x0018
			}
			cycles += 11
			
		case 0xE7: // RST 20
			if z.Hooks.OnRST20 != nil {
				z.A = z.Hooks.OnRST20()
			} else {
				z.push(z.PC)
				z.PC = 0x0020
			}
			cycles += 11
			
		case 0xEF: // RST 28
			if z.Hooks.OnRST28 != nil {
				z.Hooks.OnRST28()
			} else {
				z.push(z.PC)
				z.PC = 0x0028
			}
			cycles += 11
			
		case 0xF7: // RST 30
			if z.Hooks.OnRST30 != nil {
				z.Hooks.OnRST30()
			} else {
				z.push(z.PC)
				z.PC = 0x0030
			}
			cycles += 11
			
		case 0xFF: // RST 38 (Interrupt)
			if z.Hooks.OnRST38 != nil {
				z.Hooks.OnRST38()
			} else {
				z.push(z.PC)
				z.PC = 0x0038
			}
			cycles += 11
			
		case 0xD3: // OUT (n), A
			port := z.memory[z.PC]
			z.PC++
			if z.Hooks.OnOUT != nil {
				z.Hooks.OnOUT(port, z.A)
			}
			cycles += 11
			
		case 0xDB: // IN A, (n)
			port := z.memory[z.PC]
			z.PC++
			if z.Hooks.OnIN != nil {
				z.A = z.Hooks.OnIN(port)
			}
			cycles += 11
			
		case 0x76: // HALT
			if z.Hooks.OnHalt != nil {
				z.Hooks.OnHalt()
			}
			
			// Check IFF1 for proper HALT semantics
			if !z.Z80.GetIFF1() {
				// Interrupts disabled - program termination
				z.Z80.SetHalted(true)
				return z.Screen.GetOutput(), cycles
			} else {
				// Interrupts enabled - wait for interrupt
				// For now, just continue (interrupt simulation handled in main loop)
				cycles += 4 // HALT instruction cycles
			}
			
		case 0xC9: // RET
			z.PC = z.pop()
			cycles += 10
			if z.PC == 0 { // End of program
				return z.Screen.GetOutput(), cycles
			}
			
		default:
			// Execute normal instruction
			z.PC-- // Back up since we already incremented
			z.step()
			cycles += 4 // Approximate cycles
		}
		
		// Safety check
		if cycles > 1000000 {
			break
		}
	}
	
	return z.Screen.GetOutput(), cycles
}

// WriteMemory writes to memory with hook support
func (z *Z80WithScreen) WriteMemory(addr uint16, value byte) {
	z.memory[addr] = value
	if z.Hooks.OnMemWrite != nil {
		z.Hooks.OnMemWrite(addr, value)
	}
}

// ReadMemory reads from memory with hook support
func (z *Z80WithScreen) ReadMemory(addr uint16) byte {
	if z.Hooks.OnMemRead != nil {
		return z.Hooks.OnMemRead(addr)
	}
	return z.memory[addr]
}

// PrintScreen prints the current screen state
func (z *Z80WithScreen) PrintScreen() {
	print(z.Screen.GetScreen())
}

// PrintCompactScreen prints only non-empty lines
func (z *Z80WithScreen) PrintCompactScreen() {
	print(z.Screen.GetCompactScreen())
}

// SendKeypress sends a keypress to the ZX Spectrum
func (z *Z80WithScreen) SendKeypress(key byte) {
	z.Screen.SendKey(key)
}

// SendString sends a string to the ZX Spectrum
func (z *Z80WithScreen) SendString(str string) {
	z.Screen.SendString(str)
}

// HasInput returns true if input is available
func (z *Z80WithScreen) HasInput() bool {
	return z.Screen.HasInput()
}