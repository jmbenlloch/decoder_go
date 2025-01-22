package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"unsafe"

	sqlx "github.com/jmoiron/sqlx"
)

const CLOCK_TICK float32 = 0.025

// Map to keep SiPM data until is read. FEC-ID -> SiPM data
var sipmPayloads map[uint16][]uint16 = make(map[uint16][]uint16)

var huffmanCodesPmts *HuffmanNode
var huffmanCodesSipms *HuffmanNode
var sensorsMap *SensorsMap
var dbConn *sqlx.DB
var configuration Configuration

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

	writer := NewWriter(configuration)
	defer writer.Close()

	evtCount := -1
	for {
		header, eventData, err := readEvent(file)
		if err != nil {
			break
		}
		evtCount++
		if evtCount >= configuration.MaxEvents {
			break
		}
		if evtCount < configuration.Skip {
			continue
		}
		event := readGDC(eventData, header)
		writer.WriteEvent(&event)
	}
}
func readEvent(file *os.File) (EventHeaderStruct, []byte, error) {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	fmt.Println("GDC Header size:", headerSize)
	headerBinary := make([]byte, headerSize)
	nRead, err := file.Read(headerBinary)
	if err != nil {
		fmt.Println("Error reading header:", err)
		return header, nil, err
	}
	if nRead == 0 {
		fmt.Println("End of file")
		return header, nil, err
	}

	headerReader := bytes.NewReader(headerBinary)
	binary.Read(headerReader, binary.LittleEndian, &header)
	fmt.Printf("Evt id: %d. GDC %d, LDC %d\n", header.EventId, header.EventGdcId, header.EventLdcId)
	fmt.Println("Header:", header)
	fmt.Println("Superevent:", header.EventTypeAttribute[0]&SUPER_EVENT)

	payloadSize := uint32(header.EventSize) - uint32(headerSize)
	eventData := make([]byte, payloadSize)
	file.Read(eventData)
	return header, eventData, nil
}

func readGDC(eventData []byte, header EventHeaderStruct) EventType {
	event := EventType{
		PmtWaveforms:  make(map[uint16][]int16),
		SipmWaveforms: make(map[uint16][]int16),
		Baselines:     make(map[uint16]uint16),
	}
	event.RunNumber = uint32(header.EventRunNb)
	event.EventID = EventIdGetNbInRun(header.EventId)

	// If we want to use the DB and the sensors map is not loaded, load it
	if !configuration.NoDB && sensorsMap == nil {
		event.SensorsMap = getSensorsFromDB(dbConn, int(header.EventRunNb))
	}

	// Read LDCs
	position := 0
	for {
		nRead := readLDC(eventData, position, &event)
		// Next LDC
		position += nRead
		fmt.Printf("\tPosition: %d, Length of eventData: %d\n", position, len(eventData))
		if position >= len(eventData) {
			break
		}
	}
	return event
}

func readLDC(eventData []byte, position int, event *EventType) int {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	fmt.Println("LDC header size:", headerSize)
	ldcHeaderBinary := eventData[position : position+int(headerSize)]
	ldcHeaderReader := bytes.NewReader(ldcHeaderBinary)
	binary.Read(ldcHeaderReader, binary.LittleEndian, &header)
	fmt.Printf("\tEvt id: %d. GDC %d, LDC %d\n", header.EventId, header.EventGdcId, header.EventLdcId)
	fmt.Println("\tHeader:", header)
	fmt.Println("\tSuperevent:", header.EventTypeAttribute[0]&SUPER_EVENT)

	// Read equipment header
	startLDCPayload := position + int(header.EventHeadSize)
	startPosition := 0
	for {
		nRead := readEquipment(eventData[startLDCPayload:], startPosition, header, event)
		// Next equipment
		startPosition += nRead
		if startPosition+int(header.EventHeadSize) >= int(header.EventSize) {
			break
		}
	}

	return int(header.EventSize)
}

func readEquipment(eventData []byte, position int, header EventHeaderStruct, event *EventType) int {
	var eqHeader EquipmentHeaderStruct
	eqHeaderSize := unsafe.Sizeof(eqHeader)
	fmt.Println("Equipment Header size:", eqHeaderSize)

	fmt.Println("\t\tPosition:", position)

	eqHeaderBinary := eventData[position : position+int(eqHeaderSize)]
	eqHeaderReader := bytes.NewReader(eqHeaderBinary)
	binary.Read(eqHeaderReader, binary.LittleEndian, &eqHeader)
	fmt.Printf("\t\tEq id: %d. eq type %d\n", eqHeader.EquipmentId, eqHeader.EquipmentType)
	fmt.Println("\t\tHeader:", eqHeader)
	fmt.Printf("\t\teqPosition: %d, offset: %d, ldc size: %d\n", position)

	start := position + int(eqHeaderSize)
	end := position + int(eqHeader.EquipmentSize)
	payload := flipWords(eventData[start:end])

	fmt.Printf("\t\t payload: ")
	for i := 0; i < 30; i++ {
		fmt.Printf(" %x", payload[i])
	}
	fmt.Printf("\n")

	fmt.Printf("\t\t originl: ")
	for i := 0; i < 20; i++ {
		fmt.Printf(" %x", eventData[start+i])
	}
	fmt.Printf("\n")

	fmt.Printf("\t\t end payload: ")
	for i := len(payload) - 20; i < len(payload); i++ {
		fmt.Printf(" %x", payload[i])
	}
	fmt.Printf("\n")

	fmt.Printf("\t\t end originl: ")
	for i := end - 20; i < end; i++ {
		fmt.Printf(" %x", eventData[i])
	}
	fmt.Printf("\n")

	evtFormat := ReadCommonHeader(payload)
	// Set event timestamp. All subevents should be at the same time
	// so we can use the timestamp from the any of them
	event.Timestamp = evtFormat.Timestamp
	// Set trigger type. All subevents should have the same trigger type
	event.TriggerType = evtFormat.TriggerType

	switch evtFormat.FWVersion {
	case 10:
		fmt.Println("FW Version 10")
		switch evtFormat.FecType {
		case 0:
			fmt.Println("PMT FEC")
			if configuration.ReadPMTs {
				ReadPmtFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event)
			}
		case 1:
			fmt.Println("SiPM FEC")
			if configuration.ReadSiPMs {
				ReadSipmFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event)
			}
		case 2:
			fmt.Println("Trigger FEC")
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
	fmt.Println("Data size:", len(data))
	fmt.Printf("Data: ")
	for i := len(data) - 20; i < len(data); i++ {
		fmt.Printf(" %x", data[i])
	}
	fmt.Printf("\n")

	dataUint16 := *(*[]uint16)(unsafe.Pointer(&data))
	fmt.Println("Data size casted to uint16:", len(dataUint16))
	fmt.Printf("Data casted: ")
	for i := len(data)/2 - 20; i < len(data)/2; i++ {
		fmt.Printf(" %x", dataUint16[i])
	}
	fmt.Printf("\n")

	fmt.Printf("Data start: ")
	for i := 0; i < 20; i++ {
		fmt.Printf(" %x", data[i])
	}
	fmt.Printf("\n")

	fmt.Printf("Data start casted: ")
	for i := 0; i < 20; i++ {
		fmt.Printf(" %x", dataUint16[i])
	}
	fmt.Printf("\n")

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
