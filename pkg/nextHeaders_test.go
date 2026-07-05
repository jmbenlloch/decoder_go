package decoder

import (
	"reflect"
	"testing"
)

// buildFW10Header assembles a complete firmware-10 common header (the format
// written by the current DAQ) with known values in every field.
func buildFW10Header() []uint16 {
	return []uint16{
		0x0000, 0x0000, // sequence counter = 0 (word 1 is the one checked)
		// FormatID H: FecType=1 (SiPM), ZS=1, Compressed=1, Baseline=1,
		// DualMode=0, ErrorBit=0
		0x0001 | 1<<4 | 1<<5 | 1<<6,
		// FormatID L: FWVersion=10, ChannelsHG=1
		10 | 1<<15,
		1234, // word count
		// EventID: TriggerType=9, TriggerCounter = ((w&0xFFF0)<<12) + next word
		9 | 0x0020, // counter high nibble contribution: 0x20<<12 = 0x20000
		0x0005,     // counter low word -> TriggerCounter = 0x20005
		// Juliett event conf (halved in the payload)
		32000, // BufferSamples/2   -> 64000
		3200,  // PreTrigger/2      -> 6400
		400,   // BufferSamples2/2  -> 800
		100,   // PreTrigger2/2     -> 200
		0x00FF, // channel mask
		// Baselines: ch0..ch5 = 0x123,0x456,0x789,0xABC,0xDEF,0x135
		0x1234, 0x5678, 0x9ABC, 0xDEF1, 0x3500,
		// FecID word: NumberOfChannels=27 (bits 0-4), FecID=17 (bits 5-15)
		27 | 17<<5,
		// Timestamp high, low, FTh|CT word (FTBit=1, FTh=0x2AB)
		0x0012, 0x3456, 0x2AB | 1<<15,
		// TriggerFT
		0xCAFE,
	}
}

func TestReadCommonHeaderFW10(t *testing.T) {
	evt := ReadCommonHeader(buildFW10Header())

	if evt.FecType != 1 {
		t.Errorf("FecType = %d, want 1", evt.FecType)
	}
	if !evt.ZeroSuppression || !evt.CompressedData || !evt.Baseline {
		t.Errorf("ZS/Compressed/Baseline = %t/%t/%t, want all true",
			evt.ZeroSuppression, evt.CompressedData, evt.Baseline)
	}
	if evt.DualModeBit || evt.ErrorBit {
		t.Errorf("DualMode/ErrorBit = %t/%t, want both false", evt.DualModeBit, evt.ErrorBit)
	}
	if evt.FWVersion != 10 {
		t.Errorf("FWVersion = %d, want 10", evt.FWVersion)
	}
	if !evt.ChannelsHG {
		t.Error("ChannelsHG = false, want true")
	}
	if evt.WordCount != 1234 {
		t.Errorf("WordCount = %d, want 1234", evt.WordCount)
	}
	if evt.TriggerType != 9 {
		t.Errorf("TriggerType = %d, want 9", evt.TriggerType)
	}
	if evt.TriggerCounter != 0x20005 {
		t.Errorf("TriggerCounter = %#x, want 0x20005", evt.TriggerCounter)
	}
	if evt.BufferSamples != 64000 || evt.PreTrigger != 6400 {
		t.Errorf("BufferSamples/PreTrigger = %d/%d, want 64000/6400",
			evt.BufferSamples, evt.PreTrigger)
	}
	if evt.BufferSamples2 != 800 || evt.PreTrigger2 != 200 {
		t.Errorf("BufferSamples2/PreTrigger2 = %d/%d, want 800/200",
			evt.BufferSamples2, evt.PreTrigger2)
	}
	if evt.ChannelMask != 0x00FF {
		t.Errorf("ChannelMask = %#x, want 0x00FF", evt.ChannelMask)
	}
	wantBaselines := []uint16{0x123, 0x456, 0x789, 0xABC, 0xDEF, 0x135}
	if !reflect.DeepEqual(evt.Baselines, wantBaselines) {
		t.Errorf("Baselines = %#x, want %#x", evt.Baselines, wantBaselines)
	}
	if evt.NumberOfChannels != 27 {
		t.Errorf("NumberOfChannels = %d, want 27", evt.NumberOfChannels)
	}
	if evt.FecID != 17 {
		t.Errorf("FecID = %d, want 17", evt.FecID)
	}
	// Timestamp = ((0x0012<<16 | 0x3456) << 10 | 0x2AB) & 0x3FFFFFFFFF
	wantTS := ((uint64(0x0012)<<16 | 0x3456) << 10) | 0x2AB
	wantTS &= 0x03FFFFFFFFFF
	if evt.Timestamp != wantTS {
		t.Errorf("Timestamp = %#x, want %#x", evt.Timestamp, wantTS)
	}
	if evt.FTBit != 1 {
		t.Errorf("FTBit = %d, want 1", evt.FTBit)
	}
	if evt.TriggerFT != 0xCAFE {
		t.Errorf("TriggerFT = %#x, want 0xCAFE", evt.TriggerFT)
	}
	if evt.HeaderSize != 22 {
		t.Errorf("HeaderSize = %d, want 22", evt.HeaderSize)
	}
}

