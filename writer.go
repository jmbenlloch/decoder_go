package main

import (
	"fmt"
	"reflect"
	"sort"

	hdf5 "github.com/jmbenlloch/go-hdf5"
)

type Writer struct {
	File1              *hdf5.File
	File2              *hdf5.File
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
	SipmWaveforms      *hdf5.Dataset
	Baselines          *hdf5.Dataset
}

const N_TRG_CH = 64

func NewWriter(config Configuration) *Writer {
	// Set string size for HDF5
	hdf5.SetStringLength(STRLEN)

	writer := &Writer{}
	writer.File1 = openFile(configuration.FileOut)
	writer.RunGroup, _ = createGroup(writer.File1, "Run")
	writer.RDGroup, _ = createGroup(writer.File1, "RD")
	writer.SensorsGroup, _ = createGroup(writer.File1, "Sensors")
	writer.TriggerGroup, _ = createGroup(writer.File1, "Trigger")
	writer.EventTable = createTable(writer.RunGroup, "events", EventDataHDF5{})
	writer.RunInfoTable = createTable(writer.RunGroup, "runInfo", RunInfoHDF5{})
	writer.TriggerParamsTable = createTable(writer.TriggerGroup, "configuration", TriggerParamsHDF5{})
	writer.TriggerLostTable = createTable(writer.TriggerGroup, "triggerLost", TriggerLostHDF5{})
	writer.TriggerTypeTable = createTable(writer.TriggerGroup, "trigger", TriggerTypeHDF5{})
	writer.TriggerChannels = create2dArray(writer.TriggerGroup, "events", N_TRG_CH)
	writer.PmtMappingTable = createTable(writer.SensorsGroup, "DataPmt", SensorMappingHDF5{})
	writer.SipmMappingTable = createTable(writer.SensorsGroup, "DataSipm", SensorMappingHDF5{})
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
		trigger_type: event.TriggerType,
	})

	pmtSorted := sortSensorsBySensorID(event.SensorsMap.Pmts.ToSensorID)
	sipmSorted := sortSensorsBySensorID(event.SensorsMap.Sipms.ToSensorID)

	npmts := len(pmtSorted)
	nsipms := len(sipmSorted)
	pmtSamples := len(event.PmtWaveforms[uint16(pmtSorted[0].channel)])
	sipmSamples := len(event.SipmWaveforms[uint16(sipmSorted[0].channel)])

	if !w.FirstEvt {
		writeEntryToTable(w.RunInfoTable, RunInfoHDF5{run_number: int32(event.RunNumber)})
		writeArrayToTable(w.PmtMappingTable, &pmtSorted)
		writeArrayToTable(w.SipmMappingTable, &sipmSorted)

		w.writeTriggerConfiguration(event.TriggerConfig)

		w.SipmWaveforms = createWaveformsArray(w.RDGroup, "sipmrwf", nsipms, sipmSamples)
		w.PmtWaveforms = createWaveformsArray(w.RDGroup, "pmtrwf", npmts, pmtSamples)
		w.Baselines = create2dArray(w.RDGroup, "pmt_baselines", npmts)

		w.FirstEvt = true
	}

	writeEntryToTable(w.EventTable, datatest)

	// Write waveforms
	pmtData := make([]int16, npmts*pmtSamples)
	for i, sensor := range pmtSorted {
		for j, sample := range event.PmtWaveforms[uint16(sensor.channel)] {
			pmtData[i*pmtSamples+j] = int16(sample)
		}
	}
	writeWaveforms(w.PmtWaveforms, &pmtData)

	sipmData := make([]int16, nsipms*sipmSamples)
	for i, sensor := range sipmSorted {
		for j, sample := range event.SipmWaveforms[uint16(sensor.channel)] {
			sipmData[i*sipmSamples+j] = int16(sample)
		}
	}
	writeWaveforms(w.SipmWaveforms, &sipmData)

	baselines := make([]int16, npmts)
	for i, sensor := range pmtSorted {
		baselines[i] = int16(event.Baselines[uint16(sensor.channel)])
	}
	write2dArray(w.Baselines, &baselines)

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
	w.File1.Close()
	//w.File2.Close()
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
