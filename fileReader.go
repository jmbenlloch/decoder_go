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
		if VerbosityLevel > 0 {
			InfoLog.Info("Max events reached", "module", "fileReader")
		}
		return header, nil, io.EOF
	}
	if f.EvtCount < configuration.Skip {
		if VerbosityLevel > 0 {
			message := fmt.Sprintf("Skipping event %d with ID %d", f.EvtCount, EventIdGetNbInRun(header.EventId))
			InfoLog.Info(message, "module", "fileReader")
		}
		return f.getNextEvent()
	}
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Reading event %d with ID %d", f.EvtCount, EventIdGetNbInRun(header.EventId))
		InfoLog.Info(message, "module", "fileReader")
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
			if err != io.EOF {
				errMessage := fmt.Errorf("error reading header counting events: %w", err)
				ErrorLog.Error(errMessage.Error())
			}
			break
		}
		if nRead == 0 {
			if VerbosityLevel > 1 {
				InfoLog.Debug("End of file")
			}
			break
		}

		headerReader := bytes.NewReader(headerBinary)
		binary.Read(headerReader, binary.LittleEndian, &header)
		if VerbosityLevel > 1 {
			message := fmt.Sprintf("Evt id: %d. GDC %d", EventIdGetNbInRun(header.EventId), header.EventGdcId)
			InfoLog.Debug(message, "module", "evtCounter")
		}
		payloadSize := uint32(header.EventSize) - uint32(headerSize)
		file.Seek(int64(payloadSize), 1)

		if !validEvent(header) {
			if VerbosityLevel > 1 {
				InfoLog.Info("Skipping invalid event: %d\n", EventIdGetNbInRun(header.EventId))
			}
			continue
		}
		evtCount++
	}
	// Go back to the beginning of the file
	file.Seek(0, 0)
	return evtCount
}
