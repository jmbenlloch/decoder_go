package decoder

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSipmDataEqualLength(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222, 0x3333}
	dataB := []uint16{0xaaaa, 0xbbbb, 0xcccc}
	want := []uint16{0x1111, 0xaaaa, 0x2222, 0xbbbb, 0x3333, 0xcccc}

	got := buildSipmData(dataA, dataB)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSipmData(equal) = %#04x, want %#04x", got, want)
	}
}

// TestBuildSipmDataAOneLonger covers the normal hardware condition where the
// interleaved stream has an odd total word count, so link A (odd positions)
// ends up with exactly one more word than link B.
func TestBuildSipmDataAOneLonger(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222, 0x3333}
	dataB := []uint16{0xaaaa, 0xbbbb}
	want := []uint16{0x1111, 0xaaaa, 0x2222, 0xbbbb, 0x3333}

	got := buildSipmData(dataA, dataB)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSipmData(A one longer) = %#04x, want %#04x", got, want)
	}
}

// TestBuildSipmDataBOneLonger is the mirror case: link B ends up with the
// extra trailing word instead of link A.
func TestBuildSipmDataBOneLonger(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222}
	dataB := []uint16{0xaaaa, 0xbbbb, 0xcccc}
	want := []uint16{0x1111, 0xaaaa, 0x2222, 0xbbbb, 0xcccc}

	got := buildSipmData(dataA, dataB)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSipmData(B one longer) = %#04x, want %#04x", got, want)
	}
}

func TestBuildSipmDataEmpty(t *testing.T) {
	got := buildSipmData(nil, nil)

	if len(got) != 0 {
		t.Fatalf("buildSipmData(empty) = %#04x, want empty", got)
	}
}

func TestComputeSipmPositionRoundTrip(t *testing.T) {
	cases := []struct{ elecID, position uint16 }{
		{1000, 0},
		{1063, 63},
		{2000, 64},
		{2063, 127},
		{14000, 832},
	}
	for _, c := range cases {
		if got := computeSipmPosition(c.elecID); got != c.position {
			t.Errorf("computeSipmPosition(%d) = %d, want %d", c.elecID, got, c.position)
		}
		if got := computeSipmIDFromPosition(c.position); got != c.elecID {
			t.Errorf("computeSipmIDFromPosition(%d) = %d, want %d", c.position, got, c.elecID)
		}
	}
}

// The four channel-mask words cover channels 63..0 MSB-first: word 0 bit 15 is
// channel 63, word 3 bit 0 is channel 0. Returned elecIDs are sorted.
func TestSipmChannelMask(t *testing.T) {
	data := []uint16{0x8000, 0x0000, 0x0000, 0x0001}

	chMask, positions, pos := sipmChannelMask(data, 0, 0)

	if !reflect.DeepEqual(chMask, []uint16{1000, 1063}) {
		t.Errorf("channel mask = %v, want [1000 1063]", chMask)
	}
	if !reflect.DeepEqual(positions, []uint16{0, 63}) {
		t.Errorf("positions = %v, want [0 63]", positions)
	}
	if pos != 4 {
		t.Errorf("position = %d, want 4", pos)
	}
}

func TestSipmChannelMaskFebOffset(t *testing.T) {
	// febID 3 -> elecIDs in the 4000 range
	data := []uint16{0x0000, 0x0000, 0x0000, 0x0001}
	chMask, positions, _ := sipmChannelMask(data, 0, 3)
	if !reflect.DeepEqual(chMask, []uint16{4000}) {
		t.Errorf("channel mask = %v, want [4000]", chMask)
	}
	if !reflect.DeepEqual(positions, []uint16{192}) {
		t.Errorf("positions = %v, want [192]", positions)
	}
}

func TestComputeSipmTimeRaw(t *testing.T) {
	// Without zero suppression the FT word is returned untouched.
	evtFormat := EventFormat{FWVersion: 10, ZeroSuppression: false}
	ft, pos := computeSipmTime([]uint16{0x0123}, 0, &evtFormat)
	if ft != 0x123 || pos != 1 {
		t.Errorf("computeSipmTime(raw) = (%#x, %d), want (0x123, 1)", ft, pos)
	}
}

func TestComputeSipmTimeZS(t *testing.T) {
	// With ZS the FT is rebased onto the ring buffer start position:
	// ring = BufferSamples2/40 = 1000
	// start = (TriggerFT - PreTrigger + BufferSamples2)/40 % ring
	//       = (4000 - 2000 + 40000)/40 % 1000 = 50
	evtFormat := EventFormat{
		FWVersion:       10,
		TriggerType:     1,
		ZeroSuppression: true,
		TriggerFT:       4000,
		PreTrigger:      2000,
		BufferSamples2:  40000,
		FTBit:           0,
	}

	// FT ahead of start position
	ft, pos := computeSipmTime([]uint16{60}, 0, &evtFormat)
	if ft != 10 || pos != 1 {
		t.Errorf("computeSipmTime(ZS, no wrap) = (%d, %d), want (10, 1)", ft, pos)
	}

	// FT behind start position wraps around the ring buffer
	ft, _ = computeSipmTime([]uint16{30}, 0, &evtFormat)
	if ft != 980 {
		t.Errorf("computeSipmTime(ZS, wrap) = %d, want 980", ft)
	}
}

// TestBuildSipmDataLargeMismatchPanics ensures a difference of more than one
// word is still treated as a real error, not silently tolerated.
func TestBuildSipmDataLargeMismatchPanics(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222, 0x3333, 0x4444}
	dataB := []uint16{0xaaaa}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("buildSipmData did not panic on a >1 word length mismatch")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "must have the same length") {
			t.Fatalf("panic value = %v, want message about mismatched link length", r)
		}
	}()

	buildSipmData(dataA, dataB)
}
