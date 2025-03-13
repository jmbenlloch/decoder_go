package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/ianlancetaylor/cgosymbolizer"
	sqlx "github.com/jmoiron/sqlx"
	decoder "github.com/next-exp/decoder_go/pkg"
	"github.com/next-exp/hdf5-go"
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
	algorithm := flag.String("algorithm", "blosclz", "Blosc algorithm")
	noBlosc := flag.Bool("no-blosc", false, "Do not use blosc")
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

	if *noBlosc {
		configuration.UseBlosc = false
	} else {
		configuration.BloscAlgorithm = parseAlgorithm(*algorithm)
		fmt.Println("Blosc algorithm: ", configuration.BloscAlgorithm)
	}

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
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Number of events: %d", evtCount)
		logger.Info(message, "main")
	}

	decoder.LoadDatabase(dbConn, runNumber)

	fileReader := NewFileReader(file)

	start := time.Now()
	jobs := make(chan WorkerData, 100)
	results := make(chan decoder.EventType, 100)

	for w := 1; w <= configuration.NumWorkers; w++ {
		go worker(w, jobs, results)
	}
	go sendEventsToWorkers(fileReader, jobs)

	decodedEvents := make([]decoder.EventType, 0)
	for event := range results {
		decodedEvents = append(decodedEvents, event)
		if len(decodedEvents) == evtCount {
			break
		}
	}
	fmt.Println("Total events processed: ", len(decodedEvents))

	// Create writers
	for compressionLevel := 0; compressionLevel < 10; compressionLevel++ {
		if configuration.UseBlosc {
			for _, shuffle := range shuffles {
				//for i := 0; i < 3; i++ {
				fmt.Println("Algorithm: ", configuration.BloscAlgorithm.Name, "Compression level: ", compressionLevel, "Shuffle: ", shuffle.Name)
				configuration.BloscShuffle = shuffle
				configuration.CompressionLevel = compressionLevel
				decoder.SetConfiguration(configuration)
				start := time.Now()
				writer := decoder.NewWriter(configuration.FileOut)
				processWorkerResults(decodedEvents, writer)
				writer.Close()
				duration := time.Since(start)
				fileInfo, err := os.Stat(configuration.FileOut)
				if err != nil {
					logger.Error(fmt.Sprintf("Error getting file info: %v", err))
					continue
				}
				fmt.Printf("(%s, comp %d, %s) Time: %d ms, size %d bytes\n", configuration.BloscAlgorithm.Name, compressionLevel, shuffle.Name, duration.Milliseconds(), fileInfo.Size())
				//	}
			}
		} else {
			for i := 0; i < 3; i++ {
				fmt.Println("Algorithm: standard hdf5, Compression level: ", compressionLevel)
				configuration.CompressionLevel = compressionLevel
				decoder.SetConfiguration(configuration)
				start := time.Now()
				writer := decoder.NewWriter(configuration.FileOut)
				processWorkerResults(decodedEvents, writer)
				writer.Close()
				duration := time.Since(start)
				fileInfo, err := os.Stat(configuration.FileOut)
				if err != nil {
					logger.Error(fmt.Sprintf("Error getting file info: %v", err))
					continue
				}
				fmt.Printf("(hdf5, comp %d) Time: %d ms, size %d bytes\n", compressionLevel, duration.Milliseconds(), fileInfo.Size())
			}

		}
	}

	duration := time.Since(start)
	fmt.Printf("Total time: %d ms\n", duration.Milliseconds())
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

func parseAlgorithm(algorithm string) decoder.BloscAlgorithm {
	var b decoder.BloscAlgorithm
	found := false
	for i, v := range bloscAlgorithmStrings {
		if v == algorithm {
			b.Name = algorithm
			b.Code = hdf5.BloscFilter(i)
			found = true
			break
		}
	}
	if !found {
		fmt.Println("Unknown algorithm: ", algorithm)
		os.Exit(1)
	}
	return b
}

var bloscAlgorithmStrings = []string{
	"blosclz",
	"lz4",
	"lz4hc",
	"snappy",
	"zlib",
	"zstd",
}

var shuffles []decoder.BloscShuffle = []decoder.BloscShuffle{
	decoder.BloscShuffle{
		Name: "no-shuffle",
		Code: decoder.BLOSC_NOSHUFFLE,
	},
	decoder.BloscShuffle{
		Name: "byte-shuffle",
		Code: decoder.BLOSC_SHUFFLE,
	},
	decoder.BloscShuffle{
		Name: "bit-shuffle",
		Code: decoder.BLOSC_BITSHUFFLE,
	},
}
