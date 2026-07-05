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

func TestWritePmtPedestals(t *testing.T) {
	evtFormat := EventFormat{
		Baselines: []uint16{0x100, 0x101, 0x102, 0x103, 0x104, 0x105},
	}
	// Only 6 baselines per FEC: elecIDs 100..110 (even) map to indices 0..5,
	// and the second dozen (112..122) reuses the same indices.
	channelMask := []uint16{100, 102, 110, 112, 122, 301}
	baselines := make(map[uint16]uint16)

	writePmtPedestals(&evtFormat, channelMask, baselines)

	want := map[uint16]uint16{
		100: 0x100, // (0%12)/2 = 0
		102: 0x101, // (2%12)/2 = 1
		110: 0x105, // (10%12)/2 = 5
		112: 0x100, // (12%12)/2 = 0
		122: 0x105, // (22%12)/2 = 5
		301: 0x100, // (1%12)/2 = 0
	}
	if !reflect.DeepEqual(baselines, want) {
		t.Errorf("baselines = %v, want %v", baselines, want)
	}
}

func newTestEvent() *EventType {
	return &EventType{
		PmtWaveforms:     make(map[uint16][]int16),
		BlrWaveforms:     make(map[uint16][]int16),
		SipmWaveforms:    make(map[uint16][]int16),
		FibersLG:         make(map[uint16][]int16),
		FibersHG:         make(map[uint16][]int16),
		Baselines:        make(map[uint16]uint16),
		BlrBaselines:     make(map[uint16]uint16),
		FiberBaselinesLG: make(map[uint16]uint16),
		FiberBaselinesHG: make(map[uint16]uint16),
	}
}

func TestProcessPmtIdsExtTriggerAndSum(t *testing.T) {
	event := newTestEvent()
	event.PmtWaveforms[100] = []int16{1, 2}
	event.PmtWaveforms[115] = []int16{3, 4} // ext trigger channel
	event.PmtWaveforms[116] = []int16{5, 6} // pmt sum channel
	event.Baselines[100] = 10
	event.Baselines[115] = 20
	event.Baselines[116] = 30

	config := Configuration{ExtTrigger: 115, PmtSumCh: 116}
	processPmtIds(event, config)

	if event.ExtTrgWaveform == nil || !reflect.DeepEqual(*event.ExtTrgWaveform, []int16{3, 4}) {
		t.Errorf("ExtTrgWaveform = %v, want [3 4]", event.ExtTrgWaveform)
	}
	if event.PmtSumWaveform == nil || !reflect.DeepEqual(*event.PmtSumWaveform, []int16{5, 6}) {
		t.Errorf("PmtSumWaveform = %v, want [5 6]", event.PmtSumWaveform)
	}
	if event.PmtSumBaseline != 30 {
		t.Errorf("PmtSumBaseline = %d, want 30", event.PmtSumBaseline)
	}
	if _, ok := event.PmtWaveforms[115]; ok {
		t.Error("ext trigger channel still present in PmtWaveforms")
	}
	if _, ok := event.PmtWaveforms[116]; ok {
		t.Error("pmt sum channel still present in PmtWaveforms")
	}
	if len(event.PmtWaveforms) != 1 || len(event.Baselines) != 1 {
		t.Errorf("remaining waveforms/baselines = %d/%d, want 1/1",
			len(event.PmtWaveforms), len(event.Baselines))
	}
}

func TestProcessPmtIdsDualMode(t *testing.T) {
	event := newTestEvent()
	event.PmtConfig.DualMode = true
	event.PmtWaveforms[100] = []int16{1}
	event.PmtWaveforms[112] = []int16{2} // dual of 100
	event.Baselines[100] = 10
	event.Baselines[112] = 20

	// Ext trigger / sum channels out of the way
	processPmtIds(event, Configuration{ExtTrigger: -1, PmtSumCh: -1})

	if !reflect.DeepEqual(event.BlrWaveforms[100], []int16{2}) {
		t.Errorf("BlrWaveforms[100] = %v, want [2]", event.BlrWaveforms[100])
	}
	if event.BlrBaselines[100] != 20 {
		t.Errorf("BlrBaselines[100] = %d, want 20", event.BlrBaselines[100])
	}
	if _, ok := event.PmtWaveforms[112]; ok {
		t.Error("dual channel 112 still present in PmtWaveforms")
	}
	if !reflect.DeepEqual(event.PmtWaveforms[100], []int16{1}) {
		t.Errorf("PmtWaveforms[100] = %v, want [1]", event.PmtWaveforms[100])
	}
}

func TestProcessPmtIdsChannelsHG(t *testing.T) {
	event := newTestEvent()
	event.PmtConfig.ChannelsHG = true
	event.PmtWaveforms[100] = []int16{1} // LG stays
	event.PmtWaveforms[101] = []int16{2} // HG moves to BLR under same ID
	event.Baselines[100] = 10
	event.Baselines[101] = 20

	processPmtIds(event, Configuration{ExtTrigger: -1, PmtSumCh: -1})

	if !reflect.DeepEqual(event.BlrWaveforms[101], []int16{2}) {
		t.Errorf("BlrWaveforms[101] = %v, want [2]", event.BlrWaveforms[101])
	}
	if event.BlrBaselines[101] != 20 {
		t.Errorf("BlrBaselines[101] = %d, want 20", event.BlrBaselines[101])
	}
	if _, ok := event.PmtWaveforms[101]; ok {
		t.Error("HG channel 101 still present in PmtWaveforms")
	}
}

func TestDecodeChargeIndiaPmtCompressed(t *testing.T) {
	root := testHuffmanTree(123456)

	// Two channels at waveform positions 0 and 1; at time 1 the decoder uses
	// waveform[time-1] as the prediction base.
	wf0 := []int16{5, 0}
	wf1 := []int16{7, 0}
	wfPointers := []*[]int16{&wf0, &wf1}
	chPositions := []uint16{0, 1}

	// Bits: "01" (+1) for ch0, "001" (-1) for ch1 -> 01001 at bits 31..27
	data := []uint16{0x4800, 0x0000}
	currentBit := 31

	decodeChargeIndiaPmtCompressed(data, 0, wfPointers, &currentBit, root, chPositions, 1)

	if wf0[1] != 6 {
		t.Errorf("ch0 waveform[1] = %d, want 6", wf0[1])
	}
	if wf1[1] != 6 {
		t.Errorf("ch1 waveform[1] = %d, want 6", wf1[1])
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
