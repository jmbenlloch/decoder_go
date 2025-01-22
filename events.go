package main

type EventType struct {
	RunNumber     uint32
	PmtWaveforms  map[uint16][]int16
	SipmWaveforms map[uint16][]int16
	Baselines     map[uint16]uint16
	EventID       uint32
	Timestamp     uint64
	SensorsMap    SensorsMap
	SipmMapping   SensorMapping
	TriggerConfig TriggerData
}

type SensorsMap struct {
	Pmts  SensorMapping
	Sipms SensorMapping
}

type SensorMapping struct {
	ToElecID   map[uint16]uint16
	ToSensorID map[uint16]uint16
}
