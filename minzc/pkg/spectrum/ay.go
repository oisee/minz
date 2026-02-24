package spectrum

// AY-3-8912 / YM2149 sound chip emulator.
// Faithful Go port of AYumi by Peter Sovietov (MIT license).
// https://github.com/true-grue/ayumi

import (
	"math"
)

const (
	ayToneChannels = 3
	ayDecimateFactor = 8
	ayFIRSize        = 192
	ayDCFilterSize   = 1024
)

// AY DAC table (16 levels, each doubled to 32 entries).
var ayDACTable = [32]float64{
	0.0, 0.0,
	0.00999465934234, 0.00999465934234,
	0.0144502937362, 0.0144502937362,
	0.0210574502174, 0.0210574502174,
	0.0307011520562, 0.0307011520562,
	0.0455481803616, 0.0455481803616,
	0.0644998855573, 0.0644998855573,
	0.107362478065, 0.107362478065,
	0.126588845655, 0.126588845655,
	0.20498970016, 0.20498970016,
	0.292210269322, 0.292210269322,
	0.372838941024, 0.372838941024,
	0.492530708782, 0.492530708782,
	0.635324635691, 0.635324635691,
	0.805584802014, 0.805584802014,
	1.0, 1.0,
}

// YM DAC table (32 distinct levels).
var ymDACTable = [32]float64{
	0.0, 0.0,
	0.00465400167849, 0.00772106507973,
	0.0109559777218, 0.0139620050355,
	0.0169985503929, 0.0200198367285,
	0.024368657969, 0.029694056611,
	0.0350652323186, 0.0403906309606,
	0.0485389486534, 0.0583352407111,
	0.0680552376593, 0.0777752346075,
	0.0925154497597, 0.111085679408,
	0.129747463188, 0.148485542077,
	0.17666895552, 0.211551079576,
	0.246387426566, 0.281101701381,
	0.333730067903, 0.400427252613,
	0.467383840696, 0.53443198291,
	0.635172045472, 0.75800717174,
	0.879926756695, 1.0,
}

type ayToneChannel struct {
	tonePeriod  int
	toneCounter int
	tone        int
	tOff        int // tone disable (1=off)
	nOff        int // noise disable (1=off)
	eOn         int // envelope enable
	volume      int // 0-15
	panLeft     float64
	panRight    float64
}

type ayInterpolator struct {
	c [4]float64
	y [4]float64
}

type ayDCFilter struct {
	sum   float64
	delay [ayDCFilterSize]float64
}

// Envelope segment actions (integer dispatch to avoid init cycles).
const (
	envActSlideUp   = 0
	envActSlideDown = 1
	envActHoldTop   = 2
	envActHoldBottom = 3
)

// 16 envelope shapes, each with 2 segments.
// Indexed by envelope shape register (R13, 0-15).
var envelopeShapes = [16][2]int{
	{envActSlideDown, envActHoldBottom}, // 0
	{envActSlideDown, envActHoldBottom}, // 1
	{envActSlideDown, envActHoldBottom}, // 2
	{envActSlideDown, envActHoldBottom}, // 3
	{envActSlideUp, envActHoldBottom},   // 4
	{envActSlideUp, envActHoldBottom},   // 5
	{envActSlideUp, envActHoldBottom},   // 6
	{envActSlideUp, envActHoldBottom},   // 7
	{envActSlideDown, envActSlideDown},  // 8
	{envActSlideDown, envActHoldBottom}, // 9
	{envActSlideDown, envActSlideUp},    // 10
	{envActSlideDown, envActHoldTop},    // 11
	{envActSlideUp, envActSlideUp},      // 12
	{envActSlideUp, envActHoldTop},      // 13
	{envActSlideUp, envActSlideDown},    // 14
	{envActSlideUp, envActHoldBottom},   // 15
}

