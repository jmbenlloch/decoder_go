package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
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

var (
	InfoLog        *slog.Logger
	ErrorLog       *slog.Logger
	VerbosityLevel int
	DiscardErrors  bool
)

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handlerStdOut := NewHandler(os.Stdout, opts)
	handlerStdErr := slog.NewJSONHandler(os.Stderr, opts)
	InfoLog = slog.New(handlerStdOut)
	ErrorLog = slog.New(handlerStdErr)
}

func main() {
	configFilename := flag.String("config", "", "Configuration file path")
	flag.Parse()

	var err error
	configuration, err = LoadConfiguration(*configFilename)
	if err != nil {
		message := fmt.Errorf("Error reading configuration file: %w", err)
		ErrorLog.Error(message.Error())
		return
	}
	VerbosityLevel = configuration.Verbosity
	DiscardErrors = configuration.Discard
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Reading configuration file: %s", *configFilename)
		InfoLog.Info(message, "module", "main")
	}
	if VerbosityLevel > 0 {
		printConfiguration(configuration, InfoLog)
	}

	dbConn, err = ConnectToDatabase(configuration.User, configuration.Passwd, configuration.Host, configuration.DBName)
	if err != nil {
		message := fmt.Errorf("Error connection to database: %w", err)
		ErrorLog.Error(message.Error())
		return
	}

	file, err := os.Open(configuration.FileIn)
	if err != nil {
		message := fmt.Errorf("Error opening file: %w", err)
		ErrorLog.Error(message.Error())
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

	evtCount := countEvents(file)
	evtsToRead := numberOfEventsToProcess(evtCount, configuration.Skip, configuration.MaxEvents)
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Number of events: %d", evtCount)
		InfoLog.Info(message, "module", "main")
	}

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
			// TODO: This should be modified to write in parallel trigger1 and trigger2
			processWorkerResults(results, writer, writer2, evtsToRead)
		}
		close(results)
	} else {
		for {
			header, eventData, err := fileReader.getNextEvent()
			if err != nil {
				if err != io.EOF {
					message := fmt.Errorf("error reading event: %w", err)
					ErrorLog.Error(message.Error())
				}
				break
			}
			processEvent(eventData, header, writer, writer2)
		}
	}
	duration := time.Since(start)
	fmt.Println("Total time : ", duration.Milliseconds())
}

func processEvent(eventData []byte, header EventHeaderStruct, writer *Writer, writer2 *Writer) {
	defer func() {
		if r := recover(); r != nil {
			eventID := EventIdGetNbInRun(header.EventId)
			errMessage := fmt.Errorf("decoder recovered from panic on event %d: %v", eventID, r)
			ErrorLog.Error(errMessage.Error())
			message := fmt.Sprintf("discarding event %d", eventID)
			ErrorLog.Error(message)
		}
	}()

	event, err := readGDC(eventData, header)
	if err != nil {
		message := fmt.Errorf("error reading GDC data: %w", err)
		ErrorLog.Error(message.Error())
		return
	}
	if event.Error && DiscardErrors {
		message := fmt.Sprintf("discarding event %d", event.EventID)
		ErrorLog.Error(message)
		return
	}
	processDecodedEvent(event, configuration, writer, writer2)
}

func numberOfEventsToProcess(fileEvtCount int, skipEvts int, maxEvtCount int) int {
	evtsToRead := maxEvtCount - skipEvts
	if evtsToRead > fileEvtCount {
		evtsToRead = fileEvtCount
	}
	return evtsToRead
}
