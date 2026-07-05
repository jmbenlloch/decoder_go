package decoder

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// dbFixture mirrors the JSON written by scripts/dump_db_fixture.sh: a
// snapshot of the DB tables the decoder needs, so fixture tests run without
// MySQL access.
type dbFixture struct {
	HuffmanPmt []struct {
		Value int    `json:"value"`
		Code  string `json:"code"`
	} `json:"huffman_pmt"`
	HuffmanSipm []struct {
		Value int    `json:"value"`
		Code  string `json:"code"`
	} `json:"huffman_sipm"`
	FecElecIDBase []struct {
		FecID      uint16 `json:"fec_id"`
		BaseElecID uint16 `json:"base_elecid"`
	} `json:"fec_elecid_base"`
	ChannelMapping []struct {
		ElecID   int    `json:"elec_id"`
		SensorID int    `json:"sensor_id"`
		Label    string `json:"label"`
	} `json:"channel_mapping"`
}

// loadDBFixture installs the huffman trees and FEC elecID bases from a JSON
// fixture into the package globals normally filled by LoadDatabase, restoring
// the previous values on test cleanup.
func loadDBFixture(t *testing.T, path string) dbFixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading DB fixture: %v", err)
	}
	var fixture dbFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parsing DB fixture: %v", err)
	}

	oldPmts, oldSipms, oldBase := huffmanCodesPmts, huffmanCodesSipms, fecElecIDBase
	t.Cleanup(func() {
		huffmanCodesPmts, huffmanCodesSipms, fecElecIDBase = oldPmts, oldSipms, oldBase
	})

	huffmanCodesPmts = &HuffmanNode{}
	for _, hc := range fixture.HuffmanPmt {
		parse_huffman_line(int32(hc.Value), hc.Code, huffmanCodesPmts)
	}
	huffmanCodesSipms = &HuffmanNode{}
	for _, hc := range fixture.HuffmanSipm {
		parse_huffman_line(int32(hc.Value), hc.Code, huffmanCodesSipms)
	}
	fecElecIDBase = make(map[uint16]uint16)
	for _, fb := range fixture.FecElecIDBase {
		fecElecIDBase[fb.FecID] = fb.BaseElecID
	}
	return fixture
}

// setTestConfiguration swaps the package configuration and restores the
// TestMain zero-value baseline on cleanup.
func setTestConfiguration(t *testing.T, config Configuration) {
	t.Helper()
	old := GetConfiguration()
	SetConfiguration(config)
	t.Cleanup(func() { SetConfiguration(old) })
}

func readFixtureEvents(t *testing.T, path string, n int) []struct {
	Header EventHeaderStruct
	Data   []byte
} {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer file.Close()

	events := make([]struct {
		Header EventHeaderStruct
		Data   []byte
	}, 0, n)
	for i := 0; i < n; i++ {
		header, data, err := ReadEventFromFile(file)
		if err != nil {
			t.Fatalf("reading fixture event %d: %v", i, err)
		}
		events = append(events, struct {
			Header EventHeaderStruct
			Data   []byte
		}{header, data})
	}
	return events
}

func sumWaveform(wf []int16) int64 {
	var sum int64
	for _, v := range wf {
		sum += int64(v)
	}
	return sum
}

// The fixture holds the first two physics events of DEMO++ run 15022
// (PMTs + SiPMs, compressed), extracted with scripts/extract_rd_events.py.
// Golden values below were read from the known-good decoder output
// run_15022.gdc1demo.demopp.00000.h5 (events table, DataPMT/DataSiPM mappings
// and pmtrwf/sipmrwf waveforms for the corresponding sensor rows).
const demoFixture = "testdata/run_15022_2evt.rd"
const demoDBFixture = "testdata/demopp_15022_db.json"