// AYChip emulates an AY-3-8912 or YM2149 PSG.
type AYChip struct {
	channels [ayToneChannels]ayToneChannel

	noisePeriod  int
	noiseCounter int
	noise        int

	envelopeCounter int
	envelopePeriod  int
	envelopeShape   int
	envelopeSegment int
	envelope        int

	dacTable *[32]float64

	step float64
	x    float64

	interpLeft  ayInterpolator
	interpRight ayInterpolator

	firLeft  [ayFIRSize * 2]float64
	firRight [ayFIRSize * 2]float64
	firIndex int

	dcLeft  ayDCFilter
	dcRight ayDCFilter
	dcIndex int

	Left  float64
	Right float64

	// Current register select (written via port $FFFD)
	selectedReg byte

	// Frame-synchronized audio buffer.
	// EndFrame() renders one frame's worth of samples into this buffer,
	// which is then drained by the audio mixer.
	frameBufLeft  []float64
	frameBufRight []float64
	frameBufPos   int // read position
	frameBufLen   int // valid samples
	sampleRate    int
	enabled       bool
}

// NewAYChip creates and configures an AY/YM chip.
// isYM: true for YM2149, false for AY-3-8910.
// clockRate: chip clock in Hz (1773400 for ZX Spectrum 128K).
// sampleRate: output sample rate (44100 typical).
func NewAYChip(isYM bool, clockRate float64, sampleRate int) *AYChip {
	ay := &AYChip{}
	ay.step = clockRate / float64(sampleRate*8*ayDecimateFactor)
	ay.sampleRate = sampleRate
	if isYM {
		ay.dacTable = &ymDACTable
	} else {
		ay.dacTable = &ayDACTable
	}
	ay.enabled = true
	ay.noise = 1
	ay.SetEnvelopePeriod(1)
	for i := 0; i < ayToneChannels; i++ {
		ay.SetTone(i, 1)
	}
	// Default panning: A=left, B=center, C=right (ACB stereo)
	ay.SetPan(0, 0.1, true)
	ay.SetPan(1, 0.5, true)
	ay.SetPan(2, 0.9, true)

	// Pre-allocate frame buffer (882 samples per frame at 44100Hz/50Hz)
	frameSamples := sampleRate / 50
	ay.frameBufLeft = make([]float64, frameSamples+1)
	ay.frameBufRight = make([]float64, frameSamples+1)

	return ay
}

func (ay *AYChip) resetSegment() {
	act := envelopeShapes[ay.envelopeShape][ay.envelopeSegment]
	if act == envActSlideDown || act == envActHoldTop {
		ay.envelope = 31
	} else {
		ay.envelope = 0
	}
}

func (ay *AYChip) execEnvelopeAction() {
	act := envelopeShapes[ay.envelopeShape][ay.envelopeSegment]
	switch act {
	case envActSlideUp:
		ay.envelope++
		if ay.envelope > 31 {
			ay.envelopeSegment ^= 1
			ay.resetSegment()
		}
	case envActSlideDown:
		ay.envelope--
		if ay.envelope < 0 {
			ay.envelopeSegment ^= 1
			ay.resetSegment()
		}
	case envActHoldTop:
		// no-op
	case envActHoldBottom:
		// no-op
	}
}

func (ay *AYChip) updateTone(index int) int {
	ch := &ay.channels[index]
	ch.toneCounter++
	if ch.toneCounter >= ch.tonePeriod {
		ch.toneCounter = 0
		ch.tone ^= 1
	}
	return ch.tone
}

func (ay *AYChip) updateNoise() int {
	ay.noiseCounter++
	if ay.noiseCounter >= ay.noisePeriod<<1 {
		ay.noiseCounter = 0
		bit0x3 := (ay.noise ^ (ay.noise >> 3)) & 1
		ay.noise = (ay.noise >> 1) | (bit0x3 << 16)
	}
	return ay.noise & 1
}

func (ay *AYChip) updateEnvelope() int {
	ay.envelopeCounter++
	if ay.envelopeCounter >= ay.envelopePeriod {
		ay.envelopeCounter = 0
		ay.execEnvelopeAction()
	}
	return ay.envelope
}

func (ay *AYChip) updateMixer() {
	noise := ay.updateNoise()
	envelope := ay.updateEnvelope()
	ay.Left = 0
	ay.Right = 0
	for i := 0; i < ayToneChannels; i++ {
		ch := &ay.channels[i]
		out := (ay.updateTone(i) | ch.tOff) & (noise | ch.nOff)
		if ch.eOn != 0 {
			out *= envelope
		} else {
			out *= ch.volume*2 + 1
		}
		ay.Left += ay.dacTable[out] * ch.panLeft
		ay.Right += ay.dacTable[out] * ch.panRight
	}
}

