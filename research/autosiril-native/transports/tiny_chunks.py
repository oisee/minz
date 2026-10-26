"""Test binding: split each physical CP/M record without changing its bytes."""

TINY_READ = '''
global contract_buffer:[u8;128]
global contract_at:u16
global contract_length:u16
fun read_chunk()->u32 {
    if contract_at == contract_length {
        let result:u32 = physical_read_chunk()
        if (result >> 16) != 0 { return result }
        contract_at = 0
        contract_length = u16(result)
        var index:u16 = 0
        while index < contract_length {
            contract_buffer[index] = input_chunk[index]
            index = index + 1
        }
    }
    var count:u16 = contract_length - contract_at
    if count > CHUNK { count = CHUNK }
    var index:u16 = 0
    while index < count {
        input_chunk[index] = contract_buffer[contract_at+index]
        index = index + 1
    }
    contract_at = contract_at + count
    return u32(count)
}
'''
