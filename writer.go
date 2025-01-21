package main

import (
	"github.com/jmbenlloch/go-hdf5"
)

type Writer struct {
	File1        *hdf5.File
	File2        *hdf5.File
	RunGroup     *hdf5.Group
	RDGroup      *hdf5.Group
	SensorsGroup *hdf5.Group
	TriggerGroup *hdf5.Group
	EventTable   *hdf5.Dataset
}

func NewWriter(config Configuration) *Writer {
	writer := &Writer{}
	writer.File1 = openFile(configuration.FileOut)
	writer.RunGroup, _ = createGroup(writer.File1, "Run")
	writer.RDGroup, _ = createGroup(writer.File1, "RD")
	writer.SensorsGroup, _ = createGroup(writer.File1, "Sensors")
	writer.TriggerGroup, _ = createGroup(writer.File1, "Trigger")
	writer.EventTable = createTable(writer.RDGroup, "events", EventDataHDF5{})
	return writer
}

func (w *Writer) WriteEvent(event *EventType) {
	// Write event data
	datatest := EventDataHDF5{
		timestamp:  event.Timestamp,
		evt_number: int32(event.EventID),
	}
	_ = datatest
	writeEventData(w.EventTable, datatest)
}

func (w *Writer) Close() {
	w.EventTable.Close()
	w.RunGroup.Close()
	w.RDGroup.Close()
	w.SensorsGroup.Close()
	w.TriggerGroup.Close()
	w.File1.Close()
	//w.File2.Close()
}
