package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"time"
	"unsafe"

	sqlx "github.com/jmoiron/sqlx"
)

const CLOCK_TICK float32 = 0.025

var huffmanCodesPmts *HuffmanNode
var huffmanCodesSipms *HuffmanNode
var sensorsMap *SensorsMap
var dbConn *sqlx.DB
var configuration Configuration

//var data *[]byte
//var globalPosition int = 0

func main() {
	configFilename := flag.String("config", "", "Configuration file path")
	flag.Parse()

	var err error
	configuration, err = LoadConfiguration(*configFilename)
	if err != nil {
		fmt.Println("Error reading configuration file: ", err)
		return
	}

	dbConn, err = ConnectToDatabase(configuration.User, configuration.Passwd, configuration.Host, configuration.DBName)
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}

	file, err := os.Open(configuration.FileIn)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	var writer, writer2 *Writer
	writer = NewWriter(configuration.FileOut)
	if configuration.SplitTrg {
		writer2 = NewWriter(configuration.FileOut2)
		defer writer2.Close()
	}
	defer writer.Close()

	jobs := make(chan WorkerData, configuration.NumWorkers)
	results := make(chan EventType, 1000)

	for w := 1; w <= configuration.NumWorkers; w++ {
		go worker(w, jobs, results)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error getting file info:", err)
		return
	}
	fileSize := fileInfo.Size()
	fmt.Println("File size in bytes:", fileSize)
	dataRead := make([]byte, fileSize)
	nRead, err := file.Read(dataRead)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	fmt.Println("Bytes read:", nRead)
	//data = &dataRead

	evtCount := countEvents(file)

	start := time.Now()
	go sendEventsToWorkers(file, jobs, configuration)

	var totalTime int64 = 0

	evtsProcessed := 0
	for event := range results {
		fmt.Println("Processed event: ", evtsProcessed, event.EventID, evtCount)
		evtsProcessed++

		start := time.Now()
		//if configuration.SplitTrg {
		//	switch int(event.TriggerType) {
		//	case configuration.TrgCode1:
		//		writer.WriteEvent(&event)
		//	case configuration.TrgCode2:
		//		writer2.WriteEvent(&event)
		//	}
		//} else {
		//	writer.WriteEvent(&event)
		//}

		duration := time.Since(start)
		totalTime += duration.Milliseconds()
	}
	fmt.Println("Total time writing: ", totalTime)
	duration := time.Since(start)
	fmt.Println("Total time : ", duration.Milliseconds())
	close(results)
}

func countEvents(file *os.File) int {
	evtCount := 0
	for {
		var header EventHeaderStruct
		headerSize := unsafe.Sizeof(header)
		headerBinary := make([]byte, headerSize)
		nRead, err := file.Read(headerBinary)
		if err != nil {
			fmt.Println("Error reading header:", err)
			break
		}
		if nRead == 0 {
			fmt.Println("End of file")
			break
		}

		headerReader := bytes.NewReader(headerBinary)
		binary.Read(headerReader, binary.LittleEndian, &header)
		fmt.Printf("Evt id: %d. GDC %d, LDC %d\n", header.EventId, header.EventGdcId, header.EventLdcId)
		//fmt.Println("Header:", header)
		payloadSize := uint32(header.EventSize) - uint32(headerSize)
		//fmt.Println("Payload size:", payloadSize)
		//fmt.Println("event type: ", header.EventType)
		file.Seek(int64(payloadSize), 1)

		if !validEvent(header) {
			continue
		}
		evtCount++
	}
	// Go back to the beginning of the file
	file.Seek(0, 0)
	fmt.Println("Number of events:", evtCount)
	return evtCount
}

func validEvent(header EventHeaderStruct) bool {
	return header.EventType == PHYSICS_EVENT || header.EventType == CALIBRATION_EVENT
}

func sendEventsToWorkers(file *os.File, jobs chan<- WorkerData, configuration Configuration) {
	evtCount := -1
	for {
		header, eventData, err := readEvent(file)
		if err != nil {
			break
		}
		if !validEvent(header) {
			continue
		}
		evtCount++
		if evtCount >= configuration.MaxEvents {
			break
		}
		if evtCount < configuration.Skip {
			continue
		}
		//event := readGDC(eventData, header)
		jobs <- WorkerData{Data: eventData, Header: header}
	}
	close(jobs)
}

