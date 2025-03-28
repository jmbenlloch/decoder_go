package decoder

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	hdf5 "github.com/next-exp/hdf5-go"
	"golang.org/x/exp/maps"
)

type Writer struct {
	File               *hdf5.File
	Filename           string
	FirstEvt           bool
	RunGroup           *hdf5.Group
	RDGroup            *hdf5.Group
	SensorsGroup       *hdf5.Group
	TriggerGroup       *hdf5.Group
	EventTable         *hdf5.Dataset
	RunInfoTable       *hdf5.Dataset
	TriggerParamsTable *hdf5.Dataset
	TriggerTypeTable   *hdf5.Dataset
	TriggerLostTable   *hdf5.Dataset
	TriggerChannels    *hdf5.Dataset
	PmtMappingTable    *hdf5.Dataset
	SipmMappingTable   *hdf5.Dataset
	PmtWaveforms       *hdf5.Dataset
	BlrWaveforms       *hdf5.Dataset
	ExtTrgWaveform     *hdf5.Dataset
	PmtSumWaveform     *hdf5.Dataset
	PmtSumBaseline     *hdf5.Dataset
	SipmWaveforms      *hdf5.Dataset
	Baselines          *hdf5.Dataset
	BlrBaselines       *hdf5.Dataset
	EvtCounter         int
}

const N_TRG_CH = 64

func NewWriter(filename string) *Writer {
	// Set string size for HDF5
	hdf5.SetStringLength(STRLEN)

	// So far we are not using Blosc
	if configuration.UseBlosc {
		blosc_version, blosc_date, err := hdf5.RegisterBlosc()
		fmt.Println("Blosc version: ", blosc_version, " date: ", blosc_date)
		if err != nil {
			logger.Error(err.Error())
		}
	}

	writer := &Writer{}
	fmt.Println("hdf5writer: Creating file: ", filename)
	writer.File = openFile(filename)
	writer.Filename = filename
	writer.RunGroup = createGroup(writer.File, "Run")
	writer.RDGroup = createGroup(writer.File, "RD")
	writer.SensorsGroup = createGroup(writer.File, "Sensors")
	writer.TriggerGroup = createGroup(writer.File, "Trigger")
	writer.EventTable = createTable(writer.RunGroup, "events", EventDataHDF5{})
	writer.RunInfoTable = createTable(writer.RunGroup, "runInfo", RunInfoHDF5{})
	writer.TriggerParamsTable = createTable(writer.TriggerGroup, "configuration", TriggerParamsHDF5{})
	writer.TriggerLostTable = createTable(writer.TriggerGroup, "triggerLost", TriggerLostHDF5{})
	writer.TriggerTypeTable = createTable(writer.TriggerGroup, "trigger", TriggerTypeHDF5{})
	writer.TriggerChannels = create2dArray(writer.TriggerGroup, "events", N_TRG_CH)
	writer.PmtMappingTable = createTable(writer.SensorsGroup, "DataPMT", SensorMappingHDF5{})
	writer.SipmMappingTable = createTable(writer.SensorsGroup, "DataSiPM", SensorMappingHDF5{})
	writer.EvtCounter = 0
	return writer
}

func sortSensorsBySensorID(sensorsFromElecIDToSensorID map[uint16]uint16) []SensorMappingHDF5 {
	// The array MUST be allocated at creation, if not, HDF5 will panic
	// doing appends will not work
	sorted := make([]SensorMappingHDF5, len(sensorsFromElecIDToSensorID))
	count := 0
	for elecID, sensorID := range sensorsFromElecIDToSensorID {
		sensor := SensorMappingHDF5{
			channel:  int32(elecID),
			sensorID: int32(sensorID),
		}
		sorted[count] = sensor
		count++
	}

	// Sort by sensorID
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].sensorID < sorted[j].sensorID
	})
	return sorted
}

