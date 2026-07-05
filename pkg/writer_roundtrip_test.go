package decoder

import (
	"path/filepath"
	"reflect"
	"testing"

	hdf5 "github.com/next-exp/hdf5-go"
)

// --- read-back helpers -----------------------------------------------------

func openForRead(t *testing.T, path string) *hdf5.File {
	t.Helper()
	f, err := hdf5.OpenFile(path, hdf5.F_ACC_RDONLY)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func datasetDims(t *testing.T, f *hdf5.File, path string) []uint {
	t.Helper()
	dset, err := f.OpenDataset(path)
	if err != nil {
		t.Fatalf("opening dataset %s: %v", path, err)
	}
	// Note: datasets returned by OpenDataset must not be Close()d here:
	// hdf5-go's Dataset.Close panics on the nil datatype it stores for
	// opened (vs created) datasets. They are released with the file.
	dims, _, err := dset.Space().SimpleExtentDims()
	if err != nil {
		t.Fatalf("dims of %s: %v", path, err)
	}
	return dims
}

func readInt16Dataset(t *testing.T, f *hdf5.File, path string) ([]int16, []uint) {
	t.Helper()
	dset, err := f.OpenDataset(path)
	if err != nil {
		t.Fatalf("opening dataset %s: %v", path, err)
	}
	// Note: datasets returned by OpenDataset must not be Close()d here:
	// hdf5-go's Dataset.Close panics on the nil datatype it stores for
	// opened (vs created) datasets. They are released with the file.
	dims, _, err := dset.Space().SimpleExtentDims()
	if err != nil {
		t.Fatalf("dims of %s: %v", path, err)
	}
	n := uint(1)
	for _, d := range dims {
		n *= d
	}
	data := make([]int16, n)
	if n > 0 {
		if err := dset.Read(&data); err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
	}
	return data, dims
}

func readTableRows[T any](t *testing.T, f *hdf5.File, path string) []T {
	t.Helper()
	dset, err := f.OpenDataset(path)
	if err != nil {
		t.Fatalf("opening table %s: %v", path, err)
	}
	// See note in readInt16Dataset about not closing opened datasets.
	dims, _, err := dset.Space().SimpleExtentDims()
	if err != nil {
		t.Fatalf("dims of %s: %v", path, err)
	}
	rows := make([]T, dims[0])
	if dims[0] > 0 {
		if err := dset.Read(&rows); err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
	}
	return rows
}

// --- test event construction ------------------------------------------------

func rampWaveform(start, n int) []int16 {
	wf := make([]int16, n)
	for i := range wf {
		wf[i] = int16(start + i)
	}
	return wf
}

// buildWriterTestEvent creates a fully populated event with 2 PMTs (+BLR),
// 2 SiPMs, 1 LG + 1 HG fiber, ext-trigger and PMT-sum waveforms and trigger
// data, using `offset` to vary sample values between events.
func buildWriterTestEvent(eventID uint32, offset int) *EventType {
	event := newTestEvent()
	event.RunNumber = 42
	event.EventID = eventID
	event.Timestamp = uint64(111 * eventID)
	event.TriggerType = 3

	event.PmtWaveforms[100] = rampWaveform(offset, 16)
	event.PmtWaveforms[102] = rampWaveform(offset+100, 16)
	event.Baselines[100] = 3000
	event.Baselines[102] = 3010
	event.BlrWaveforms[100] = rampWaveform(offset+200, 16)
	event.BlrWaveforms[102] = rampWaveform(offset+300, 16)
	event.BlrBaselines[100] = 3020
	event.BlrBaselines[102] = 3030

	event.SipmWaveforms[1000] = rampWaveform(offset+400, 4)
	event.SipmWaveforms[1063] = rampWaveform(offset+500, 4)

	event.FibersLG[500] = rampWaveform(offset+600, 8)
	event.FibersHG[500] = rampWaveform(offset+700, 8)
	event.FiberBaselinesLG[500] = 400
	event.FiberBaselinesHG[500] = 410

	ext := rampWaveform(offset+800, 5)
	event.ExtTrgWaveform = &ext
	sum := rampWaveform(offset+900, 5)
	event.PmtSumWaveform = &sum
	event.PmtSumBaseline = 1234

	event.TriggerConfig = TriggerData{
		TriggerType:  3,
		TriggerLost1: 11,
		TriggerLost2: 22,
		TriggerMask:  0x2ABCD,
		// 100 is a PMT; 501 is the odd (HG) elecID of fiber 500
		TrgChannels: []uint16{100, 501},
	}
	return event
}

// --- tests -------------------------------------------------------------------

// TestWriterRoundTripNoDB writes two events in NoDB mode and reads every
// dataset back, verifying shapes, ordering and content.
func TestWriterRoundTripNoDB(t *testing.T) {
	setTestConfiguration(t, Configuration{
		NoDB:             true,
		WriteData:        true,
		CompressionLevel: 4,
	})

	path := filepath.Join(t.TempDir(), "nodb.h5")
	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	writer.WriteEvent(buildWriterTestEvent(7, 0))
	writer.WriteEvent(buildWriterTestEvent(8, 1000))
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f := openForRead(t, path)

	// Run group
	events := readTableRows[EventDataHDF5](t, f, "/Run/events")
	if len(events) != 2 || events[0].evt_number != 7 || events[1].evt_number != 8 {
		t.Errorf("events = %+v, want evt_numbers 7, 8", events)
	}
	if events[0].timestamp != 777 || events[1].timestamp != 888 {
		t.Errorf("timestamps = %d, %d, want 777, 888", events[0].timestamp, events[1].timestamp)
	}
	runInfo := readTableRows[RunInfoHDF5](t, f, "/Run/runInfo")
	if len(runInfo) != 1 || runInfo[0].run_number != 42 {
		t.Errorf("runInfo = %+v, want run 42", runInfo)
	}

	// Sensor mappings: NoDB -> ordered by elecID, sensorID -1
	pmtMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataPMT")
	wantPmtMap := []SensorMappingHDF5{{100, -1}, {102, -1}}
	if !reflect.DeepEqual(pmtMap, wantPmtMap) {
		t.Errorf("DataPMT = %v, want %v", pmtMap, wantPmtMap)
	}
	blrMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataBLR")
	if !reflect.DeepEqual(blrMap, wantPmtMap) {
		t.Errorf("DataBLR = %v, want %v", blrMap, wantPmtMap)
	}
	sipmMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataSiPM")
	if !reflect.DeepEqual(sipmMap, []SensorMappingHDF5{{1000, -1}, {1063, -1}}) {
		t.Errorf("DataSiPM = %v", sipmMap)
	}
	fiberLGMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataFiberLG")
	if !reflect.DeepEqual(fiberLGMap, []SensorMappingHDF5{{500, -1}}) {
		t.Errorf("DataFiberLG = %v", fiberLGMap)
	}

	// PMT waveforms: (2 events, 2 channels, 16 samples), elecID order
	pmtData, pmtDims := readInt16Dataset(t, f, "/RD/pmtrwf")
	if !reflect.DeepEqual(pmtDims, []uint{2, 2, 16}) {
		t.Fatalf("pmtrwf dims = %v, want [2 2 16]", pmtDims)
	}
	if !reflect.DeepEqual(pmtData[0:16], rampWaveform(0, 16)) {
		t.Errorf("evt0 pmt 100 = %v", pmtData[0:16])
	}
	if !reflect.DeepEqual(pmtData[16:32], rampWaveform(100, 16)) {
		t.Errorf("evt0 pmt 102 = %v", pmtData[16:32])
	}
	if !reflect.DeepEqual(pmtData[32:48], rampWaveform(1000, 16)) {
		t.Errorf("evt1 pmt 100 = %v", pmtData[32:48])
	}

	pmtBase, baseDims := readInt16Dataset(t, f, "/RD/pmt_baselines")
	if !reflect.DeepEqual(baseDims, []uint{2, 2}) || !reflect.DeepEqual(pmtBase, []int16{3000, 3010, 3000, 3010}) {
		t.Errorf("pmt_baselines = %v dims %v", pmtBase, baseDims)
	}

	// BLR waveforms follow the BLR mapping order in NoDB mode
	blrData, blrDims := readInt16Dataset(t, f, "/RD/pmtblr")
	if !reflect.DeepEqual(blrDims, []uint{2, 2, 16}) {
		t.Fatalf("pmtblr dims = %v", blrDims)
	}
	if !reflect.DeepEqual(blrData[0:16], rampWaveform(200, 16)) {
		t.Errorf("evt0 blr 100 = %v", blrData[0:16])
	}

	// SiPMs
	sipmData, sipmDims := readInt16Dataset(t, f, "/RD/sipmrwf")
	if !reflect.DeepEqual(sipmDims, []uint{2, 2, 4}) {
		t.Fatalf("sipmrwf dims = %v", sipmDims)
	}
	if !reflect.DeepEqual(sipmData[0:4], rampWaveform(400, 4)) {
		t.Errorf("evt0 sipm 1000 = %v", sipmData[0:4])
	}
	if !reflect.DeepEqual(sipmData[8:12], rampWaveform(1400, 4)) {
		t.Errorf("evt1 sipm 1000 = %v", sipmData[8:12])
	}

	// Fibers
	lgData, lgDims := readInt16Dataset(t, f, "/RD/fiberrwf_lg")
	if !reflect.DeepEqual(lgDims, []uint{2, 1, 8}) {
		t.Fatalf("fiberrwf_lg dims = %v", lgDims)
	}
	if !reflect.DeepEqual(lgData[0:8], rampWaveform(600, 8)) {
		t.Errorf("evt0 fiber lg = %v", lgData[0:8])
	}
	hgData, _ := readInt16Dataset(t, f, "/RD/fiberrwf_hg")
	if !reflect.DeepEqual(hgData[0:8], rampWaveform(700, 8)) {
		t.Errorf("evt0 fiber hg = %v", hgData[0:8])
	}
	lgBase, _ := readInt16Dataset(t, f, "/RD/fiber_baselines_lg")
	if !reflect.DeepEqual(lgBase, []int16{400, 400}) {
		t.Errorf("fiber_baselines_lg = %v", lgBase)
	}

	// Ext trigger / PMT sum
	extData, extDims := readInt16Dataset(t, f, "/RD/ext_pmt")
	if !reflect.DeepEqual(extDims, []uint{2, 5}) || !reflect.DeepEqual(extData[0:5], rampWaveform(800, 5)) {
		t.Errorf("ext_pmt = %v dims %v", extData, extDims)
	}
	sumData, _ := readInt16Dataset(t, f, "/RD/pmt_sum")
	if !reflect.DeepEqual(sumData[0:5], rampWaveform(900, 5)) {
		t.Errorf("pmt_sum = %v", sumData)
	}
	sumBase, _ := readInt16Dataset(t, f, "/RD/pmt_sum_baseline")
	if !reflect.DeepEqual(sumBase, []int16{1234, 1234}) {
		t.Errorf("pmt_sum_baseline = %v", sumBase)
	}

	// Trigger: type, lost counters, active-channel matrix over sorted
	// PMT+fiber elecIDs [100 102 500]; TrgChannels 100 and 501->500
	trgTypes := readTableRows[TriggerTypeHDF5](t, f, "/Trigger/trigger")
	if len(trgTypes) != 2 || trgTypes[0].trigger_type != 3 {
		t.Errorf("trigger types = %+v", trgTypes)
	}
	trgLost := readTableRows[TriggerLostHDF5](t, f, "/Trigger/triggerLost")
	if len(trgLost) != 2 || trgLost[0].triggerLost1 != 11 || trgLost[0].triggerLost2 != 22 {
		t.Errorf("triggerLost = %+v", trgLost)
	}
	trgEvents, trgDims := readInt16Dataset(t, f, "/Trigger/events")
	if !reflect.DeepEqual(trgDims, []uint{2, 3}) {
		t.Fatalf("Trigger/events dims = %v", trgDims)
	}
	if !reflect.DeepEqual(trgEvents, []int16{1, 0, 1, 1, 0, 1}) {
		t.Errorf("Trigger/events = %v, want [1 0 1 1 0 1]", trgEvents)
	}

	// Trigger configuration: one row per scalar TriggerData field (22),
	// written once
	trgConf := readTableRows[TriggerParamsHDF5](t, f, "/Trigger/configuration")
	if len(trgConf) != 22 {
		t.Fatalf("trigger configuration rows = %d, want 22", len(trgConf))
	}
	if string(trgConf[0].paramStr[:11]) != "triggerType" || trgConf[0].value != 3 {
		t.Errorf("first param = %q value %d", trgConf[0].paramStr, trgConf[0].value)
	}
}

// TestWriterRoundTripDB verifies DB-mode ordering: mappings and waveform rows
// follow sensorID order from the DB channel map, and trigger channels are
// resolved through it.
func TestWriterRoundTripDB(t *testing.T) {
	setTestConfiguration(t, Configuration{
		NoDB:             false,
		WriteData:        true,
		CompressionLevel: 4,
	})

	oldMap := sensorsMap
	t.Cleanup(func() { sensorsMap = oldMap })
	sensorsMap = SensorsMap{
		Pmts: SensorMapping{
			// sensorID order inverts elecID order on purpose
			ToSensorID: map[uint16]uint16{100: 3, 102: 2},
			ToElecID:   map[uint16]uint16{3: 100, 2: 102},
		},
		Sipms: SensorMapping{
			ToSensorID: map[uint16]uint16{1000: 5001, 1063: 5000},
			ToElecID:   map[uint16]uint16{5001: 1000, 5000: 1063},
		},
		Fibers: SensorMapping{
			ToSensorID: map[uint16]uint16{500: 7000},
			ToElecID:   map[uint16]uint16{7000: 500},
		},
	}

	path := filepath.Join(t.TempDir(), "db.h5")
	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	event := buildWriterTestEvent(7, 0)
	// BLR handling in DB mode is a known open issue (B2 in TESTING_PLAN.md):
	// keep it out of this test
	event.BlrWaveforms = map[uint16][]int16{}
	event.BlrBaselines = map[uint16]uint16{}
	writer.WriteEvent(event)
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f := openForRead(t, path)

	pmtMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataPMT")
	wantPmtMap := []SensorMappingHDF5{{102, 2}, {100, 3}}
	if !reflect.DeepEqual(pmtMap, wantPmtMap) {
		t.Errorf("DataPMT = %v, want %v (sensorID order)", pmtMap, wantPmtMap)
	}

	// Waveform rows must follow sensorID order: row 0 = elecID 102
	pmtData, pmtDims := readInt16Dataset(t, f, "/RD/pmtrwf")
	if !reflect.DeepEqual(pmtDims, []uint{1, 2, 16}) {
		t.Fatalf("pmtrwf dims = %v", pmtDims)
	}
	if !reflect.DeepEqual(pmtData[0:16], rampWaveform(100, 16)) {
		t.Errorf("row 0 = %v, want elecID 102 data (ramp from 100)", pmtData[0:16])
	}
	if !reflect.DeepEqual(pmtData[16:32], rampWaveform(0, 16)) {
		t.Errorf("row 1 = %v, want elecID 100 data (ramp from 0)", pmtData[16:32])
	}

	sipmMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataSiPM")
	if !reflect.DeepEqual(sipmMap, []SensorMappingHDF5{{1063, 5000}, {1000, 5001}}) {
		t.Errorf("DataSiPM = %v", sipmMap)
	}
	sipmData, _ := readInt16Dataset(t, f, "/RD/sipmrwf")
	if !reflect.DeepEqual(sipmData[0:4], rampWaveform(500, 4)) {
		t.Errorf("sipm row 0 = %v, want elecID 1063 data", sipmData[0:4])
	}

	fiberLGMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataFiberLG")
	if !reflect.DeepEqual(fiberLGMap, []SensorMappingHDF5{{500, 7000}}) {
		t.Errorf("DataFiberLG = %v", fiberLGMap)
	}

	// Trigger channels over sorted sensorIDs [2 3 7000]:
	// elecID 100 -> sensorID 3 -> pos 1; 501 -> fiber 500 -> 7000 -> pos 2
	trgEvents, trgDims := readInt16Dataset(t, f, "/Trigger/events")
	if !reflect.DeepEqual(trgDims, []uint{1, 3}) {
		t.Fatalf("Trigger/events dims = %v", trgDims)
	}
	if !reflect.DeepEqual(trgEvents, []int16{0, 1, 1}) {
		t.Errorf("Trigger/events = %v, want [0 1 1]", trgEvents)
	}
}

// TestEndToEndNoDBFixture runs the full pipeline (raw file -> ReadGDC ->
// Writer) on the committed DEMO++ fixture in NoDB mode and cross-checks the
// resulting HDF5 with golden values from the known-good output.
func TestEndToEndNoDBFixture(t *testing.T) {
	loadDBFixture(t, demoDBFixture)
	setTestConfiguration(t, Configuration{
		NoDB:             true,
		WriteData:        true,
		CompressionLevel: 4,
		ReadPMTs:         true,
		ReadSiPMs:        true,
		ReadTrigger:      true,
		ReadFibers:       true,
		ExtTrigger:       -1,
		PmtSumCh:         -1,
	})

	path := filepath.Join(t.TempDir(), "e2e.h5")
	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	events := readFixtureEvents(t, demoFixture, 2)
	for i := range events {
		event, err := ReadGDC(events[i].Data, events[i].Header)
		if err != nil {
			t.Fatalf("ReadGDC event %d: %v", i, err)
		}
		ProcessDecodedEvent(event, GetConfiguration(), writer, nil)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f := openForRead(t, path)

	evts := readTableRows[EventDataHDF5](t, f, "/Run/events")
	if len(evts) != 2 || evts[0].evt_number != 1 || evts[1].evt_number != 2 {
		t.Fatalf("events = %+v", evts)
	}
	if evts[0].timestamp != 1782922854775 {
		t.Errorf("timestamp = %d, want 1782922854775", evts[0].timestamp)
	}

	pmtMap := readTableRows[SensorMappingHDF5](t, f, "/Sensors/DataPMT")
	wantPmtMap := []SensorMappingHDF5{{200, -1}, {202, -1}, {204, -1}}
	if !reflect.DeepEqual(pmtMap, wantPmtMap) {
		t.Errorf("DataPMT = %v, want %v", pmtMap, wantPmtMap)
	}

	pmtData, pmtDims := readInt16Dataset(t, f, "/RD/pmtrwf")
	if !reflect.DeepEqual(pmtDims, []uint{2, 3, 64000}) {
		t.Fatalf("pmtrwf dims = %v, want [2 3 64000]", pmtDims)
	}
	// Golden: run_15022 known-good output, event 0, elecID 200 (row 0)
	var sum int64
	for _, v := range pmtData[:64000] {
		sum += int64(v)
	}
	if sum != 197658313 {
		t.Errorf("evt0 pmt row0 sum = %d, want 197658313", sum)
	}

	sipmDims := datasetDims(t, f, "/RD/sipmrwf")
	if !reflect.DeepEqual(sipmDims, []uint{2, 256, 1600}) {
		t.Errorf("sipmrwf dims = %v, want [2 256 1600]", sipmDims)
	}

	baselines, baseDims := readInt16Dataset(t, f, "/RD/pmt_baselines")
	if !reflect.DeepEqual(baseDims, []uint{2, 3}) || baselines[0] != 3088 || baselines[1] != 3018 || baselines[2] != 3014 {
		t.Errorf("pmt_baselines = %v dims %v, want [3088 3018 3014 ...]", baselines, baseDims)
	}
}
