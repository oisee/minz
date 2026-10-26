"""Research OS adapter: expose every documented CP/M directory success code."""


def instrument(driver):
    old = 'func (d *diskOS) invoke(fn byte, de uint16) byte {'
    assert driver.count(old) == 1, 'changed CP/M OS adapter anchor'
    driver = driver.replace(old, 'func (d *diskOS) invokeRaw(fn byte, de uint16) byte {', 1)
    return driver + '''
// CP/M2.2 may return any directory slot0..3 for successful15/16/22.
// File operations still happen in the original OS adapter; no music is hosted.
var directoryCode = flag.Uint("directory-code",0,"CP/M directory success code0..3")
func (d *diskOS) invoke(fn byte,de uint16) byte {
 if *directoryCode>3 { panic("invalid directory success fixture") }
 status:=d.invokeRaw(fn,de)
 if status==0 && (fn==15 || fn==16 || fn==22) { return byte(*directoryCode) }
 return status
}
'''
