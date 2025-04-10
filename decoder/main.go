package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	sqlx "github.com/jmoiron/sqlx"
	decoder "github.com/next-exp/decoder_go/pkg"
)

var dbConn *sqlx.DB
var configuration decoder.Configuration

var (
	logger         Logger
	VerbosityLevel int
	DiscardErrors  bool
)

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handlerStdOut := NewHandler(os.Stdout, opts)
	handlerStdErr := slog.NewJSONHandler(os.Stderr, opts)
	logger = Logger{
		InfoLog:  slog.New(handlerStdOut),
		ErrorLog: slog.New(handlerStdErr),
	}
}

func main() {
	configFilename := flag.String("config", "", "Configuration file path")
	flag.Parse()

	var err error
	configuration, err = LoadConfiguration(*configFilename)
	if err != nil {
		message := fmt.Errorf("Error reading configuration file: %w", err)
		logger.Error(message.Error())
		return
	}
	decoder.SetConfiguration(configuration)
	decoder.SetLogger(logger)

	VerbosityLevel = configuration.Verbosity
	DiscardErrors = configuration.Discard
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Reading configuration file: %s", *configFilename)
		logger.Info(message, "main")
	}
	if VerbosityLevel > 0 {
		printConfiguration(configuration, logger)
	}

	dbConn, err = decoder.ConnectToDatabase(configuration.User, configuration.Passwd, configuration.Host, configuration.DBName)
	if err != nil {
		message := fmt.Errorf("Error connection to database: %w", err)
		logger.Error(message.Error())
		return
	}
	defer dbConn.Close()

	file, err := os.Open(configuration.FileIn)
	if err != nil {
		message := fmt.Errorf("Error opening file: %w", err)
		logger.Error(message.Error())
		return
	}
	defer file.Close()

	evtCount, runNumber := countEvents(file)
	//evtsToRead := numberOfEventsToProcess(evtCount, configuration.Skip, configuration.MaxEvents)
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Number of events: %d", evtCount)
		logger.Info(message, "main")
	}

	decoder.LoadDatabase(dbConn, runNumber)

	fileReader := NewFileReader(file)

	decodedEvents := make([]decoder.EventType, 0)

	for {
		header, eventData, err := fileReader.getNextEvent()
		if err != nil {
			if err != io.EOF {
				message := fmt.Errorf("error reading event: %w", err)
				logger.Error(message.Error())
			}
			break
		}
		decodedEvent := decodeEvent(eventData, header)
		decodedEvents = append(decodedEvents, decodedEvent)
	}
	fmt.Println("Total events processed: ", len(decodedEvents))

	// Write files in infinite loop
	nLoop := 0
	for {
		start := time.Now()
		fmt.Println("Loop number: ", nLoop)

		// Create writers
		var writer, writer2 *decoder.Writer
		//var writer *decoder.Writer
		writer = decoder.NewWriter(configuration.FileOut)
		if configuration.SplitTrg {
			writer2 = decoder.NewWriter(configuration.FileOut2)
		}

		for i, event := range decodedEvents {
			fmt.Println("Writing event: ", i, event.EventID)
			decoder.ProcessDecodedEvent(event, configuration, writer, writer2)
		}

		writer.Close()
		if configuration.SplitTrg {
			writer2.Close()
		}

		//		if nLoop == 35000 {
		//			time.Sleep(100 * time.Minute)
		//		}

		duration := time.Since(start)
		fmt.Printf("Total time: %d ms\n", duration.Milliseconds())
		nLoop++
	}
}

func processEvent(eventData []byte, header decoder.EventHeaderStruct, writer *decoder.Writer, writer2 *decoder.Writer) {
	defer func() {
		if r := recover(); r != nil {
			eventID := decoder.EventIdGetNbInRun(header.EventId)
			errMessage := fmt.Errorf("decoder recovered from panic on event %d: %v", eventID, r)
			logger.Error(errMessage.Error())
			message := fmt.Sprintf("discarding event %d", eventID)
			logger.Error(message)
		}
	}()

	event, err := decoder.ReadGDC(eventData, header)
	if err != nil {
		message := fmt.Errorf("error reading GDC data: %w", err)
		logger.Error(message.Error())
		return
	}
	if event.Error && DiscardErrors {
		message := fmt.Sprintf("discarding event %d", event.EventID)
		logger.Error(message)
		return
	}
	decoder.ProcessDecodedEvent(event, configuration, writer, writer2)
}

func numberOfEventsToProcess(fileEvtCount int, skipEvts int, maxEvtCount int) int {
	evtsToRead := maxEvtCount - skipEvts
	if evtsToRead > fileEvtCount {
		evtsToRead = fileEvtCount
	}
	return evtsToRead
}

func decodeEvent(eventData []byte, header decoder.EventHeaderStruct) decoder.EventType {
	defer func() {
		if r := recover(); r != nil {
			eventID := decoder.EventIdGetNbInRun(header.EventId)
			errMessage := fmt.Errorf("decoder recovered from panic on event %d: %v", eventID, r)
			logger.Error(errMessage.Error())
			message := fmt.Sprintf("discarding event %d", eventID)
			logger.Error(message)
		}
	}()

	event, err := decoder.ReadGDC(eventData, header)
	if err != nil {
		message := fmt.Errorf("error reading GDC data: %w", err)
		logger.Error(message.Error())
		return decoder.EventType{Error: true}
	}
	if event.Error && DiscardErrors {
		message := fmt.Sprintf("discarding event %d", event.EventID)
		logger.Error(message)
		return decoder.EventType{Error: true}
	}
	return event
}
