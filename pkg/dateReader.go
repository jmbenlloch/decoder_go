package decoder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"
)

func ValidEvent(header EventHeaderStruct) bool {
	return header.EventType == PHYSICS_EVENT || header.EventType == CALIBRATION_EVENT
}

func ReadEventFromFile(file *os.File) (EventHeaderStruct, []byte, error) {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	headerBinary := make([]byte, headerSize)
	nRead, err := file.Read(headerBinary)
	if err != nil {
		return header, nil, err
	}

	if nRead == 0 {
		return header, nil, err
	}

	headerReader := bytes.NewReader(headerBinary)
	binary.Read(headerReader, binary.LittleEndian, &header)

	payloadSize := uint32(header.EventSize) - uint32(headerSize)
	eventData := make([]byte, payloadSize)
	file.Read(eventData)
	return header, eventData, nil

}

func ReadEvent(data []byte) (EventHeaderStruct, []byte, error) {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	if len(data) < int(headerSize) {
		return header, nil, fmt.Errorf("data is too short")
	}
	headerReader := bytes.NewReader(data[:headerSize])
	binary.Read(headerReader, binary.LittleEndian, &header)

	payloadSize := uint32(header.EventSize) - uint32(headerSize)
	eventData := data[headerSize : uint32(headerSize)+payloadSize]
	return header, eventData, nil
}

func ReadGDC(eventData []byte, header EventHeaderStruct) (EventType, error) {
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

	// Read LDCs
	position := 0
	for {
		nRead := readLDC(eventData, position, &event, sipmPayloads)
		// Next LDC
		position += nRead
		if position >= len(eventData) {
			break
		}
	}

	processPmtIds(&event, configuration)
	return event, nil
}

func readLDC(eventData []byte, position int, event *EventType, sipmPayloads map[uint16][]uint16) int {
	var header EventHeaderStruct
	headerSize := unsafe.Sizeof(header)
	ldcHeaderBinary := eventData[position : position+int(headerSize)]
	ldcHeaderReader := bytes.NewReader(ldcHeaderBinary)
	binary.Read(ldcHeaderReader, binary.LittleEndian, &header)

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

	eqHeaderBinary := eventData[position : position+int(eqHeaderSize)]
	eqHeaderReader := bytes.NewReader(eqHeaderBinary)
	binary.Read(eqHeaderReader, binary.LittleEndian, &eqHeader)
	nRead := int(eqHeader.EquipmentSize)

	start := position + int(eqHeaderSize)
	end := position + int(eqHeader.EquipmentSize)
	payload := flipWords(eventData[start:end])

	evtFormat := ReadCommonHeader(payload)
	// Set event timestamp. All subevents should be at the same time
	// so we can use the timestamp from the any of them
	event.Timestamp = evtFormat.Timestamp
	// Set trigger type. All subevents should have the same trigger type
	event.TriggerType = evtFormat.TriggerType

	// Check error bit
	if evtFormat.ErrorBit {
		evtNumber := event.EventID
		errMessage := fmt.Sprintf("event %d ErrorBit is %t, fec: 0x%x", evtNumber, evtFormat.ErrorBit, evtFormat.FecID)
		logger.Error(errMessage)
		event.Error = true
		if configuration.Discard {
			return nRead
		}
	}

	switch evtFormat.FWVersion {
	case 10:
		switch evtFormat.FecType {
		case 0:
			if configuration.Verbosity > 1 {
				message := fmt.Sprintf("PMT FEC %d (0x%02x)", evtFormat.FecID, evtFormat.FecID)
				logger.Info(message, "dateReader")
			}
			if configuration.ReadPMTs {
				ReadPmtFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event)
				event.PmtConfig = PmtConfig{
					Baselines:  evtFormat.Baseline,
					DualMode:   evtFormat.DualModeBit,
					ChannelsHG: evtFormat.ChannelsHG,
				}
			}
		case 1:
			if configuration.Verbosity > 1 {
				message := fmt.Sprintf("SiPM FEC %d (0x%02x)", evtFormat.FecID, evtFormat.FecID)
				logger.Info(message, "dateReader")
			}
			if configuration.ReadSiPMs {
				ReadSipmFEC(payload[evtFormat.HeaderSize:], &evtFormat, &header, event, sipmPayloads)
			}
		case 2:
			if configuration.Verbosity > 1 {
				message := fmt.Sprintf("Triger FEC %d (0x%02x)", evtFormat.FecID, evtFormat.FecID)
				logger.Info(message, "dateReader")
			}
			if configuration.ReadTrigger {
				ReadTriggerFEC(payload[evtFormat.HeaderSize:], event)
			}
		}
	default:
		errMessage := fmt.Errorf("Unknown firwmare version: %d. Event ID %d, FEC 0x%02x",
			evtFormat.FWVersion, EventIdGetNbInRun(header.EventId), evtFormat.FecID)
		logger.Error(errMessage.Error())
	}

	return nRead
}

func flipWords(data []byte) []uint16 {
	positionIn := 0
	positionOut := 0

	dataUint16 := *(*[]uint16)(unsafe.Pointer(&data))
	dataFlipped := make([]uint16, len(data)/2) // TODO round up

	for positionIn*2 < len(data) {
		// Skip sequence counters. Size taken empirically
		if positionIn > 0 && positionIn%3996 == 0 {
			positionIn += 2
		}
		dataFlipped[positionOut] = dataUint16[positionIn+1]
		dataFlipped[positionOut+1] = dataUint16[positionIn]
		positionIn += 2
		positionOut += 2
	}

	return dataFlipped[:positionOut]
}
