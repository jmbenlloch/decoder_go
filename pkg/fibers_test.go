package decoder

import (
	"reflect"
	"testing"
)

func TestComputeFiberElecID(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{6: 500, 7: 500})

	cases := []struct {
		fecID   uint16
		channel uint16
		want    uint16
	}{
		{6, 0, 500},
		{6, 4, 508},
		{7, 0, 501},
		{7, 9, 519},
	}
	for _, c := range cases {
		got, err := computeFiberElecID(c.fecID, c.channel, 10)
		if err != nil {
			t.Fatalf("computeFiberElecID(%d, %d): %v", c.fecID, c.channel, err)
		}
		if got != c.want {
			t.Errorf("computeFiberElecID(%d, %d) = %d, want %d", c.fecID, c.channel, got, c.want)
		}
	}

	if _, err := computeFiberElecID(42, 0, 10); err == nil {
		t.Fatal("computeFiberElecID with unknown FEC: expected error, got nil")
	}
}

func TestComputeFiberPosition(t *testing.T) {
	cases := []struct{ elecID, want uint16 }{
		{500, 0}, {501, 0}, {510, 5}, {519, 9}, {611, 5},
	}
	for _, c := range cases {
		if got := computeFiberPosition(c.elecID); got != c.want {
			t.Errorf("computeFiberPosition(%d) = %d, want %d", c.elecID, got, c.want)
		}
	}
}

func TestWriteFiberPedestals(t *testing.T) {
	evtFormat := EventFormat{
		Baselines: []uint16{0x200, 0x201, 0x202, 0x203, 0x204, 0x205},
	}
	channelMask := []uint16{500, 510, 512, 523}
	baselines := make(map[uint16]uint16)

	writeFiberPedestals(&evtFormat, channelMask, baselines)

	want := map[uint16]uint16{
		500: 0x200, // (0%12)/2 = 0
		510: 0x205, // (10%12)/2 = 5
		512: 0x200, // (12%12)/2 = 0
		523: 0x205, // (23%12)/2 = 5
	}
	if !reflect.DeepEqual(baselines, want) {
		t.Errorf("baselines = %v, want %v", baselines, want)
	}
}

func TestProcessFiberIdsChannelsHG(t *testing.T) {
	event := newTestEvent()
	event.FiberConfig.ChannelsHG = true
	// Regular pair: 501 (HG) pairs with 500 (LG)
	event.FibersLG[500] = []int16{1}
	event.FibersLG[501] = []int16{2}
	event.FiberBaselinesLG[500] = 10
	event.FiberBaselinesLG[501] = 20

	processFiberIds(event, Configuration{})

	if !reflect.DeepEqual(event.FibersHG[500], []int16{2}) {
		t.Errorf("FibersHG[500] = %v, want [2]", event.FibersHG[500])
	}
	if event.FiberBaselinesHG[500] != 20 {
		t.Errorf("FiberBaselinesHG[500] = %d, want 20", event.FiberBaselinesHG[500])
	}
	if _, ok := event.FibersLG[501]; ok {
		t.Error("HG channel 501 still present in FibersLG")
	}
	if !reflect.DeepEqual(event.FibersLG[500], []int16{1}) {
		t.Errorf("FibersLG[500] = %v, want [1]", event.FibersLG[500])
	}
}

// Channels X17 and X19 have swapped HG/LG pairing in the hardware:
// X17 pairs with X18, X19 pairs with X16.
func TestProcessFiberIdsHardwareSwap(t *testing.T) {
	event := newTestEvent()
	event.FiberConfig.ChannelsHG = true
	event.FibersLG[517] = []int16{17}
	event.FibersLG[519] = []int16{19}
	event.FiberBaselinesLG[517] = 17
	event.FiberBaselinesLG[519] = 19

	processFiberIds(event, Configuration{})

	if !reflect.DeepEqual(event.FibersHG[518], []int16{17}) {
		t.Errorf("FibersHG[518] = %v, want [17] (from 517)", event.FibersHG[518])
	}
	if !reflect.DeepEqual(event.FibersHG[516], []int16{19}) {
		t.Errorf("FibersHG[516] = %v, want [19] (from 519)", event.FibersHG[516])
	}
	if event.FiberBaselinesHG[518] != 17 || event.FiberBaselinesHG[516] != 19 {
		t.Errorf("swapped baselines = %d/%d, want 17/19",
			event.FiberBaselinesHG[518], event.FiberBaselinesHG[516])
	}
	if len(event.FibersLG) != 0 {
		t.Errorf("FibersLG still has %d channels, want 0", len(event.FibersLG))
	}
}

// Without the HG flag nothing moves.
func TestProcessFiberIdsNoHG(t *testing.T) {
	event := newTestEvent()
	event.FibersLG[500] = []int16{1}
	event.FibersLG[501] = []int16{2}

	processFiberIds(event, Configuration{})

	if len(event.FibersLG) != 2 || len(event.FibersHG) != 0 {
		t.Errorf("FibersLG/HG = %d/%d channels, want 2/0",
			len(event.FibersLG), len(event.FibersHG))
	}
}

func TestFibersChannelMask(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{6: 500})

	evtFormat := EventFormat{
		FecID:       6,
		FWVersion:   10,
		ChannelMask: 0b0000100000000011, // channels 0, 1, 11
	}
	chMask, positions, err := fibersChannelMask(&evtFormat)
	if err != nil {
		t.Fatalf("fibersChannelMask: %v", err)
	}
	if !reflect.DeepEqual(chMask, []uint16{500, 502, 522}) {
		t.Errorf("channel mask = %v, want [500 502 522]", chMask)
	}
	if !reflect.DeepEqual(positions, []uint16{0, 1, 11}) {
		t.Errorf("positions = %v, want [0 1 11]", positions)
	}
}