func TestReadEventFromFileFixture(t *testing.T) {
	events := readFixtureEvents(t, demoFixture, 2)

	for i, want := range []uint32{1, 2} {
		header := events[i].Header
		if EventIdGetNbInRun(header.EventId) != want {
			t.Errorf("event %d ID = %d, want %d", i, EventIdGetNbInRun(header.EventId), want)
		}
		if header.EventRunNb != 15022 {
			t.Errorf("event %d run = %d, want 15022", i, header.EventRunNb)
		}
		if !ValidEvent(header) {
			t.Errorf("event %d type = %d, not a valid physics/calibration event", i, header.EventType)
		}
		if header.EventMagic != EVENT_MAGIC_NUMBER {
			t.Errorf("event %d magic = %#x", i, header.EventMagic)
		}
		wantPayload := int(header.EventSize) - 80
		if len(events[i].Data) != wantPayload {
			t.Errorf("event %d payload = %d bytes, want %d", i, len(events[i].Data), wantPayload)
		}
	}

	// After the last event only EOF is left
	file, err := os.Open(demoFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ReadEventFromFile(file)
	ReadEventFromFile(file)
	if _, _, err := ReadEventFromFile(file); err == nil {
		t.Error("expected error after last event, got nil")
	}
}

func TestReadGDCDemoFixture(t *testing.T) {
	loadDBFixture(t, demoDBFixture)
	setTestConfiguration(t, Configuration{
		ReadPMTs:    true,
		ReadSiPMs:   true,
		ReadTrigger: true,
		ReadFibers:  true,
		ExtTrigger:  -1,
		PmtSumCh:    -1,
	})

	events := readFixtureEvents(t, demoFixture, 2)

	event, err := ReadGDC(events[0].Data, events[0].Header)
	if err != nil {
		t.Fatalf("ReadGDC event 1: %v", err)
	}
	if event.Error {
		t.Fatal("event 1 flagged as error")
	}
	if event.EventID != 1 || event.RunNumber != 15022 {
		t.Errorf("EventID/Run = %d/%d, want 1/15022", event.EventID, event.RunNumber)
	}
	if event.Timestamp != 1782922854775 {
		t.Errorf("Timestamp = %d, want 1782922854775", event.Timestamp)
	}

	// PMTs: 3 channels at elecIDs 200, 202, 204 with 64000 samples
	if len(event.PmtWaveforms) != 3 {
		t.Fatalf("PMT channels = %d, want 3", len(event.PmtWaveforms))
	}
	for _, elecID := range []uint16{200, 202, 204} {
		wf, ok := event.PmtWaveforms[elecID]
		if !ok {
			t.Fatalf("PMT elecID %d missing; have %v", elecID, mapKeys(event.PmtWaveforms))
		}
		if len(wf) != 64000 {
			t.Errorf("PMT %d samples = %d, want 64000", elecID, len(wf))
		}
	}
	if got := event.PmtWaveforms[200][:6]; !reflect.DeepEqual(got, []int16{3088, 3088, 3088, 3088, 3088, 3089}) {
		t.Errorf("PMT 200 first samples = %v", got)
	}
	if sum := sumWaveform(event.PmtWaveforms[200]); sum != 197658313 {
		t.Errorf("PMT 200 waveform sum = %d, want 197658313", sum)
	}
	wantBaselines := map[uint16]uint16{200: 3088, 202: 3018, 204: 3014}
	if !reflect.DeepEqual(event.Baselines, wantBaselines) {
		t.Errorf("PMT baselines = %v, want %v", event.Baselines, wantBaselines)
	}

	// SiPMs: 256 channels with 1600 samples
	if len(event.SipmWaveforms) != 256 {
		t.Fatalf("SiPM channels = %d, want 256", len(event.SipmWaveforms))
	}
	wf12038, ok := event.SipmWaveforms[12038]
	if !ok {
		t.Fatal("SiPM elecID 12038 missing")
	}
	if len(wf12038) != 1600 {
		t.Errorf("SiPM 12038 samples = %d, want 1600", len(wf12038))
	}
	if got := wf12038[:8]; !reflect.DeepEqual(got, []int16{50, 47, 50, 49, 66, 53, 50, 48}) {
		t.Errorf("SiPM 12038 first samples = %v", got)
	}
	if sum := sumWaveform(wf12038); sum != 80699 {
		t.Errorf("SiPM 12038 waveform sum = %d, want 80699", sum)
	}

	// No fibers in DEMO++ data
	if len(event.FibersLG) != 0 || len(event.FibersHG) != 0 {
		t.Errorf("fibers = %d LG / %d HG channels, want none",
			len(event.FibersLG), len(event.FibersHG))
	}

	// Second event
	event2, err := ReadGDC(events[1].Data, events[1].Header)
	if err != nil {
		t.Fatalf("ReadGDC event 2: %v", err)
	}
	if event2.EventID != 2 || event2.Timestamp != 1782922854875 {
		t.Errorf("event 2 ID/Timestamp = %d/%d, want 2/1782922854875",
			event2.EventID, event2.Timestamp)
	}
	if got := event2.PmtWaveforms[200][:4]; !reflect.DeepEqual(got, []int16{3087, 3089, 3089, 3089}) {
		t.Errorf("event 2 PMT 200 first samples = %v", got)
	}
}

// The fixture holds the first two physics events of HDDEMO run 616 (fibers
// LG/HG + SiPMs, raw mode), extracted with scripts/extract_rd_events.py.
const hddemoFixture = "testdata/run_616_2evt.rd"
const hddemoDBFixture = "testdata/hddemo_616_db.json"

// TestReadGDCHddemo616 decodes the first two events of HDDEMO run 616 from
// the committed fixture. Golden values come from the known-good
// run_616_nodb.h5.
func TestReadGDCHddemo616(t *testing.T) {
	loadDBFixture(t, hddemoDBFixture)
	setTestConfiguration(t, Configuration{
		ReadPMTs:    true,
		ReadSiPMs:   true,
		ReadTrigger: true,
		ReadFibers:  true,
		ExtTrigger:  -1,
		PmtSumCh:    -1,
	})

	events := readFixtureEvents(t, hddemoFixture, 2)

	event, err := ReadGDC(events[0].Data, events[0].Header)
	if err != nil {
		t.Fatalf("ReadGDC event 1: %v", err)
	}
	if event.EventID != 1 || event.RunNumber != 616 {
		t.Errorf("EventID/Run = %d/%d, want 1/616", event.EventID, event.RunNumber)
	}
	if event.Timestamp != 1777558171242 {
		t.Errorf("Timestamp = %d, want 1777558171242", event.Timestamp)
	}

	// Fibers: 36 LG + 36 HG channels (even elecIDs 100..322), 64000 samples
	if len(event.FibersLG) != 36 || len(event.FibersHG) != 36 {
		t.Fatalf("fibers = %d LG / %d HG channels, want 36/36",
			len(event.FibersLG), len(event.FibersHG))
	}
	lg100 := event.FibersLG[100]
	if len(lg100) != 64000 {
		t.Errorf("fiber LG 100 samples = %d, want 64000", len(lg100))
	}
	if got := lg100[:8]; !reflect.DeepEqual(got, []int16{2955, 2957, 2961, 2960, 2958, 2956, 2958, 2953}) {
		t.Errorf("fiber LG 100 first samples = %v", got)
	}
	if sum := sumWaveform(lg100); sum != 189288838 {
		t.Errorf("fiber LG 100 sum = %d, want 189288838", sum)
	}
	hg100 := event.FibersHG[100]
	if got := hg100[:8]; !reflect.DeepEqual(got, []int16{3214, 3375, 3581, 3599, 3588, 3388, 3258, 3086}) {
		t.Errorf("fiber HG 100 first samples = %v", got)
	}
	if sum := sumWaveform(hg100); sum != 201383399 {
		t.Errorf("fiber HG 100 sum = %d, want 201383399", sum)
	}

	// SiPMs: 832 channels, 1600 samples; elecID 1000 is the first row of the
	// NoDB-sorted sipmrwf
	if len(event.SipmWaveforms) != 832 {
		t.Fatalf("SiPM channels = %d, want 832", len(event.SipmWaveforms))
	}
	s1000 := event.SipmWaveforms[1000]
	if len(s1000) != 1600 {
		t.Errorf("SiPM 1000 samples = %d, want 1600", len(s1000))
	}
	if got := s1000[:8]; !reflect.DeepEqual(got, []int16{570, 575, 580, 567, 576, 581, 571, 575}) {
		t.Errorf("SiPM 1000 first samples = %v", got)
	}
	if sum := sumWaveform(s1000); sum != 920557 {
		t.Errorf("SiPM 1000 sum = %d, want 920557", sum)
	}

	// Second event
	event2, err := ReadGDC(events[1].Data, events[1].Header)
	if err != nil {
		t.Fatalf("ReadGDC event 2: %v", err)
	}
	if event2.EventID != 2 || event2.Timestamp != 1777558171342 {
		t.Errorf("event 2 ID/Timestamp = %d/%d, want 2/1777558171342",
			event2.EventID, event2.Timestamp)
	}
}

func mapKeys(m map[uint16][]int16) []uint16 {
	keys := make([]uint16, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