func sortSensorsByElecID(sensors map[uint16][]int16) []SensorMappingHDF5 {
	// The array MUST be allocated at creation, if not, HDF5 will panic
	// doing appends will not work
	nSensors := len(sensors)
	sorted := make([]SensorMappingHDF5, nSensors)
	count := 0
	for elecID, _ := range sensors {
		sensor := SensorMappingHDF5{
			channel:  int32(elecID),
			sensorID: -1,
		}
		sorted[count] = sensor
		count++
	}

	// Sort by sensorID
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].channel < sorted[j].channel
	})
	return sorted
}

func (w *Writer) WriteEvent(event *EventType) {
	// Write event data
	evtTimestamp := EventDataHDF5{
		timestamp:  event.Timestamp,
		evt_number: int32(event.EventID),
	}

	writeEntryToTable(w.TriggerLostTable, TriggerLostHDF5{
		triggerLost1: int32(event.TriggerConfig.TriggerLost1),
		triggerLost2: int32(event.TriggerConfig.TriggerLost2),
	}, w.EvtCounter)

	writeEntryToTable(w.TriggerTypeTable, TriggerTypeHDF5{
		trigger_type: int32(event.TriggerType),
	}, w.EvtCounter)

	var pmtSorted, sipmSorted []SensorMappingHDF5
	var nPmts, nBlrs, nSipms int
	var pmtSamples, sipmSamples int

	if configuration.NoDB {
		pmtSorted = sortSensorsByElecID(event.PmtWaveforms)
		sipmSorted = sortSensorsByElecID(event.SipmWaveforms)
		nPmts = len(event.PmtWaveforms)
		nSipms = len(event.SipmWaveforms)
	} else {
		pmtSorted = sortSensorsBySensorID(sensorsMap.Pmts.ToSensorID)
		sipmSorted = sortSensorsBySensorID(sensorsMap.Sipms.ToSensorID)
		nPmts = len(pmtSorted)
		nSipms = len(sipmSorted)
	}
	nBlrs = len(event.BlrWaveforms)

	if nPmts > 0 {
		if len(event.PmtWaveforms) > 0 {
			randomPmt := maps.Values(event.PmtWaveforms)[0]
			pmtSamples = len(randomPmt)
		} else {
			pmtSamples = 1
		}
	}

	if nSipms > 0 {
		if len(event.SipmWaveforms) > 0 {
			randomSipm := maps.Values(event.SipmWaveforms)[0]
			sipmSamples = len(randomSipm)
		} else {
			sipmSamples = 1
		}
	}

	if !w.FirstEvt {
		writeEntryToTable(w.RunInfoTable, RunInfoHDF5{run_number: int32(event.RunNumber)}, w.EvtCounter)
		writeArrayToTable(w.PmtMappingTable, &pmtSorted, w.EvtCounter)
		writeArrayToTable(w.SipmMappingTable, &sipmSorted, w.EvtCounter)

		w.writeTriggerConfiguration(event.TriggerConfig)

		if nPmts > 0 {
			w.PmtWaveforms = create3dArray(w.RDGroup, "pmtrwf", nPmts, pmtSamples)
			w.Baselines = create2dArray(w.RDGroup, "pmt_baselines", nPmts)
		}
		if nSipms > 0 {
			w.SipmWaveforms = create3dArray(w.RDGroup, "sipmrwf", nSipms, sipmSamples)
		}

		if event.ExtTrgWaveform != nil {
			samples := len(*event.ExtTrgWaveform)
			w.ExtTrgWaveform = create2dArray(w.RDGroup, "ext_pmt", samples)
		}

		if event.PmtSumWaveform != nil {
			samples := len(*event.PmtSumWaveform)
			w.PmtSumWaveform = create2dArray(w.RDGroup, "pmt_sum", samples)
			w.PmtSumBaseline = create2dArray(w.RDGroup, "pmt_sum_baseline", 1)
		}

		if len(event.BlrWaveforms) > 0 {
			w.BlrWaveforms = create3dArray(w.RDGroup, "pmt_blr", nPmts, pmtSamples)
			w.BlrBaselines = create2dArray(w.RDGroup, "blr_baselines", nPmts)
		}

		w.FirstEvt = true
	}

	writeEntryToTable(w.EventTable, evtTimestamp, w.EvtCounter)

	// Write waveforms
	if nPmts > 0 {
		writeWaveforms(w.PmtWaveforms, event.PmtWaveforms, pmtSorted, w.EvtCounter, nPmts, pmtSamples)
		writeBaselines(w.Baselines, event.Baselines, pmtSorted, w.EvtCounter, nPmts)
	}
	if nBlrs > 0 {
		// This uses the same channel order as the PMTs
		// it works well when reading the channel map from DB
		// in no-DB mode, if there is a dual channel of a missing normal channel,
		// it will not be written.
		writeWaveforms(w.BlrWaveforms, event.BlrWaveforms, pmtSorted, w.EvtCounter, nPmts, pmtSamples)
		writeBaselines(w.BlrBaselines, event.BlrBaselines, pmtSorted, w.EvtCounter, nPmts)
	}
	if nSipms > 0 {
		writeWaveforms(w.SipmWaveforms, event.SipmWaveforms, sipmSorted, w.EvtCounter, nSipms, sipmSamples)
	}
	if event.ExtTrgWaveform != nil {
		writeSingleWaveform(w.ExtTrgWaveform, event.ExtTrgWaveform, w.EvtCounter)
	}
	if event.PmtSumWaveform != nil {
		writeSingleWaveform(w.PmtSumWaveform, event.PmtSumWaveform, w.EvtCounter)
		pmtSumBaseline := []int16{int16(event.PmtSumBaseline)}
		writeSingleWaveform(w.PmtSumBaseline, &pmtSumBaseline, w.EvtCounter)
	}

	trgChannels := make([]int16, N_TRG_CH)
	for _, sensor := range event.TriggerConfig.TrgChannels {
		if sensor < N_TRG_CH {
			trgChannels[sensor] = 1
		} else {
			fmt.Println("Trigger channel out of range: ", sensor)
		}
	}
	write2dArray(w.TriggerChannels, &trgChannels, w.EvtCounter, N_TRG_CH)

	w.EvtCounter++
}

