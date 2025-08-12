# TODO for MinZ v0.13.0 - Complete I/O System

## ✅ Completed (Design Phase)

### CTIE System (v0.12.0) ✅
- [x] ADR for Compile-Time Interface Execution
- [x] Purity analyzer implementation
- [x] MIR interpreter for compile-time execution  
- [x] Const tracker for finding optimizable calls
- [x] Integration with compiler pipeline
- [x] Comprehensive testing (100% success rate)
- [x] Documentation and announcement

### I/O System Design ✅
- [x] File I/O via ROM/BDOS interception design
- [x] AY-3-8912 sound chip integration design
- [x] Keyboard dual-mode system design
- [x] TCP/IP networking without port conflicts
- [x] MZE interceptor architecture
- [x] Complete documentation (docs 185-190)

## 🚧 In Progress

### MZE Interceptor Framework
- [ ] Core interceptor chain in emulator
- [ ] Port I/O routing system
- [ ] Command-line flags for features
- [ ] Logging and debugging support

## 📋 TODO for v0.13.0

### Week 1-2: Foundation
- [ ] Complete MZE interceptor framework
- [ ] Add port interception to CPU loop
- [ ] Test basic IN/OUT interception

### Week 3: Compiler Support
- [ ] Implement `@platform` directive parser
- [ ] Add module import resolution
- [ ] Support inline assembly blocks
- [ ] Platform-specific code generation

### Week 4: File I/O Modules
- [ ] Implement zx.io module (tape/disk)
- [ ] Implement cpm.io module (BDOS)
- [ ] Create msx.io wrapper
- [ ] Test file operations

### Week 5: Sound System
- [ ] Integrate Ayumi emulator library
- [ ] Implement AY port interceptor
- [ ] Create zx.ay module
- [ ] Build sound effects library

### Week 6: Keyboard System
- [ ] Implement matrix scanning
- [ ] Add enhanced buffer mode
- [ ] Terminal integration
- [ ] Test with games

### Week 7-8: Networking
- [ ] Implement TCP interceptor
- [ ] Bridge to host networking
- [ ] Create net.tcp module
- [ ] HTTP client library

### Week 9: Testing & Polish
- [ ] E2E test suite
- [ ] Performance benchmarks
- [ ] Documentation updates
- [ ] Release preparation

## 📊 Success Metrics

### Functionality
- [ ] All I/O operations work in MZE
- [ ] No port conflicts on any platform
- [ ] Same code runs on hardware/emulator

### Performance
- [ ] Zero overhead for native operations
- [ ] <5ms latency for network operations
- [ ] Real-time sound generation

### Quality
- [ ] >80% test coverage
- [ ] All examples compile and run
- [ ] Documentation complete

## 🎯 Stretch Goals

### If Time Permits
- [ ] WebSocket support
- [ ] UDP for games
- [ ] MIDI via AY
- [ ] Serial port support
- [ ] Joystick input

## 📈 Progress Tracking

```
Design:          ████████████████████ 100%
Specifications:  ████████████████████ 100%
Implementation:  ████░░░░░░░░░░░░░░░░ 20%
Testing:         ██░░░░░░░░░░░░░░░░░░ 10%
Documentation:   ████████████████░░░░ 80%
```

## 🚀 Release Checklist for v0.13.0

### Must Have
- [ ] File I/O working on ZX/CP/M/MSX
- [ ] Sound generation via AY
- [ ] Keyboard input (both modes)
- [ ] Basic TCP client
- [ ] MZE fully integrated
- [ ] Documentation complete

### Nice to Have
- [ ] HTTP client library
- [ ] Sound effects library
- [ ] Network game example
- [ ] Music player demo

## 📝 Notes

### Key Insights
1. **MSX-DOS = CP/M 2.2** - One module serves both!
2. **Port 0x9000+** - Safe for networking
3. **Ayumi emulator** - Cycle-perfect AY sound
4. **ROM interception** - Transparent file I/O

### Risks
1. MZE integration complexity
2. Platform detection in compiler
3. Module system not yet implemented
4. Testing on real hardware

### Dependencies
- Ayumi sound emulator (Go port needed?)
- Network libraries for Go
- Audio output library (PortAudio?)

## 🎉 When Complete

MinZ v0.13.0 will be the first 8-bit language with:
- Complete I/O system
- TCP/IP networking
- Modern file operations
- Professional sound
- All with zero-cost abstractions!

---

**Last Updated**: August 2025
**Target Release**: Q1 2025
**Status**: Ready for implementation!