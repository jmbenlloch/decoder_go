package main

import (
	"fmt"
	"reflect"
	"sort"

	hdf5 "github.com/jmbenlloch/go-hdf5"
	"golang.org/x/exp/maps"
)

type Writer struct {
	File               *hdf5.File
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
	SipmWaveforms      *hdf5.Dataset
	Baselines          *hdf5.Dataset
	BlrBaselines       *hdf5.Dataset
}

const N_TRG_CH = 64

func NewWriter(filename string) *Writer {
	// Set string size for HDF5
	hdf5.SetStringLength(STRLEN)

	// So far we are not using Blosc
	//hdf5.RegisterBlosc()

	writer := &Writer{}
	writer.File = openFile(filename)
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
	datatest := EventDataHDF5{
		timestamp:  event.Timestamp,
		evt_number: int32(event.EventID),
	}

	writeEntryToTable(w.TriggerLostTable, TriggerLostHDF5{
		triggerLost1: int32(event.TriggerConfig.TriggerLost1),
		triggerLost2: int32(event.TriggerConfig.TriggerLost2),
	})

	writeEntryToTable(w.TriggerTypeTable, TriggerTypeHDF5{
		trigger_type: int32(event.TriggerType),
	})

	var pmtSorted, sipmSorted []SensorMappingHDF5
	var nPmts, nBlrs, nSipms int
	var pmtSamples, sipmSamples int

	if configuration.NoDB {
		pmtSorted = sortSensorsByElecID(event.PmtWaveforms)
		sipmSorted = sortSensorsByElecID(event.SipmWaveforms)
		nPmts = len(event.PmtWaveforms)
		nSipms = len(event.SipmWaveforms)
	} else {
		pmtSorted = sortSensorsBySensorID(event.SensorsMap.Pmts.ToSensorID)
		sipmSorted = sortSensorsBySensorID(event.SensorsMap.Sipms.ToSensorID)
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
		writeEntryToTable(w.RunInfoTable, RunInfoHDF5{run_number: int32(event.RunNumber)})
		writeArrayToTable(w.PmtMappingTable, &pmtSorted)
		writeArrayToTable(w.SipmMappingTable, &sipmSorted)

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
		}

		if len(event.BlrWaveforms) > 0 {
			w.BlrWaveforms = create3dArray(w.RDGroup, "pmt_blr", nPmts, pmtSamples)
			w.BlrBaselines = create2dArray(w.RDGroup, "blr_baselines", nPmts)
		}

		w.FirstEvt = true
	}

	writeEntryToTable(w.EventTable, datatest)

	// Write waveforms
	if nPmts > 0 {
		writeWaveforms(w.PmtWaveforms, event.PmtWaveforms, pmtSorted, nPmts, pmtSamples)
		writeBaselines(w.Baselines, event.Baselines, pmtSorted, nPmts)
	}
	if nBlrs > 0 {
		// This uses the same channel order as the PMTs
		// it works well when reading the channel map from DB
		// in no-DB mode, if there is a dual channel of a missing normal channel,
		// it will not be written.
		writeWaveforms(w.BlrWaveforms, event.BlrWaveforms, pmtSorted, nPmts, pmtSamples)
		writeBaselines(w.BlrBaselines, event.BlrBaselines, pmtSorted, nPmts)
	}
	if nSipms > 0 {
		writeWaveforms(w.SipmWaveforms, event.SipmWaveforms, sipmSorted, nSipms, sipmSamples)
	}
	if event.ExtTrgWaveform != nil {
		writeSingleWaveform(w.ExtTrgWaveform, event.ExtTrgWaveform)
	}
	if event.PmtSumWaveform != nil {
		writeSingleWaveform(w.PmtSumWaveform, event.PmtSumWaveform)
	}

	trgChannels := make([]int16, N_TRG_CH)
	for _, sensor := range event.TriggerConfig.TrgChannels {
		if sensor < N_TRG_CH {
			trgChannels[sensor] = 1
		} else {
			fmt.Println("Trigger channel out of range: ", sensor)
		}
	}
	write2dArray(w.TriggerChannels, &trgChannels)

}

func writeWaveforms(dset *hdf5.Dataset, waveforms map[uint16][]int16,
	order []SensorMappingHDF5, nSensors int, nSamples int) {
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
	write3dArray(dset, &data)
}

func writeSingleWaveform(dset *hdf5.Dataset, waveform *[]int16) {
	data := make([]int16, len(*waveform))
	for i, value := range *waveform {
		data[i] = value
	}
	write2dArray(dset, &data)
}

func writeBaselines(dset *hdf5.Dataset, baselines map[uint16]uint16,
	order []SensorMappingHDF5, nSensors int) {
	data := make([]int16, nSensors)
	for i, sensor := range order {
		// Write only if the corresponding sensor has data
		// if not, the baseline will be zero
		if _, ok := baselines[uint16(sensor.channel)]; !ok {
			continue
		}
		data[i] = int16(baselines[uint16(sensor.channel)])
	}
	write2dArray(dset, &data)
}

func (w *Writer) Close() {
	w.EventTable.Close()
	w.RunInfoTable.Close()
	if w.PmtWaveforms != nil {
		w.PmtWaveforms.Close()
	}
	if w.Baselines != nil {
		w.Baselines.Close()
	}
	if w.SipmWaveforms != nil {
		w.SipmWaveforms.Close()
	}
	if w.ExtTrgWaveform != nil {
		w.ExtTrgWaveform.Close()
	}
	if w.PmtSumWaveform != nil {
		w.PmtSumWaveform.Close()
	}
	if w.BlrWaveforms != nil {
		w.BlrWaveforms.Close()
	}
	if w.BlrBaselines != nil {
		w.BlrBaselines.Close()
	}
	w.PmtMappingTable.Close()
	w.SipmMappingTable.Close()
	if w.TriggerParamsTable != nil {
		w.TriggerParamsTable.Close()
	}
	w.TriggerLostTable.Close()
	w.TriggerTypeTable.Close()
	w.TriggerChannels.Close()
	w.RunGroup.Close()
	w.RDGroup.Close()
	w.SensorsGroup.Close()
	w.TriggerGroup.Close()
	w.File.Close()
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
	writeArrayToTable(w.TriggerParamsTable, &toWrite)
}

func processDecodedEvent(event EventType, configuration Configuration,
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
