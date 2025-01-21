package main

import (
	"fmt"

	"github.com/jmbenlloch/go-hdf5"
)

type WriterData struct {
	file    *hdf5.File
	data    *hdf5.Dataset
	charges *hdf5.Dataset
}

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

//	func createChargesArray(file *hdf5.File) *hdf5.Dataset {
//		const nCharges = 32
//		dimsArray := []uint{0, 0}
//		unlimitedDims := -1 // H5S_UNLIMITED is -1L
//		maxDimsArray := []uint{uint(unlimitedDims), nCharges}
//		file_spaceArray, err := hdf5.CreateSimpleDataspace(dimsArray, maxDimsArray)
//		if err != nil {
//			panic(err)
//		}
//
//		// create property list
//		plistArray, err := hdf5.NewPropList(hdf5.P_DATASET_CREATE)
//		if err != nil {
//			fmt.Println("plist")
//			panic(err)
//		}
//		chunksArray := []uint{32768, nCharges}
//		plistArray.SetChunk(chunksArray)
//		// Set compression level
//		plistArray.SetDeflate(4)
//
//		// create the memory data type
//		dtypeArray, err := hdf5.NewDatatypeFromValue(int16(7))
//		fmt.Println(dtypeArray)
//		if err != nil {
//			fmt.Println("datatype")
//			panic("could not create a dtype")
//		}
//
//		// create the dataset
//		dsnameArray := "charges"
//		//dsetArray, err := f.CreateDatasetWith(dsnameArray, dtypeArray, file_spaceArray, plistArray)
//		dsetArray, err := file.CreateDatasetWith(dsnameArray, hdf5.T_NATIVE_INT16, file_spaceArray, plistArray)
//		if err != nil {
//			fmt.Println("dataset")
//			fmt.Println(err)
//			panic(err)
//		}
//
//		dimsGot, maxdimsGot, err := dsetArray.Space().SimpleExtentDims()
//		fmt.Println("1-Size array: ", dimsGot, maxdimsGot)
//
//		return dsetArray
//	}

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
}

//func writeCharges(dataset *hdf5.Dataset, events *[]EventData) {
//	//	charges := [2][32]int16{
//	//		{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2},
//	//		{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2},
//	//	}
//	const nCharges = 32
//	length := uint(len(*events))
//	charges := make([][32]uint16, length)
//	//fmt.Println(charges)
//
//	for evt := 0; evt < int(length); evt++ {
//		for sensor := 0; sensor < nCharges; sensor++ {
//			charges[evt][sensor] = (*events)[evt].Charges[sensor]
//		}
//	}
//
//	dims := []uint{length, nCharges}
//	dataspace, err := hdf5.CreateSimpleDataspace(dims, nil)
//	if err != nil {
//		fmt.Println("space")
//		panic(err)
//	}
//
//	// extend
//	dimsGot, maxdimsGot, err := dataset.Space().SimpleExtentDims()
//	eventsInFile := dimsGot[0]
//	fmt.Println("2-Size array: ", dimsGot, maxdimsGot)
//	newsize := []uint{eventsInFile + length, nCharges}
//	dataset.Resize(newsize)
//	filespace := dataset.Space()
//	fmt.Println(filespace)
//
//	dimsGot, maxdimsGot, err = dataset.Space().SimpleExtentDims()
//	fmt.Println("3-Size array: ", dimsGot, maxdimsGot)
//
//	start := []uint{eventsInFile, 0}
//	count := []uint{length, nCharges}
//	filespace.SelectHyperslab(start, nil, count, nil)
//
//	// write data to the dataset
//	fmt.Printf(":: dset.Write...\n")
//	//err = dsetArray.Write(&charges)
//	err = dataset.WriteSubset(&charges, dataspace, filespace)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Printf(":: dset.Write... [ok]\n")
//}
//