func writeWaveforms(dset *hdf5.Dataset, waveforms map[uint16][]int16,
	order []SensorMappingHDF5, evtCounter int, nSensors int, nSamples int) {
	data := make([]int16, nSensors*nSamples)
	for i, sensor := range order {
		// Write only if the corresponding sensor has data
		// if not, the data array will be filled with zeros for that sensor
		if _, ok := waveforms[uint16(sensor.channel)]; !ok {
			continue
		}
		for j, sample := range waveforms[uint16(sensor.channel)] {
			data[i*nSamples+j] = int16(sample)
		}
	}
	write3dArray(dset, &data, evtCounter, nSensors, nSamples)
}

func writeSingleWaveform(dset *hdf5.Dataset, waveform *[]int16, evtCounter int) {
	nSamples := len(*waveform)
	data := make([]int16, nSamples)
	for i, value := range *waveform {
		data[i] = value
	}
	write2dArray(dset, &data, evtCounter, nSamples)
}

func writeBaselines(dset *hdf5.Dataset, baselines map[uint16]uint16,
	order []SensorMappingHDF5, evtCounter int, nSensors int) {
	data := make([]int16, nSensors)
	for i, sensor := range order {
		// Write only if the corresponding sensor has data
		// if not, the baseline will be zero
		if _, ok := baselines[uint16(sensor.channel)]; !ok {
			continue
		}
		data[i] = int16(baselines[uint16(sensor.channel)])
	}
	write2dArray(dset, &data, evtCounter, nSensors)
}

