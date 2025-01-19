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
				eqPosition += int(eqHeader.EquipmentSize)
				fmt.Printf("\t\teqPosition: %d, offset: %d, ldc size: %d\n", eqPosition, eqPosition-position, ldcHeader.EventSize)
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
