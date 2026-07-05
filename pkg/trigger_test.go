package decoder

import (
	"reflect"
	"testing"
)

func TestCheckBit(t *testing.T) {
	cases := []struct {
		mask uint16
		pos  uint16
		want bool
	}{
		{0x0001, 0, true},
		{0x0001, 1, false},
		{0x8000, 15, true},
		{0x8000, 14, false},
		{0xFFFF, 7, true},
		{0x0000, 0, false},
	}
	for _, c := range cases {
		if got := CheckBit(c.mask, c.pos); got != c.want {
			t.Errorf("CheckBit(%#04x, %d) = %t, want %t", c.mask, c.pos, got, c.want)
		}
	}
}

// TestReadTriggerFEC decodes a hand-built 17-word trigger FEC payload and
// checks every field lands in TriggerData with the documented bit layout.
func TestReadTriggerFEC(t *testing.T) {
	data := []uint16{
		0x0002, // conf8: triggerMask bits 16-25
		0xABCD, // conf7: triggerMask bits 0-15
		0x1234, // conf6: triggerDiff1
		0x5678, // conf5: triggerDiff2
		// conf4: windowA1=42, chanA1=85, auto=1, dual=0, external=1
		42 | 85<<6 | 1<<13 | 0<<14 | 1<<15,
		// conf3: windowB1=5, chanB1=100, mask=1, trgB2=1, trgB1=0
		5 | 100<<6 | 1<<13 | 1<<14 | 0<<15,
		// conf2: windowA2=63, chanA2=1
		63 | 1<<6,
		// conf1: windowB2=0, chanB2=127
		0 | 127<<6,
		// conf0: extN=15, intN=2748
		15 | 2748<<4,
		0x8000, // trigger type = bit 15
		// Trigger channels 47..32, 31..16, 15..0 (MSB = highest channel)
		0x8010, // ch47, ch36
		0x0200, // ch25
		0x1801, // ch12, ch11, ch0
		0xDEAD, 0xBEEF, // triggerLost2
		0x0102, 0x0304, // triggerLost1
	}

	event := EventType{}
	ReadTriggerFEC(data, &event)
	trg := event.TriggerConfig

	if trg.TriggerMask != 0x2ABCD {
		t.Errorf("TriggerMask = %#x, want 0x2ABCD", trg.TriggerMask)
	}
	if trg.TriggerDiff1 != 0x1234 || trg.TriggerDiff2 != 0x5678 {
		t.Errorf("TriggerDiff = %#x, %#x, want 0x1234, 0x5678", trg.TriggerDiff1, trg.TriggerDiff2)
	}
	if trg.WindowA1 != 42 || trg.ChanA1 != 85 {
		t.Errorf("WindowA1/ChanA1 = %d/%d, want 42/85", trg.WindowA1, trg.ChanA1)
	}
	if trg.AutoTrigger != 1 || trg.DualTrigger != 0 || trg.ExternalTrigger != 1 {
		t.Errorf("Auto/Dual/External = %d/%d/%d, want 1/0/1",
			trg.AutoTrigger, trg.DualTrigger, trg.ExternalTrigger)
	}
	if trg.WindowB1 != 5 || trg.ChanB1 != 100 || trg.Mask != 1 || trg.TriggerB2 != 1 || trg.TriggerB1 != 0 {
		t.Errorf("conf3 fields = %d/%d/%d/%d/%d, want 5/100/1/1/0",
			trg.WindowB1, trg.ChanB1, trg.Mask, trg.TriggerB2, trg.TriggerB1)
	}
	if trg.WindowA2 != 63 || trg.ChanA2 != 1 {
		t.Errorf("WindowA2/ChanA2 = %d/%d, want 63/1", trg.WindowA2, trg.ChanA2)
	}
	if trg.WindowB2 != 0 || trg.ChanB2 != 127 {
		t.Errorf("WindowB2/ChanB2 = %d/%d, want 0/127", trg.WindowB2, trg.ChanB2)
	}
	if trg.TriggerExtN != 15 || trg.TriggerIntN != 2748 {
		t.Errorf("ExtN/IntN = %d/%d, want 15/2748", trg.TriggerExtN, trg.TriggerIntN)
	}
	if trg.TriggerType != 1 {
		t.Errorf("TriggerType = %d, want 1", trg.TriggerType)
	}
	// Channels are appended from ch47 downwards; elecID = 100*(ch/12+1) + ch%12
	wantChannels := []uint16{411, 400, 301, 200, 111, 100}
	if !reflect.DeepEqual(trg.TrgChannels, wantChannels) {
		t.Errorf("TrgChannels = %v, want %v", trg.TrgChannels, wantChannels)
	}
	if trg.TriggerLost2 != 0xDEADBEEF {
		t.Errorf("TriggerLost2 = %#x, want 0xDEADBEEF", trg.TriggerLost2)
	}
	if trg.TriggerLost1 != 0x01020304 {
		t.Errorf("TriggerLost1 = %#x, want 0x01020304", trg.TriggerLost1)
	}
}

// TestReadTriggerFECNoChannels: all channel-mask words zero -> empty slice.
func TestReadTriggerFECNoChannels(t *testing.T) {
	data := make([]uint16, 17)
	event := EventType{}
	ReadTriggerFEC(data, &event)
	if len(event.TriggerConfig.TrgChannels) != 0 {
		t.Errorf("TrgChannels = %v, want empty", event.TriggerConfig.TrgChannels)
	}
}
