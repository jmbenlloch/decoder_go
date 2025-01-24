package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"time"
	"unsafe"
)

func validEvent(header EventHeaderStruct) bool {
	return header.EventType == PHYSICS_EVENT || header.EventType == CALIBRATION_EVENT
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
		startTime := time.Now()
		nRead := readLDC(eventData, position, &event, sipmPayloads)
		duration := time.Since(startTime)
		_ = duration
		//fmt.Printf("Time reading LDC: %d\n", duration.Milliseconds())
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
		startTime := time.Now()
		nRead := readEquipment(eventData[startLDCPayload:], startPosition, header, event, sipmPayloads)
		duration := time.Since(startTime)
		_ = duration
		//fmt.Printf("\tTime reading equipment: %d\n", duration.Milliseconds())
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
				//start := time.Now()
				ReadPmtFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event)
				//duration := time.Since(start)
				//fmt.Printf("Time reading PMT FEC %d: %d\n", evtFormat.FecID, duration.Milliseconds())
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
