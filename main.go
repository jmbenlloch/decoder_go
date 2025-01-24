package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

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

	// Create writers
	var writer, writer2 *Writer
	writer = NewWriter(configuration.FileOut)
	if configuration.SplitTrg {
		writer2 = NewWriter(configuration.FileOut2)
		defer writer2.Close()
	}
	defer writer.Close()
	//fileInfo, err := file.Stat()
	//if err != nil {
	//	fmt.Println("Error getting file info:", err)
	//	return
	//}
	//fileSize := fileInfo.Size()
	//fmt.Println("File size in bytes:", fileSize)
	//dataRead := make([]byte, fileSize)
	//nRead, err := file.Read(dataRead)
	//if err != nil {
	//	fmt.Println("Error reading file:", err)
	//	return
	//}
	//fmt.Println("Bytes read:", nRead)
	//data = &dataRead

	evtCount := countEvents(file)
	evtsToRead := numberOfEventsToProcess(evtCount, configuration.Skip, configuration.MaxEvents)
	fmt.Println("Number of events:", evtCount)

	fileReader := NewFileReader(file)

	start := time.Now()
	if configuration.Parallel {
		jobs := make(chan WorkerData, configuration.NumWorkers)
		results := make(chan EventType, configuration.NumWorkers)

		for w := 1; w <= configuration.NumWorkers; w++ {
			go worker(w, jobs, results)
		}
		go sendEventsToWorkers(fileReader, jobs)

		if evtsToRead > 0 {
			processWorkerResults(results, writer, writer2, evtsToRead)
		}
		close(results)
	} else {
		for {
			header, eventData, err := fileReader.getNextEvent()
			fmt.Printf("Reading event %d\n", EventIdGetNbInRun(header.EventId))
			if err != nil {
				fmt.Println("Error reading event:", err)
				break
			}
			if err == io.EOF {
				break
			}
			event := readGDC(eventData, header)
			processDecodedEvent(event, configuration, writer, writer2)
		}
	}
	duration := time.Since(start)
	fmt.Println("Total time : ", duration.Milliseconds())
}

func numberOfEventsToProcess(fileEvtCount int, skipEvts int, maxEvtCount int) int {
	evtsToRead := maxEvtCount - skipEvts
	if evtsToRead > fileEvtCount {
		evtsToRead = fileEvtCount
	}
	return evtsToRead
}
