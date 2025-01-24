package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"
)

type FileReader struct {
	File     *os.File
	EvtCount int
}

func NewFileReader(file *os.File) *FileReader {
	return &FileReader{File: file, EvtCount: -1}
}

func (f *FileReader) getNextEvent() (EventHeaderStruct, []byte, error) {
	header, eventData, err := readEvent(f.File)
	if err != nil {
		return header, nil, err
	}
	if !validEvent(header) {
		return f.getNextEvent()
	}
	f.EvtCount++
	if f.EvtCount >= configuration.MaxEvents {
		fmt.Println("Max events reached")
		return header, nil, io.EOF
	}
	if f.EvtCount < configuration.Skip {
		fmt.Println("Skipping event")
		return f.getNextEvent()
	}
	return header, eventData, nil
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
	return evtCount
}