// SetPan sets stereo panning for a channel. pan: 0.0=left, 1.0=right.
// eqp: true for equal-power panning (sqrt curve).
func (ay *AYChip) SetPan(index int, pan float64, eqp bool) {
	if index < 0 || index >= ayToneChannels {
		return
	}
	if eqp {
		ay.channels[index].panLeft = math.Sqrt(1 - pan)
		ay.channels[index].panRight = math.Sqrt(pan)
	} else {
		ay.channels[index].panLeft = 1 - pan
		ay.channels[index].panRight = pan
	}
}

// SetTone sets the 12-bit tone period for a channel.
func (ay *AYChip) SetTone(index int, period int) {
	if index < 0 || index >= ayToneChannels {
		return
	}
	period &= 0xFFF
	if period == 0 {
		period = 1
	}
	ay.channels[index].tonePeriod = period
}

// SetNoise sets the 5-bit noise period.
func (ay *AYChip) SetNoise(period int) {
	period &= 0x1F
	if period == 0 {
		period = 1
	}
	ay.noisePeriod = period
}

// SetMixer sets mixer flags for a channel.
// tOff: 1=tone disabled, nOff: 1=noise disabled, eOn: 1=envelope mode.
func (ay *AYChip) SetMixer(index int, tOff, nOff, eOn int) {
	if index < 0 || index >= ayToneChannels {
		return
	}
	ay.channels[index].tOff = tOff & 1
	ay.channels[index].nOff = nOff & 1
	ay.channels[index].eOn = eOn
}

// SetVolume sets the 4-bit volume for a channel (0-15).
func (ay *AYChip) SetVolume(index int, volume int) {
	if index < 0 || index >= ayToneChannels {
		return
	}
	ay.channels[index].volume = volume & 0xF
}

// SetEnvelopePeriod sets the 16-bit envelope period.
func (ay *AYChip) SetEnvelopePeriod(period int) {
	period &= 0xFFFF
	if period == 0 {
		period = 1
	}
	ay.envelopePeriod = period
}

// SetEnvelopeShape sets the envelope shape (0-15) and resets the envelope.
func (ay *AYChip) SetEnvelopeShape(shape int) {
	ay.envelopeShape = shape & 0xF
	ay.envelopeCounter = 0
	ay.envelopeSegment = 0
	ay.resetSegment()
}

// WriteRegister writes a value to an AY register (0-15).
// This is the standard register-level interface used by port I/O.
func (ay *AYChip) WriteRegister(reg byte, val byte) {
	switch reg {
	case 0: // Channel A fine tune
		ay.SetTone(0, (ay.channels[0].tonePeriod & 0xF00) | int(val))
	case 1: // Channel A coarse tune
		ay.SetTone(0, (ay.channels[0].tonePeriod & 0x0FF) | (int(val&0x0F) << 8))
	case 2: // Channel B fine tune
		ay.SetTone(1, (ay.channels[1].tonePeriod & 0xF00) | int(val))
	case 3: // Channel B coarse tune
		ay.SetTone(1, (ay.channels[1].tonePeriod & 0x0FF) | (int(val&0x0F) << 8))
	case 4: // Channel C fine tune
		ay.SetTone(2, (ay.channels[2].tonePeriod & 0xF00) | int(val))
	case 5: // Channel C coarse tune
		ay.SetTone(2, (ay.channels[2].tonePeriod & 0x0FF) | (int(val&0x0F) << 8))
	case 6: // Noise period
		ay.SetNoise(int(val))
	case 7: // Mixer control
		for i := 0; i < 3; i++ {
			tOff := int((val >> uint(i)) & 1)
			nOff := int((val >> uint(i+3)) & 1)
			ay.SetMixer(i, tOff, nOff, ay.channels[i].eOn)
		}
	case 8: // Channel A volume
		if val&0x10 != 0 {
			ay.channels[0].eOn = 1
		} else {
			ay.channels[0].eOn = 0
			ay.SetVolume(0, int(val&0x0F))
		}
	case 9: // Channel B volume
		if val&0x10 != 0 {
			ay.channels[1].eOn = 1
		} else {
			ay.channels[1].eOn = 0
			ay.SetVolume(1, int(val&0x0F))
		}
	case 10: // Channel C volume
		if val&0x10 != 0 {
			ay.channels[2].eOn = 1
		} else {
			ay.channels[2].eOn = 0
			ay.SetVolume(2, int(val&0x0F))
		}
	case 11: // Envelope fine tune
		ay.SetEnvelopePeriod((ay.envelopePeriod & 0xFF00) | int(val))
	case 12: // Envelope coarse tune
		ay.SetEnvelopePeriod((ay.envelopePeriod & 0x00FF) | (int(val) << 8))
	case 13: // Envelope shape
		ay.SetEnvelopeShape(int(val))
	case 14, 15: // I/O ports (ignored)
	}
}