func readEvent(file *os.File) (EventHeaderStruct, []byte, error) {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	//fmt.Println("GDC Header size:", headerSize)
	headerBinary := make([]byte, headerSize)
	nRead, err := file.Read(headerBinary)
	if err != nil {
		fmt.Println("Error reading header:", err)
		return header, nil, err
	}
	//	headerBinary = (*data)[globalPosition : globalPosition+int(headerSize)]
	//globalPosition += int(headerSize)
	if nRead == 0 {
		fmt.Println("End of file")
		return header, nil, err
	}

	headerReader := bytes.NewReader(headerBinary)
	binary.Read(headerReader, binary.LittleEndian, &header)
	//fmt.Printf("Evt id: %d. GDC %d, LDC %d\n", header.EventId, header.EventGdcId, header.EventLdcId)
	//fmt.Println("Header:", header)

	payloadSize := uint32(header.EventSize) - uint32(headerSize)
	eventData := make([]byte, payloadSize)
	file.Read(eventData)
	//eventData := (*data)[globalPosition : globalPosition+int(payloadSize)]
	//globalPosition += int(payloadSize)
	return header, eventData, nil
}

func readGDC(eventData []byte, header EventHeaderStruct) EventType {
	// Map to keep SiPM data until is read. FEC-ID -> SiPM data
	var sipmPayloads map[uint16][]uint16 = make(map[uint16][]uint16)

	event := EventType{
		PmtWaveforms:  make(map[uint16][]int16),
		BlrWaveforms:  make(map[uint16][]int16),
		SipmWaveforms: make(map[uint16][]int16),
		Baselines:     make(map[uint16]uint16),
		BlrBaselines:  make(map[uint16]uint16),
	}
	event.RunNumber = uint32(header.EventRunNb)
	event.EventID = EventIdGetNbInRun(header.EventId)

	// If we want to use the DB and the sensors map is not loaded, load it
	if !configuration.NoDB && sensorsMap == nil {
		event.SensorsMap = getSensorsFromDB(dbConn, int(header.EventRunNb))
		sensorsMap = &event.SensorsMap
	} else {
		event.SensorsMap = *sensorsMap
	}

	// Read LDCs
	position := 0
	for {
		nRead := readLDC(eventData, position, &event, sipmPayloads)
		// Next LDC
		position += nRead
		//fmt.Printf("\tPosition: %d, Length of eventData: %d\n", position, len(eventData))
		if position >= len(eventData) {
			break
		}
	}

	processPmtIds(&event, configuration)
	return event
}

func processPmtIds(event *EventType, configuration Configuration) {
	extTriggerCh := configuration.ExtTrigger
	for elecID, waveform := range event.PmtWaveforms {
		// Check external trigger
		if elecID == uint16(extTriggerCh) {
			event.ExtTrgWaveform = &waveform
			delete(event.PmtWaveforms, elecID)
			delete(event.Baselines, elecID)
		}

		// Check dual channels
		// dual channels are used to send the original waveform and the BLR one
		// They appear as two different channels, but the signal comes from the same
		// physical channel
		// 100 - 111 -> 100 - 111 Real
		// 112 - 123 -> 100 - 111 Dual
		// 200 - 211 -> 200 - 211 Real
		// 212 - 223 -> 200 - 211 Dual
		// 300 - 311 -> 300 - 311 Real
		// 312 - 323 -> 300 - 311 Dual
		// 400 - 411 -> 400 - 411 Real
		// 412 - 423 -> 400 - 411 Dual
		// 500 - 511 -> 500 - 511 Real
		// 512 - 523 -> 500 - 511 Dual
		// 600 - 611 -> 600 - 611 Real
		// 612 - 623 -> 600 - 611 Dual
		// 700 - 711 -> 700 - 711 Real
		// 712 - 723 -> 700 - 711 Dual

		// Dual channel
		if elecID%100 >= 12 {
			newid := elecID - 12
			event.BlrWaveforms[newid] = waveform
			event.BlrBaselines[newid] = event.Baselines[elecID]
			delete(event.PmtWaveforms, elecID)
			delete(event.Baselines, elecID)
		}
	}
}

func readLDC(eventData []byte, position int, event *EventType, sipmPayloads map[uint16][]uint16) int {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	//fmt.Println("LDC header size:", headerSize)
	ldcHeaderBinary := eventData[position : position+int(headerSize)]
	ldcHeaderReader := bytes.NewReader(ldcHeaderBinary)
	binary.Read(ldcHeaderReader, binary.LittleEndian, &header)
	//fmt.Printf("\tEvt id: %d. GDC %d, LDC %d\n", header.EventId, header.EventGdcId, header.EventLdcId)
	//fmt.Println("\tHeader:", header)
	//fmt.Println("\tSuperevent:", header.EventTypeAttribute[0]&SUPER_EVENT)

	// Read equipment header
	startLDCPayload := position + int(header.EventHeadSize)
	startPosition := 0
	for {
		nRead := readEquipment(eventData[startLDCPayload:], startPosition, header, event, sipmPayloads)
		// Next equipment
		startPosition += nRead
		if startPosition+int(header.EventHeadSize) >= int(header.EventSize) {
			break
		}
	}

	return int(header.EventSize)
}

