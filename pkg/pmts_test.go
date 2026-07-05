package decoder

import (
	"reflect"
	"testing"
)

// setFecElecIDBase installs a synthetic FecElecIDBase table (normally loaded
// from the DB) and restores the previous one when the test finishes.
func setFecElecIDBase(t *testing.T, m map[uint16]uint16) {
	t.Helper()
	old := fecElecIDBase
	fecElecIDBase = m
	t.Cleanup(func() { fecElecIDBase = old })
}

func TestComputePmtElecID(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{2: 100, 3: 100, 10: 300})

	cases := []struct {
		fecID   uint16
		channel uint16
		want    uint16
	}{
		{2, 0, 100},  // even FEC -> even elecIDs
		{2, 5, 110},
		{3, 0, 101},  // odd FEC -> odd elecIDs
		{3, 11, 123},
		{10, 3, 306},
	}
	for _, c := range cases {
		got, err := computePmtElecID(c.fecID, c.channel, 10)
		if err != nil {
			t.Fatalf("computePmtElecID(%d, %d): %v", c.fecID, c.channel, err)
		}
		if got != c.want {
			t.Errorf("computePmtElecID(%d, %d) = %d, want %d", c.fecID, c.channel, got, c.want)
		}
	}
}

func TestComputePmtElecIDUnknownFec(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{2: 100})
	if _, err := computePmtElecID(99, 0, 10); err == nil {
		t.Fatal("computePmtElecID with unknown FEC: expected error, got nil")
	}
}

func TestComputePmtPosition(t *testing.T) {
	cases := []struct{ elecID, want uint16 }{
		{100, 0}, {101, 0}, {102, 1}, {111, 5}, {122, 11}, {310, 5},
	}
	for _, c := range cases {
		if got := computePmtPosition(c.elecID); got != c.want {
			t.Errorf("computePmtPosition(%d) = %d, want %d", c.elecID, got, c.want)
		}
	}
}

func TestPmtsChannelMask(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{2: 100})

	evtFormat := EventFormat{
		FecID:       2,
		FWVersion:   10,
		ChannelMask: 0b0000000000000101, // channels 0 and 2
	}
	chMask, positions, err := pmtsChannelMask(&evtFormat)
	if err != nil {
		t.Fatalf("pmtsChannelMask: %v", err)
	}
	if !reflect.DeepEqual(chMask, []uint16{100, 104}) {
		t.Errorf("channel mask = %v, want [100 104]", chMask)
	}
	if !reflect.DeepEqual(positions, []uint16{0, 2}) {
		t.Errorf("positions = %v, want [0 2]", positions)
	}
}

func TestPmtsChannelMaskUnknownFec(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{})
	evtFormat := EventFormat{FecID: 7, FWVersion: 10, ChannelMask: 0x1}
	if _, _, err := pmtsChannelMask(&evtFormat); err == nil {
		t.Fatal("pmtsChannelMask with unknown FEC: expected error, got nil")
	}
}

func TestComputeNextFThmFW10(t *testing.T) {
	evtFormat := EventFormat{
		FWVersion:      10,
		TriggerType:    1, // < 8 -> PreTrigger (not PreTrigger2)
		PreTrigger:     100,
		BufferSamples2: 64000,
		FTBit:          0,
		TriggerFT:      50,
	}

	var nextFT, nextFThm int32 = -1, -1

	// First call: FT initialized from TriggerFT; PreTrigger > FT wraps FThm.
	computeNextFThm(&nextFT, &nextFThm, &evtFormat)
	if nextFT != 50 || nextFThm != 63950 {
		t.Errorf("first call: nextFT/nextFThm = %d/%d, want 50/63950", nextFT, nextFThm)
	}

	// Subsequent calls increment FT.
	computeNextFThm(&nextFT, &nextFThm, &evtFormat)
	if nextFT != 51 || nextFThm != 63951 {
		t.Errorf("second call: nextFT/nextFThm = %d/%d, want 51/63951", nextFT, nextFThm)
	}

	// FT wraps around the ring buffer.
	nextFT = 63999
	computeNextFThm(&nextFT, &nextFThm, &evtFormat)
	if nextFT != 0 || nextFThm != 63900 {
		t.Errorf("wrap: nextFT/nextFThm = %d/%d, want 0/63900", nextFT, nextFThm)
	}
}

func TestComputeNextFThmFW10TriggerType8(t *testing.T) {
	evtFormat := EventFormat{
		FWVersion:      10,
		TriggerType:    8, // >= 8 -> PreTrigger2
		PreTrigger:     100,
		PreTrigger2:    200,
		BufferSamples2: 64000,
		FTBit:          0,
		TriggerFT:      300,
	}
	var nextFT, nextFThm int32 = -1, -1
	computeNextFThm(&nextFT, &nextFThm, &evtFormat)
	if nextFT != 300 || nextFThm != 100 {
		t.Errorf("nextFT/nextFThm = %d/%d, want 300/100", nextFT, nextFThm)
	}
}

func TestComputeNextFThmFTBit(t *testing.T) {
	// FTBit contributes bit 16 to the initial FT value.
	evtFormat := EventFormat{
		FWVersion:      10,
		TriggerType:    1,
		PreTrigger:     100,
		BufferSamples2: 128000,
		FTBit:          1,
		TriggerFT:      5,
	}
	var nextFT, nextFThm int32 = -1, -1
	computeNextFThm(&nextFT, &nextFThm, &evtFormat)
	wantFT := int32(1<<16 + 5)
	if nextFT != wantFT || nextFThm != wantFT-100 {
		t.Errorf("nextFT/nextFThm = %d/%d, want %d/%d", nextFT, nextFThm, wantFT, wantFT-100)
	}
}