// ReadRegister reads an AY register value (for completeness).
func (ay *AYChip) ReadRegister(reg byte) byte {
	switch reg {
	case 0:
		return byte(ay.channels[0].tonePeriod & 0xFF)
	case 1:
		return byte((ay.channels[0].tonePeriod >> 8) & 0x0F)
	case 2:
		return byte(ay.channels[1].tonePeriod & 0xFF)
	case 3:
		return byte((ay.channels[1].tonePeriod >> 8) & 0x0F)
	case 4:
		return byte(ay.channels[2].tonePeriod & 0xFF)
	case 5:
		return byte((ay.channels[2].tonePeriod >> 8) & 0x0F)
	case 6:
		return byte(ay.noisePeriod)
	case 7:
		var val byte
		for i := 0; i < 3; i++ {
			val |= byte(ay.channels[i].tOff) << uint(i)
			val |= byte(ay.channels[i].nOff) << uint(i+3)
		}
		return val
	case 8:
		if ay.channels[0].eOn != 0 {
			return 0x10
		}
		return byte(ay.channels[0].volume)
	case 9:
		if ay.channels[1].eOn != 0 {
			return 0x10
		}
		return byte(ay.channels[1].volume)
	case 10:
		if ay.channels[2].eOn != 0 {
			return 0x10
		}
		return byte(ay.channels[2].volume)
	case 11:
		return byte(ay.envelopePeriod & 0xFF)
	case 12:
		return byte((ay.envelopePeriod >> 8) & 0xFF)
	case 13:
		return byte(ay.envelopeShape)
	default:
		return 0xFF
	}
}

// Process generates one stereo sample pair with FIR decimation.
// After calling, read ay.Left and ay.Right for the output.
func (ay *AYChip) Process() {
	cL := &ay.interpLeft.c
	yL := &ay.interpLeft.y
	cR := &ay.interpRight.c
	yR := &ay.interpRight.y

	firBase := ayFIRSize - ay.firIndex*ayDecimateFactor
	ay.firIndex = (ay.firIndex + 1) % (ayFIRSize/ayDecimateFactor - 1)

	for i := ayDecimateFactor - 1; i >= 0; i-- {
		ay.x += ay.step
		if ay.x >= 1 {
			ay.x -= 1
			yL[0] = yL[1]
			yL[1] = yL[2]
			yL[2] = yL[3]
			yR[0] = yR[1]
			yR[1] = yR[2]
			yR[2] = yR[3]
			ay.updateMixer()
			yL[3] = ay.Left
			yR[3] = ay.Right
			y1 := yL[2] - yL[0]
			cL[0] = 0.5*yL[1] + 0.25*(yL[0]+yL[2])
			cL[1] = 0.5 * y1
			cL[2] = 0.25 * (yL[3] - yL[1] - y1)
			y1 = yR[2] - yR[0]
			cR[0] = 0.5*yR[1] + 0.25*(yR[0]+yR[2])
			cR[1] = 0.5 * y1
			cR[2] = 0.25 * (yR[3] - yR[1] - y1)
		}
		ay.firLeft[firBase+i] = (cL[2]*ay.x+cL[1])*ay.x + cL[0]
		ay.firRight[firBase+i] = (cR[2]*ay.x+cR[1])*ay.x + cR[0]
	}

	ay.Left = decimate(ay.firLeft[firBase:])
	ay.Right = decimate(ay.firRight[firBase:])
}