func readEquipment(eventData []byte, position int, header EventHeaderStruct, event *EventType,
	sipmPayloads map[uint16][]uint16) int {
	var eqHeader EquipmentHeaderStruct
	eqHeaderSize := unsafe.Sizeof(eqHeader)
	//fmt.Println("Equipment Header size:", eqHeaderSize)

	//fmt.Println("\t\tPosition:", position)

	eqHeaderBinary := eventData[position : position+int(eqHeaderSize)]
	eqHeaderReader := bytes.NewReader(eqHeaderBinary)
	binary.Read(eqHeaderReader, binary.LittleEndian, &eqHeader)
	//fmt.Printf("\t\tEq id: %d. eq type %d\n", eqHeader.EquipmentId, eqHeader.EquipmentType)
	//fmt.Println("\t\tHeader:", eqHeader)
	//fmt.Printf("\t\teqPosition: %d, offset: %d, ldc size: %d\n", position)

	start := position + int(eqHeaderSize)
	end := position + int(eqHeader.EquipmentSize)
	payload := flipWords(eventData[start:end])

	//fmt.Printf("\t\t payload: ")
	//for i := 0; i < 30; i++ {
	//	fmt.Printf(" %x", payload[i])
	//}
	//fmt.Printf("\n")

	//fmt.Printf("\t\t originl: ")
	//for i := 0; i < 20; i++ {
	//	fmt.Printf(" %x", eventData[start+i])
	//}
	//fmt.Printf("\n")

	//fmt.Printf("\t\t end payload: ")
	//for i := len(payload) - 20; i < len(payload); i++ {
	//	fmt.Printf(" %x", payload[i])
	//}
	//fmt.Printf("\n")

	//fmt.Printf("\t\t end originl: ")
	//for i := end - 20; i < end; i++ {
	//	fmt.Printf(" %x", eventData[i])
	//}
	//fmt.Printf("\n")

	evtFormat := ReadCommonHeader(payload)
	// Set event timestamp. All subevents should be at the same time
	// so we can use the timestamp from the any of them
	event.Timestamp = evtFormat.Timestamp
	// Set trigger type. All subevents should have the same trigger type
	event.TriggerType = evtFormat.TriggerType

	switch evtFormat.FWVersion {
	case 10:
		//fmt.Println("FW Version 10")
		switch evtFormat.FecType {
		case 0:
			//fmt.Println("PMT FEC")
			if configuration.ReadPMTs {
				ReadPmtFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event)
			}
		case 1:
			//fmt.Println("SiPM FEC")
			if configuration.ReadSiPMs {
				ReadSipmFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event, sipmPayloads)
			}
		case 2:
			//fmt.Println("Trigger FEC")
			if configuration.ReadTrigger {
				ReadTriggerFEC(payload[evtFormat.HeaderSize:], event)
			}
		}
	}

	nRead := int(eqHeader.EquipmentSize)
	return nRead
}

func flipWords(data []byte) []uint16 {
	positionIn := 0
	positionOut := 0
	//fmt.Println("Data size:", len(data))
	//fmt.Printf("Data: ")
	//for i := len(data) - 20; i < len(data); i++ {
	//	fmt.Printf(" %x", data[i])
	//}
	//fmt.Printf("\n")

	dataUint16 := *(*[]uint16)(unsafe.Pointer(&data))
	//	fmt.Println("Data size casted to uint16:", len(dataUint16))
	//	fmt.Printf("Data casted: ")
	//for i := len(data)/2 - 20; i < len(data)/2; i++ {
	//	fmt.Printf(" %x", dataUint16[i])
	//}
	//fmt.Printf("\n")

	//fmt.Printf("Data start: ")
	//for i := 0; i < 20; i++ {
	//	fmt.Printf(" %x", data[i])
	//}
	//fmt.Printf("\n")

	//fmt.Printf("Data start casted: ")
	//for i := 0; i < 20; i++ {
	//	fmt.Printf(" %x", dataUint16[i])
	//}
	//fmt.Printf("\n")

	dataFlipped := make([]uint16, len(data)/2) // TODO round up

	for positionIn*2 < len(data) {
		//fmt.Println(positionIn*2, len(data)/2)
		// Skip sequence counters. Size taken empirically
		if positionIn > 0 && positionIn%3996 == 0 {
			//fmt.Printf("Skipping positionIn: %d. Values %x %x\n", positionIn, dataUint16[positionIn], dataUint16[positionIn+1])
			positionIn += 2
		}
		dataFlipped[positionOut] = dataUint16[positionIn+1]
		dataFlipped[positionOut+1] = dataUint16[positionIn]
		//fmt.Printf("PositionIn: %d, PositionOut: %d, Data: out (%x %x), in (%x %x)\n", positionIn, positionOut, dataFlipped[positionOut], dataFlipped[positionOut+1], dataUint16[positionIn], dataUint16[positionIn+1])
		positionIn += 2
		positionOut += 2
	}
	//fmt.Println("positionOut ", positionOut)
	//fmt.Println("positionIn ", positionIn)

	return dataFlipped[:positionOut]
}
