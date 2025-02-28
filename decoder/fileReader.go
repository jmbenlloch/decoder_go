package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"

	decoder "github.com/next-exp/decoder_go/pkg"
)

type FileReader struct {
	File     *os.File
	EvtCount int
}

func NewFileReader(file *os.File) *FileReader {
	return &FileReader{File: file, EvtCount: -1}
}

func (f *FileReader) getNextEvent() (decoder.EventHeaderStruct, []byte, error) {
	header, eventData, err := decoder.ReadEventFromFile(f.File)
	if err != nil {
		return header, nil, err
	}
	if !decoder.ValidEvent(header) {
		return f.getNextEvent()
	}
	f.EvtCount++
	if f.EvtCount >= configuration.MaxEvents {
		if VerbosityLevel > 0 {
			logger.Info("Max events reached", "fileReader")
		}
		return header, nil, io.EOF
	}
	if f.EvtCount < configuration.Skip {
		if VerbosityLevel > 0 {
			message := fmt.Sprintf("Skipping event %d with ID %d", f.EvtCount, decoder.EventIdGetNbInRun(header.EventId))
			logger.Info(message, "fileReader")
		}
		return f.getNextEvent()
	}
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Reading event %d with ID %d", f.EvtCount, decoder.EventIdGetNbInRun(header.EventId))
		logger.Info(message, "fileReader")
	}
	return header, eventData, nil
}

func countEvents(file *os.File) (int, int) {
	evtCount := 0
	runNumber := 0
	for {
		var header decoder.EventHeaderStruct
		headerSize := unsafe.Sizeof(header)
		headerBinary := make([]byte, headerSize)
		nRead, err := file.Read(headerBinary)
		if err != nil {
			if err != io.EOF {
				errMessage := fmt.Errorf("error reading header counting events: %w", err)
				logger.Error(errMessage.Error())
			}
			break
		}
		if nRead == 0 {
			if VerbosityLevel > 1 {
				logger.Info("End of file", "evtCounter")
			}
			break
		}
		runNumber = int(header.EventRunNb)

		headerReader := bytes.NewReader(headerBinary)
		binary.Read(headerReader, binary.LittleEndian, &header)
		if VerbosityLevel > 1 {
			message := fmt.Sprintf("Evt id: %d. GDC %d", decoder.EventIdGetNbInRun(header.EventId), header.EventGdcId)
			logger.Info(message, "evtCounter")
		}
		payloadSize := uint32(header.EventSize) - uint32(headerSize)
		file.Seek(int64(payloadSize), 1)

		if !decoder.ValidEvent(header) {
			if VerbosityLevel > 1 {
				message := fmt.Sprintf("Skipping invalid event: %d", decoder.EventIdGetNbInRun(header.EventId))
				logger.Info(message, "evtCounter")
			}
			continue
		}
		evtCount++
	}
	// Go back to the beginning of the file
	file.Seek(0, 0)
	return evtCount, runNumber
}
