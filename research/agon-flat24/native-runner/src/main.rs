use ez80::{Cpu, Machine, Reg16};
struct Ram(Vec<u8>);
impl Machine for Ram {
    fn peek(&self, a: u32) -> u8 {
        self.0[a as usize]
    }
    fn poke(&mut self, a: u32, v: u8) {
        self.0[a as usize] = v;
    }
    fn use_cycles(&self, _: i32) {}
    fn port_in(&mut self, _: u16) -> u8 {
        panic!("unexpected I/O")
    }
    fn port_out(&mut self, _: u16, _: u8) {
        panic!("unexpected I/O")
    }
}
fn main() {
    let args: Vec<String> = std::env::args().collect();
    assert!(
        args.len() == 3 || (args.len() == 4 && args[3] == "--plain"),
        "usage: runner BINARY EXPECTED_A [--plain]"
    );
    let plain = args.len() == 4;
    let code = std::fs::read(&args[1]).unwrap();
    let expected: u8 = args[2].parse().unwrap();
    let mut ram = Ram(vec![0; 1 << 24]);
    ram.0[0x40045..0x40045 + code.len()].copy_from_slice(&code);
    // Deliberately distinguish a truncated address from the flat address.
    if !plain {
        ram.0[0] = 11;
        ram.0[0x50000] = expected;
    }
    let sp = 0xbfff0;
    ram.0[sp] = 0x00;
    ram.0[sp + 1] = 0x31;
    ram.0[sp + 2] = 0x00;
    let mut cpu = Cpu::new_ez80();
    cpu.set_adl(true);
    cpu.state.set_pc(0x40045);
    for reg in [Reg16::HL, Reg16::DE, Reg16::BC, Reg16::IX, Reg16::IY] {
        cpu.registers().set24(reg, 0xa5a5a5);
    }
    cpu.registers().set24(Reg16::SP, sp as u32);
    for step in 0..10000 {
        if cpu.registers().pc == 0x3100 {
            assert_eq!(
                cpu.registers().a(),
                expected,
                "wrong byte (address narrowed?)"
            );
            assert_eq!(
                cpu.registers().get24(Reg16::SP),
                sp as u32 + 3,
                "unbalanced ADL stack"
            );
            println!(
                "ADL native: A={} instructions={} stack balanced",
                expected, step
            );
            return;
        }
        cpu.execute_instruction(&mut ram);
    }
    panic!("instruction limit reached at {:06x}", cpu.registers().pc);
}