func (w *Writer) Close() error {
	fmt.Println("Closing file hdf writer ", w.Filename)
	var errs []error

	if err := w.EventTable.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing event table: %w", err))
	}
	if err := w.RunInfoTable.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing run info table: %w", err))
	}
	if w.PmtWaveforms != nil {
		if err := w.PmtWaveforms.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing PMT waveforms: %w", err))
		}
	}
	if w.Baselines != nil {
		if err := w.Baselines.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing PMT baselines: %w", err))
		}
	}
	if w.SipmWaveforms != nil {
		if err := w.SipmWaveforms.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing SiPM waveforms: %w", err))
		}
	}
	if w.ExtTrgWaveform != nil {
		if err := w.ExtTrgWaveform.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing external trigger waveform: %w", err))
		}
	}
	if w.PmtSumWaveform != nil {
		if err := w.PmtSumWaveform.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing PMT sum waveform: %w", err))
		}
	}
	if w.PmtSumBaseline != nil {
		if err := w.PmtSumBaseline.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing PMT sum baselines: %w", err))
		}
	}
	if w.BlrWaveforms != nil {
		if err := w.BlrWaveforms.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing BLR waveforms: %w", err))
		}
	}
	if w.BlrBaselines != nil {
		if err := w.BlrBaselines.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing BLR baselines: %w", err))
		}
	}
	if err := w.PmtMappingTable.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing PMT mapping table: %w", err))
	}
	if err := w.SipmMappingTable.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing SiPM mapping table: %w", err))
	}
	if w.TriggerParamsTable != nil {
		if err := w.TriggerParamsTable.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing trigger params table: %w", err))
		}
	}
	if err := w.TriggerLostTable.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing trigger lost table: %w", err))
	}
	if err := w.TriggerTypeTable.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing trigger type table: %w", err))
	}
	if err := w.TriggerChannels.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing trigger channels: %w", err))
	}
	if err := w.RunGroup.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing run group: %w", err))
	}
	if err := w.RDGroup.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing RD group: %w", err))
	}
	if err := w.SensorsGroup.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing sensors group: %w", err))
	}
	if err := w.TriggerGroup.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing trigger group: %w", err))
	}
	if err := w.File.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing file: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (w *Writer) writeTriggerConfiguration(params TriggerData) {
	t := reflect.TypeOf(params)
	n := t.NumField()
	entries := make([]TriggerParamsHDF5, n)

	fieldsToWrite := 0
	for i := 0; i < n; i++ {
		f := t.Field(i)
		paramName := f.Tag.Get("hdf5")
		// Write only single-value fields, not the slices with trigger channels
		switch {
		case f.Type.Kind() == reflect.Uint16:
			value := reflect.ValueOf(params).Field(i).Interface().(uint16)
			entry := TriggerParamsHDF5{
				paramStr: convertToHdf5String(paramName),
				value:    int32(value),
			}
			entries[fieldsToWrite] = entry
			fieldsToWrite++
		case f.Type.Kind() == reflect.Uint32:
			value := reflect.ValueOf(params).Field(i).Interface().(uint32)
			entry := TriggerParamsHDF5{
				paramStr: convertToHdf5String(paramName),
				value:    int32(value),
			}
			entries[fieldsToWrite] = entry
			fieldsToWrite++
		}
	}
	toWrite := entries[:fieldsToWrite]
	writeArrayToTable(w.TriggerParamsTable, &toWrite, w.EvtCounter)
}

func ProcessDecodedEvent(event EventType, configuration Configuration,
	writer *Writer, writer2 *Writer) {
	if configuration.WriteData && !event.Error {
		if configuration.SplitTrg {
			switch int(event.TriggerType) {
			case configuration.TrgCode1:
				writer.WriteEvent(&event)
			case configuration.TrgCode2:
				writer2.WriteEvent(&event)
			}
		} else {
			writer.WriteEvent(&event)
		}
	}
}
