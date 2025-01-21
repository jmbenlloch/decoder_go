package main

import (
	"fmt"

	"github.com/jmbenlloch/go-hdf5"
)

type EventDataHDF5 struct {
	evt_number int32
	timestamp  uint64
}

type RunInfoHDF5 struct {
	run_number int32
}

type TriggerParamsHDF5 struct {
	param string
	value int32
}

type SensorMappingHDF5 struct {
	channel  int32
	sensorID int32
}

func openFile(fname string) *hdf5.File {
	// create the file
	f, err := hdf5.CreateFile(fname, hdf5.F_ACC_TRUNC)
	if err != nil {
		panic(err)
	}
	//defer f.Close()
	fmt.Printf(":: file [%s] created (id=%d)\n", fname, f.ID())
	return f
}

func createGroup(file *hdf5.File, groupName string) (*hdf5.Group, error) {
	// create the group
	g, err := file.CreateGroup(groupName)
	return g, err
}

func createWaveformsArray(group *hdf5.Group, name string, nSensors int, nSamples int) *hdf5.Dataset {
	dimsArray := []uint{0, 0, 0}
	unlimitedDims := -1 // H5S_UNLIMITED is -1L
	maxDimsArray := []uint{uint(unlimitedDims), uint(nSensors), uint(nSamples)}
	chunks := []uint{1, 50, 32768}
	dataset := createArray(group, name, dimsArray, maxDimsArray, chunks)
	return dataset
}

func createBaselinesArray(group *hdf5.Group, name string, nSensors int) *hdf5.Dataset {
	dimsArray := []uint{0, 0}
	unlimitedDims := -1 // H5S_UNLIMITED is -1L
	maxDimsArray := []uint{uint(unlimitedDims), uint(nSensors)}
	chunks := []uint{1, 32768}
	dataset := createArray(group, name, dimsArray, maxDimsArray, chunks)
	return dataset
}

func createArray(group *hdf5.Group, name string, dims []uint, maxDims []uint, chunks []uint) *hdf5.Dataset {
	//unlimitedDims := -1 // H5S_UNLIMITED is -1L
	//maxDimsArray := []uint{uint(unlimitedDims), uint(nSensors), uint(nSamples)}
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
	plistArray.SetDeflate(4)

	// create the memory data type
	dtypeArray, err := hdf5.NewDatatypeFromValue(int16(7))
	fmt.Println(dtypeArray)
	if err != nil {
		fmt.Println("datatype")
		panic("could not create a dtype")
	}

	// create the dataset
	dsetArray, err := group.CreateDatasetWith(name, hdf5.T_NATIVE_INT16, file_spaceArray, plistArray)
	if err != nil {
		fmt.Println("dataset")
		fmt.Println(err)
		panic(err)
	}

	dimsGot, maxdimsGot, err := dsetArray.Space().SimpleExtentDims()
	fmt.Println("1-Size array: ", dimsGot, maxdimsGot)

	return dsetArray
}

func createTable(group *hdf5.Group, name string, datatype interface{}) *hdf5.Dataset {
	dims := []uint{0}
	unlimitedDims := -1 // H5S_UNLIMITED is -1L
	maxDims := []uint{uint(unlimitedDims)}
	file_space, err := hdf5.CreateSimpleDataspace(dims, maxDims)
	if err != nil {
		fmt.Println("space")
		panic(err)
	}
	fmt.Println(file_space)

	// create property list
	plist, err := hdf5.NewPropList(hdf5.P_DATASET_CREATE)
	if err != nil {
		fmt.Println("plist")
		panic(err)
	}
	chunks := []uint{32768}
	plist.SetChunk(chunks)
	// Set compression level
	plist.SetDeflate(4)

	// create the memory data type
	dtype, err := hdf5.NewDatatypeFromValue(datatype)
	if err != nil {
		fmt.Println("datatype")
		panic("could not create a dtype")
	}

	// create the dataset
	dset, err := group.CreateDatasetWith(name, dtype, file_space, plist)
	if err != nil {
		fmt.Println("dataset")
		fmt.Println(err)
		panic(err)
	}
	fmt.Printf(":: dset (id=%d)\n", dset.ID())
	return dset
}

func writeEntryToTable[T any](dataset *hdf5.Dataset, data T) {
	array := []T{data}
	writeArrayToTable(dataset, &array)
}