// RemoveDC applies a high-pass DC removal filter.
func (ay *AYChip) RemoveDC() {
	ay.Left = dcFilter(&ay.dcLeft, ay.dcIndex, ay.Left)
	ay.Right = dcFilter(&ay.dcRight, ay.dcIndex, ay.Right)
	ay.dcIndex = (ay.dcIndex + 1) & (ayDCFilterSize - 1)
}

func dcFilter(dc *ayDCFilter, index int, x float64) float64 {
	dc.sum += -dc.delay[index] + x
	dc.delay[index] = x
	return x - dc.sum/ayDCFilterSize
}

// FIR decimation filter — 192-tap symmetric polyphase.
// Coefficients from AYumi by Peter Sovietov.
func decimate(x []float64) float64 {
	y := -0.0000046183113992051936*(x[1]+x[191]) +
		-0.00001117761640887225*(x[2]+x[190]) +
		-0.000018610264502005432*(x[3]+x[189]) +
		-0.000025134586135631012*(x[4]+x[188]) +
		-0.000028494281690666197*(x[5]+x[187]) +
		-0.000026396828793275159*(x[6]+x[186]) +
		-0.000017094212558802156*(x[7]+x[185]) +
		0.000023798193576966866*(x[9]+x[183]) +
		0.000051281160242202183*(x[10]+x[182]) +
		0.00007762197826243427*(x[11]+x[181]) +
		0.000096759426664120416*(x[12]+x[180]) +
		0.00010240229300393402*(x[13]+x[179]) +
		0.000089344614218077106*(x[14]+x[178]) +
		0.000054875700118949183*(x[15]+x[177]) +
		-0.000069839082210680165*(x[17]+x[175]) +
		-0.0001447966132360757*(x[18]+x[174]) +
		-0.00021158452917708308*(x[19]+x[173]) +
		-0.00025535069106550544*(x[20]+x[172]) +
		-0.00026228714374322104*(x[21]+x[171]) +
		-0.00022258805927027799*(x[22]+x[170]) +
		-0.00013323230495695704*(x[23]+x[169]) +
		0.00016182578767055206*(x[25]+x[167]) +
		0.00032846175385096581*(x[26]+x[166]) +
		0.00047045611576184863*(x[27]+x[165]) +
		0.00055713851457530944*(x[28]+x[164]) +
		0.00056212565121518726*(x[29]+x[163]) +
		0.00046901918553962478*(x[30]+x[162]) +
		0.00027624866838952986*(x[31]+x[161]) +
		-0.00032564179486838622*(x[33]+x[159]) +
		-0.00065182310286710388*(x[34]+x[158]) +
		-0.00092127787309319298*(x[35]+x[157]) +
		-0.0010772534348943575*(x[36]+x[156]) +
		-0.0010737727700273478*(x[37]+x[155]) +
		-0.00088556645390392634*(x[38]+x[154]) +
		-0.00051581896090765534*(x[39]+x[153]) +
		0.00059548767193795277*(x[41]+x[151]) +
		0.0011803558710661009*(x[42]+x[150]) +
		0.0016527320270369871*(x[43]+x[149]) +
		0.0019152679330965555*(x[44]+x[148]) +
		0.0018927324805381538*(x[45]+x[147]) +
		0.0015481870327877937*(x[46]+x[146]) +
		0.00089470695834941306*(x[47]+x[145]) +
		-0.0010178225878206125*(x[49]+x[143]) +
		-0.0020037400552054292*(x[50]+x[142]) +
		-0.0027874356824117317*(x[51]+x[141]) +
		-0.003210329988021943*(x[52]+x[140]) +
		-0.0031540624117984395*(x[53]+x[139]) +
		-0.0025657163651900345*(x[54]+x[138]) +
		-0.0014750752642111449*(x[55]+x[137]) +
		0.0016624165446378462*(x[57]+x[135]) +
		0.0032591192839069179*(x[58]+x[134]) +
		0.0045165685815867747*(x[59]+x[133]) +
		0.0051838984346123896*(x[60]+x[132]) +
		0.0050774264697459933*(x[61]+x[131]) +
		0.0041192521414141585*(x[62]+x[130]) +
		0.0023628575417966491*(x[63]+x[129]) +
		-0.0026543507866759182*(x[65]+x[127]) +
		-0.0051990251084333425*(x[66]+x[126]) +
		-0.0072020238234656924*(x[67]+x[125]) +
		-0.0082672928192007358*(x[68]+x[124]) +
		-0.0081033739572956287*(x[69]+x[123]) +
		-0.006583111539570221*(x[70]+x[122]) +
		-0.0037839040415292386*(x[71]+x[121]) +
		0.0042781252851152507*(x[73]+x[119]) +
		0.0084176358598320178*(x[74]+x[118]) +
		0.01172566057463055*(x[75]+x[117]) +
		0.013550476647788672*(x[76]+x[116]) +
		0.013388189369997496*(x[77]+x[115]) +
		0.010979501242341259*(x[78]+x[114]) +
		0.006381274941685413*(x[79]+x[113]) +
		-0.007421229604153888*(x[81]+x[111]) +
		-0.01486456304340213*(x[82]+x[110]) +
		-0.021143584622178104*(x[83]+x[109]) +
		-0.02504275058758609*(x[84]+x[108]) +
		-0.025473530942547201*(x[85]+x[107]) +
		-0.021627310017882196*(x[86]+x[106]) +
		-0.013104323383225543*(x[87]+x[105]) +
		0.017065133989980476*(x[89]+x[103]) +
		0.036978919264451952*(x[90]+x[102]) +
		0.05823318062093958*(x[91]+x[101]) +
		0.079072012081405949*(x[92]+x[100]) +
		0.097675998716952317*(x[93]+x[99]) +
		0.11236045936950932*(x[94]+x[98]) +
		0.12176343577287731*(x[95]+x[97]) +
		0.125*x[96]

	// Shift FIR history
	copy(x[ayFIRSize-ayDecimateFactor:], x[:ayDecimateFactor])
	return y
}