// Without the Baseline flag the five baseline words are absent and the header
// is 5 words shorter.
func TestReadCommonHeaderFW10NoBaseline(t *testing.T) {
	data := buildFW10Header()
	data[2] &^= 1 << 6 // clear Baseline flag
	// remove the 5 baseline words (indices 12-16)
	data = append(data[:12], data[17:]...)

	evt := ReadCommonHeader(data)

	if evt.Baseline {
		t.Error("Baseline = true, want false")
	}
	if evt.Baselines != nil {
		t.Errorf("Baselines = %v, want nil", evt.Baselines)
	}
	if evt.FecID != 17 || evt.NumberOfChannels != 27 {
		t.Errorf("FecID/NumberOfChannels = %d/%d, want 17/27", evt.FecID, evt.NumberOfChannels)
	}
	if evt.TriggerFT != 0xCAFE {
		t.Errorf("TriggerFT = %#x, want 0xCAFE", evt.TriggerFT)
	}
	if evt.HeaderSize != 17 {
		t.Errorf("HeaderSize = %d, want 17", evt.HeaderSize)
	}
}

func TestReadCommonHeaderErrorBit(t *testing.T) {
	data := buildFW10Header()
	data[2] |= 1 << 14 // set ErrorBit
	evt := ReadCommonHeader(data)
	if !evt.ErrorBit {
		t.Error("ErrorBit = false, want true")
	}
}

// A non-zero sequence counter marks a continuation fragment: only the two
// counter words are consumed and no fields are decoded.
func TestReadCommonHeaderNonZeroSeqCounter(t *testing.T) {
	data := buildFW10Header()
	data[1] = 0x0001

	evt := ReadCommonHeader(data)

	if evt.HeaderSize != 2 {
		t.Errorf("HeaderSize = %d, want 2", evt.HeaderSize)
	}
	if evt.FWVersion != 0 || evt.FecID != 0 {
		t.Errorf("continuation fragment decoded fields: FWVersion=%d FecID=%d, want 0/0",
			evt.FWVersion, evt.FecID)
	}
}

func TestEventIdGetNbInRun(t *testing.T) {
	if got := EventIdGetNbInRun(EventIdType{42, 7}); got != 42 {
		t.Errorf("EventIdGetNbInRun = %d, want 42", got)
	}
}

func TestValidEvent(t *testing.T) {
	cases := []struct {
		evtType EventTypeType
		want    bool
	}{
		{PHYSICS_EVENT, true},
		{CALIBRATION_EVENT, true},
		{START_OF_RUN, false},
		{END_OF_RUN, false},
		{SYNC_EVENT, false},
	}
	for _, c := range cases {
		h := EventHeaderStruct{EventType: c.evtType}
		if got := ValidEvent(h); got != c.want {
			t.Errorf("ValidEvent(type %d) = %t, want %t", c.evtType, got, c.want)
		}
	}
}