func writeArrayToTable[T any](dataset *hdf5.Dataset, data *[]T) {
	length := uint(len(*data))
	dims := []uint{length}
	dataspace, err := hdf5.CreateSimpleDataspace(dims, nil)
	if err != nil {
		fmt.Println("space")
		panic(err)
	}

	// extend
	dimsGot, maxdimsGot, err := dataset.Space().SimpleExtentDims()
	eventsInFile := dimsGot[0]
	fmt.Println("Size: ", dimsGot, maxdimsGot)
	newsize := []uint{eventsInFile + length}
	dataset.Resize(newsize)
	filespace := dataset.Space()
	fmt.Println(filespace)

	start := []uint{eventsInFile}
	count := []uint{length}
	filespace.SelectHyperslab(start, nil, count, nil)

	// write data to the dataset
	fmt.Printf(":: dset.Write...\n")
	//err = dset.Write(&s2)
	err = dataset.WriteSubset(data, dataspace, filespace)
	if err != nil {
		fmt.Println("final write")
		panic(err)
	}
	fmt.Printf(":: dset.Write... [ok]\n")

	dataspace.Close()
	filespace.Close()
}

func writeTriggerConfig(dataset *hdf5.Dataset, event TriggerParamsHDF5) {
	s2 := make([]TriggerParamsHDF5, 0)
	s2 = append(s2, event)
	length := uint(len(s2))

	dims := []uint{length}
	dataspace, err := hdf5.CreateSimpleDataspace(dims, nil)
	if err != nil {
		fmt.Println("space")
		panic(err)
	}

	// extend
	dimsGot, maxdimsGot, err := dataset.Space().SimpleExtentDims()
	eventsInFile := dimsGot[0]
	fmt.Println("Size: ", dimsGot, maxdimsGot)
	newsize := []uint{eventsInFile + length}
	dataset.Resize(newsize)
	filespace := dataset.Space()
	fmt.Println(filespace)

	start := []uint{eventsInFile}
	count := []uint{length}
	filespace.SelectHyperslab(start, nil, count, nil)

	// write data to the dataset
	fmt.Printf(":: dset.Write...\n")
	//err = dset.Write(&s2)
	err = dataset.WriteSubset(&s2, dataspace, filespace)
	if err != nil {
		fmt.Println("final write")
		panic(err)
	}
	fmt.Printf(":: dset.Write... [ok]\n")

	dataspace.Close()
	filespace.Close()
}

func writeWaveforms(dataset *hdf5.Dataset, data *[]int16) {
	// extend
	dimsGot, maxdimsGot, err := dataset.Space().SimpleExtentDims()
	eventsInFile := dimsGot[0]
	nSensors := maxdimsGot[1]
	nSamples := maxdimsGot[2]
	fmt.Println("2-Size array: ", dimsGot, maxdimsGot)
	newsize := []uint{eventsInFile + 1, nSensors, nSamples}
	dataset.Resize(newsize)
	filespace := dataset.Space()
	fmt.Println(filespace)

	dimsGot, maxdimsGot, err = dataset.Space().SimpleExtentDims()
	fmt.Println("3-Size array: ", dimsGot, maxdimsGot)

	start := []uint{eventsInFile, 0, 0}
	count := []uint{1, nSensors, nSamples}
	filespace.SelectHyperslab(start, nil, count, nil)

	dataspace, err := hdf5.CreateSimpleDataspace(count, nil)
	if err != nil {
		fmt.Println("space")
		panic(err)
	}

	// write data to the dataset
	fmt.Printf(":: dset.Write...\n")
	//err = dsetArray.Write(&charges)
	//err = dataset.WriteSubset(data, dataspace, filespace)
	err = dataset.WriteSubset(data, dataspace, filespace)
	if err != nil {
		panic(err)
	}
	fmt.Printf(":: dset.Write... [ok]\n")

	dataspace.Close()
	filespace.Close()
}

func writeBaselines(dataset *hdf5.Dataset, data *[]int16) {
	// extend
	dimsGot, maxdimsGot, err := dataset.Space().SimpleExtentDims()
	eventsInFile := dimsGot[0]
	nSensors := maxdimsGot[1]
	fmt.Println("2-Size array: ", dimsGot, maxdimsGot)
	newsize := []uint{eventsInFile + 1, nSensors}
	dataset.Resize(newsize)
	filespace := dataset.Space()
	fmt.Println(filespace)

	dimsGot, maxdimsGot, err = dataset.Space().SimpleExtentDims()
	fmt.Println("3-Size array: ", dimsGot, maxdimsGot)

	start := []uint{eventsInFile, 0}
	count := []uint{1, nSensors}
	filespace.SelectHyperslab(start, nil, count, nil)

	dataspace, err := hdf5.CreateSimpleDataspace(count, nil)
	if err != nil {
		fmt.Println("space")
		panic(err)
	}

	// write data to the dataset
	fmt.Printf(":: dset.Write...\n")
	//err = dsetArray.Write(&charges)
	//err = dataset.WriteSubset(data, dataspace, filespace)
	err = dataset.WriteSubset(data, dataspace, filespace)
	if err != nil {
		panic(err)
	}
	fmt.Printf(":: dset.Write... [ok]\n")

	dataspace.Close()
	filespace.Close()
}
