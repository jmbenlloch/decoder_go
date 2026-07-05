package decoder

import (
	"reflect"
	"testing"
)

func TestSortSensorsBySensorID(t *testing.T) {
	mapping := map[uint16]uint16{
		// elecID -> sensorID, deliberately not aligned
		300: 2,
		100: 15,
		200: 7,
	}
	got := sortSensorsBySensorID(mapping)
	want := []SensorMappingHDF5{
		{channel: 300, sensorID: 2},
		{channel: 200, sensorID: 7},
		{channel: 100, sensorID: 15},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortSensorsBySensorID = %v, want %v", got, want)
	}
}

func TestSortSensorsByElecID(t *testing.T) {
	waveforms := map[uint16][]int16{
		1063: {1},
		1000: {2},
		1032: {3},
	}
	got := sortSensorsByElecID(waveforms)
	want := []SensorMappingHDF5{
		{channel: 1000, sensorID: -1},
		{channel: 1032, sensorID: -1},
		{channel: 1063, sensorID: -1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortSensorsByElecID = %v, want %v", got, want)
	}
}

// Channels present in the data but missing from the DB map get sensorID
// 0xFFFF and therefore sort last.
func TestSortSensorsBySensorIDForWaveforms(t *testing.T) {
	dbMap := map[uint16]uint16{
		500: 20,
		502: 10,
	}
	waveforms := map[uint16][]int16{
		500: {1},
		502: {2},
		504: {3}, // not in DB
	}
	got := sortSensorsBySensorIDForWaveforms(dbMap, waveforms)
	want := []SensorMappingHDF5{
		{channel: 502, sensorID: 10},
		{channel: 500, sensorID: 20},
		{channel: 504, sensorID: 0xFFFF},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortSensorsBySensorIDForWaveforms = %v, want %v", got, want)
	}
}

func TestBuildSortedElecIDs(t *testing.T) {
	pmts := map[uint16][]int16{102: nil, 100: nil}
	fibers := map[uint16][]int16{500: nil, 104: nil}
	got := buildSortedElecIDs(pmts, fibers)
	want := []uint16{100, 102, 104, 500}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildSortedElecIDs = %v, want %v", got, want)
	}
}

func TestBuildSortedSensorIDs(t *testing.T) {
	pmts := map[uint16]uint16{100: 3, 102: 1}
	fibers := map[uint16]uint16{500: 40, 502: 20}
	got := buildSortedSensorIDs(pmts, fibers)
	want := []uint16{1, 3, 20, 40}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildSortedSensorIDs = %v, want %v", got, want)
	}
}

func TestConvertToHdf5String(t *testing.T) {
	got := convertToHdf5String("trigger")
	if string(got[:7]) != "trigger" {
		t.Errorf("prefix = %q, want \"trigger\"", got[:7])
	}
	for i := 7; i < STRLEN; i++ {
		if got[i] != 0 {
			t.Errorf("byte %d = %#x, want 0 padding", i, got[i])
		}
	}
	// Longer than STRLEN: truncated, no panic
	long := convertToHdf5String("a_very_long_parameter_name_beyond_strlen")
	if string(long[:]) != "a_very_long_paramete" {
		t.Errorf("truncated = %q", long[:])
	}
}
