package main

import (
	"sort"

	"github.com/jmbenlloch/go-hdf5"
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
	PmtMappingTable    *hdf5.Dataset
	SipmMappingTable   *hdf5.Dataset
	PmtWaveforms       *hdf5.Dataset
	SipmWaveforms      *hdf5.Dataset
	Baselines          *hdf5.Dataset
}

func NewWriter(config Configuration) *Writer {
	writer := &Writer{}
	writer.File1 = openFile(configuration.FileOut)
	writer.RunGroup, _ = createGroup(writer.File1, "Run")
	writer.RDGroup, _ = createGroup(writer.File1, "RD")
	writer.SensorsGroup, _ = createGroup(writer.File1, "Sensors")
	writer.TriggerGroup, _ = createGroup(writer.File1, "Trigger")
	writer.EventTable = createTable(writer.RunGroup, "events", EventDataHDF5{})
	writer.RunInfoTable = createTable(writer.RunGroup, "runInfo", RunInfoHDF5{})
	writer.TriggerParamsTable = createTable(writer.TriggerGroup, "configuration", TriggerParamsHDF5{})
	writer.PmtMappingTable = createTable(writer.SensorsGroup, "DataPmt", SensorMappingHDF5{})
	writer.SipmMappingTable = createTable(writer.SensorsGroup, "DataSipm", SensorMappingHDF5{})
	return writer
}

func sortSensorsBySensorID(sensorsFromElecIDToSensorID map[uint16]uint16) []SensorMappingHDF5 {
	// The array MUST be allocated at creation, if not, HDF5 will panic
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
	//triggerConfig := TriggerParamsHDF5{
	//	param: "test",
	//	value: 1,
	//}

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

		w.SipmWaveforms = createWaveformsArray(w.RDGroup, "sipmrwf", nsipms, sipmSamples)
		w.PmtWaveforms = createWaveformsArray(w.RDGroup, "pmtrwf", npmts, pmtSamples)
		w.Baselines = createBaselinesArray(w.RDGroup, "pmt_baselines", npmts)

		w.FirstEvt = true
	}
	//writeTriggerConfig(w.TriggerParamsTable, triggerConfig)

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
	writeBaselines(w.Baselines, &baselines)

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
	w.TriggerParamsTable.Close()
	w.RunGroup.Close()
	w.RDGroup.Close()
	w.SensorsGroup.Close()
	w.TriggerGroup.Close()
	w.File1.Close()
	//w.File2.Close()
}
