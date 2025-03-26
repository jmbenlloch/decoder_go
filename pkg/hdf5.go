package decoder

import (
	"fmt"

	"github.com/next-exp/hdf5-go"
)

type EventDataHDF5 struct {
	evt_number int32
	timestamp  uint64
}

type TriggerLostHDF5 struct {
	triggerLost1 int32
	triggerLost2 int32
}

type TriggerTypeHDF5 struct {
	trigger_type int32
}

type RunInfoHDF5 struct {
	run_number int32
}

type TriggerParamsHDF5 struct {
	paramStr [STRLEN]byte
	value    int32
}

type SensorMappingHDF5 struct {
	channel  int32
	sensorID int32
}

const STRLEN = 20

func convertToHdf5String(s string) [STRLEN]byte {
	var byteArray [STRLEN]byte
	copy(byteArray[:], s)
	return byteArray
}

func openFile(fname string) *hdf5.File {
	f, err := hdf5.CreateFile(fname, hdf5.F_ACC_TRUNC)
	if err != nil {
		panic(err)
	}
	return f
}

func createGroup(file *hdf5.File, groupName string) *hdf5.Group {
	g, err := file.CreateGroup(groupName)
	if err != nil {
		panic(err)
	}
	return g
}

func create3dArray(group *hdf5.Group, name string, nSensors int, nSamples int) *hdf5.Dataset {
	dimsArray := []uint{0, 0, 0}
	unlimitedDims := -1 // H5S_UNLIMITED is -1L
	maxDimsArray := []uint{uint(unlimitedDims), uint(nSensors), uint(nSamples)}

	//chunks := []uint{1, 50, 32768}
	chunks := []uint{1, 50, uint(nSamples)}
	dataset := createArray(group, name, dimsArray, maxDimsArray, chunks)
	return dataset
}

func create2dArray(group *hdf5.Group, name string, nSensors int) *hdf5.Dataset {
	dimsArray := []uint{0, 0}
	unlimitedDims := -1 // H5S_UNLIMITED is -1L
	maxDimsArray := []uint{uint(unlimitedDims), uint(nSensors)}
	chunks := []uint{1, 32768}
	if nSensors < 32768 {
		chunks[1] = uint(nSensors)
	}
	dataset := createArray(group, name, dimsArray, maxDimsArray, chunks)
	return dataset
}

func createArray(group *hdf5.Group, name string, dims []uint, maxDims []uint, chunks []uint) *hdf5.Dataset {
	file_spaceArray, err := hdf5.CreateSimpleDataspace(dims, maxDims)
	if err != nil {
		panic(err)
	}

	// create property list
	plistArray, err := hdf5.NewPropList(hdf5.P_DATASET_CREATE)
	if err != nil {
		fmt.Println("plist")
		panic(err)
	}

	plistArray.SetChunk(chunks)

	// Set compression level
	if configuration.UseBlosc {
		hdf5.ConfigureBloscFilter(plistArray, configuration.BloscAlgorithm.Code, configuration.CompressionLevel, configuration.BloscShuffle.Code)
	} else {
		plistArray.SetDeflate(configuration.CompressionLevel)
	}

	// create the dataset
	dsetArray, err := group.CreateDatasetWith(name, hdf5.T_NATIVE_INT16, file_spaceArray, plistArray)
	if err != nil {
		panic(err)
	}
	return dsetArray
}

func createTable(group *hdf5.Group, name string, datatype interface{}) *hdf5.Dataset {
	dims := []uint{0}
	unlimitedDims := -1 // H5S_UNLIMITED is -1L
	maxDims := []uint{uint(unlimitedDims)}
	file_space, err := hdf5.CreateSimpleDataspace(dims, maxDims)
	if err != nil {
		panic(err)
	}

	// create property list
	plist, err := hdf5.NewPropList(hdf5.P_DATASET_CREATE)
	if err != nil {
		panic(err)
	}

	chunks := []uint{32768}
	plist.SetChunk(chunks)

	// Set compression level
	if configuration.UseBlosc {
		hdf5.ConfigureBloscFilter(plist, configuration.BloscAlgorithm.Code, configuration.CompressionLevel, configuration.BloscShuffle.Code)
	} else {
		plist.SetDeflate(configuration.CompressionLevel)
	}

	// create the memory data type
	dtype, err := hdf5.NewDatatypeFromValue(datatype)
	if err != nil {
		panic(err)
	}

	// create the dataset
	dset, err := group.CreateDatasetWith(name, dtype, file_space, plist)
	if err != nil {
		panic(err)
	}
	return dset
}

func writeEntryToTable[T any](dataset *hdf5.Dataset, data T, evtCounter int) {
	array := []T{data}
	writeArrayToTable(dataset, &array, evtCounter)
}

func writeArrayToTable[T any](dataset *hdf5.Dataset, data *[]T, evtCounter int) {
	length := uint(len(*data))
	dims := []uint{length}
	dataspace, err := hdf5.CreateSimpleDataspace(dims, nil)
	if err != nil {
		fmt.Println("space")
		panic(err)
	}

	// extend
	eventsInFile := uint(evtCounter)
	newsize := []uint{eventsInFile + length}
	dataset.Resize(newsize)
	filespace := dataset.Space()

	start := []uint{eventsInFile}
	count := []uint{length}
	filespace.SelectHyperslab(start, nil, count, nil)

	err = dataset.WriteSubset(data, dataspace, filespace)
	if err != nil {
		panic(err)
	}

	err = dataspace.Close()
	fmt.Println(err)
	err = filespace.Close()
	fmt.Println(err)
}

func write3dArray(dataset *hdf5.Dataset, data *[]int16, evtCounter int, nSensors int, nSamples int) {
	// extend
	newsize := []uint{uint(evtCounter) + 1, uint(nSensors), uint(nSamples)}
	dataset.Resize(newsize)
	filespace := dataset.Space()

	start := []uint{uint(evtCounter), 0, 0}
	count := []uint{1, uint(nSensors), uint(nSamples)}
	filespace.SelectHyperslab(start, nil, count, nil)

	dataspace, err := hdf5.CreateSimpleDataspace(count, nil)
	if err != nil {
		panic(err)
	}

	// write data to the dataset
	err = dataset.WriteSubset(data, dataspace, filespace)
	if err != nil {
		panic(err)
	}

	err = dataspace.Close()
	//fmt.Println("dataspace closed ", err)
	err = filespace.Close()
	//fmt.Println("filespace closed ", err)
}

func write2dArray(dataset *hdf5.Dataset, data *[]int16, evtCounter int, nSensors int) {
	// extend
	newsize := []uint{uint(evtCounter) + 1, uint(nSensors)}
	dataset.Resize(newsize)
	filespace := dataset.Space()

	start := []uint{uint(evtCounter), 0}
	count := []uint{1, uint(nSensors)}
	filespace.SelectHyperslab(start, nil, count, nil)

	dataspace, err := hdf5.CreateSimpleDataspace(count, nil)
	if err != nil {
		panic(err)
	}

	err = dataset.WriteSubset(data, dataspace, filespace)
	if err != nil {
		panic(err)
	}

	dataspace.Close()
	filespace.Close()
}