// RenderSamples generates n stereo sample pairs into left/right buffers.
func (ay *AYChip) RenderSamples(left, right []float64, n int) {
	for i := 0; i < n; i++ {
		ay.Process()
		ay.RemoveDC()
		if i < len(left) {
			left[i] = ay.Left
		}
		if i < len(right) {
			right[i] = ay.Right
		}
	}
}

// EndFrame renders one frame's worth of AY samples into the frame buffer.
// Call this once per frame from Machine.RunFrame(), synchronized with the
// beeper's EndFrame(). This ensures AY audio is generated at frame boundaries
// rather than on-demand from the audio callback.
// SetEnabled enables or disables AY audio output.
func (ay *AYChip) SetEnabled(enabled bool) {
	ay.enabled = enabled
}

func (ay *AYChip) EndFrame() {
	if !ay.enabled {
		ay.frameBufPos = 0
		ay.frameBufLen = 0
		return
	}
	n := ay.sampleRate / 50 // 882 samples at 44100Hz
	if n > len(ay.frameBufLeft) {
		ay.frameBufLeft = make([]float64, n)
		ay.frameBufRight = make([]float64, n)
	}
	ay.RenderSamples(ay.frameBufLeft[:n], ay.frameBufRight[:n], n)
	ay.frameBufPos = 0
	ay.frameBufLen = n
}

// Available returns the number of samples available for reading.
func (ay *AYChip) Available() int {
	return ay.frameBufLen - ay.frameBufPos
}

// ReadFrameSamples drains up to len(left) stereo sample pairs from the
// frame buffer. Returns the number of samples actually read.
func (ay *AYChip) ReadFrameSamples(left, right []float64) int {
	avail := ay.frameBufLen - ay.frameBufPos
	n := len(left)
	if n > len(right) {
		n = len(right)
	}
	if n > avail {
		n = avail
	}
	copy(left[:n], ay.frameBufLeft[ay.frameBufPos:ay.frameBufPos+n])
	copy(right[:n], ay.frameBufRight[ay.frameBufPos:ay.frameBufPos+n])
	ay.frameBufPos += n
	return n
}
