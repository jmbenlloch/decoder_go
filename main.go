package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"
)

func main() {
	println("Hello, World!")

	file, err := os.Open("run_14711.ldc1next.next-100.045.rd")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	var evtHeader EventHeaderStruct
	headerSize := unsafe.Sizeof(evtHeader)
	fmt.Println("Header size:", headerSize)

	var eqHeader EquipmentHeaderStruct
	eqHeaderSize := unsafe.Sizeof(eqHeader)
	fmt.Println("Equipment Header size:", eqHeaderSize)

	evtCount := 1
	for {
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
		binary.Read(headerReader, binary.LittleEndian, &evtHeader)
		fmt.Printf("Event count: %d, evt id: %d. GDC %d, LDC %d\n", evtCount, evtHeader.EventId, evtHeader.EventGdcId, evtHeader.EventLdcId)
		fmt.Println("Header:", evtHeader)
		fmt.Println("Superevent:", evtHeader.EventTypeAttribute[0]&SUPER_EVENT)

		payloadSize := uint32(evtHeader.EventSize) - uint32(headerSize)
		eventData := make([]byte, payloadSize)
		file.Read(eventData)

		// Read LDCs
		ldcCount := 0
		position := 0
		for {
			var ldcHeader EventHeaderStruct
			ldcHeaderBinary := eventData[position : position+int(headerSize)]
			ldcHeaderReader := bytes.NewReader(ldcHeaderBinary)
			binary.Read(ldcHeaderReader, binary.LittleEndian, &ldcHeader)
			fmt.Printf("\tLDC count: %d, evt id: %d. GDC %d, LDC %d\n", ldcCount, ldcHeader.EventId, ldcHeader.EventGdcId, ldcHeader.EventLdcId)
			fmt.Println("\tHeader:", ldcHeader)
			fmt.Println("\tSuperevent:", ldcHeader.EventTypeAttribute[0]&SUPER_EVENT)

			// Read equipment header
			equipmentCount := 0
			eqPosition := position + int(ldcHeader.EventHeadSize)
			for {
				eqHeaderBinary := eventData[eqPosition : eqPosition+int(eqHeaderSize)]
				eqHeaderReader := bytes.NewReader(eqHeaderBinary)
				binary.Read(eqHeaderReader, binary.LittleEndian, &eqHeader)
				fmt.Printf("\t\tEquipment count: %d, eq id: %d. eq type %d\n", equipmentCount, eqHeader.EquipmentId, eqHeader.EquipmentType)
				fmt.Println("\t\tHeader:", eqHeader)
				fmt.Printf("\t\teqPosition: %d, offset: %d, ldc size: %d\n", eqPosition, eqPosition-position, ldcHeader.EventSize)

				start := eqPosition + int(eqHeaderSize)
				end := eqPosition + int(eqHeader.EquipmentSize)
				payload := flipWords(eventData[start:end])

				fmt.Printf("\t\t payload: ")
				for i := 0; i < 20; i++ {
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

				ReadCommonHeader(payload)

				// Next equipment
				eqPosition += int(eqHeader.EquipmentSize)
				if (eqPosition - position) >= int(ldcHeader.EventSize) {
					break
				}
			}

			// Next LDC
			position += int(ldcHeader.EventSize)
			fmt.Printf("\tPosition: %d, Length of eventData: %d\n", position, len(eventData))
			if position >= len(eventData) {
				break
			}
		}
	}
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
