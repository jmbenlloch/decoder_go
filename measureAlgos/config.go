package main

import (
	"encoding/json"
	"fmt"
	"os"

	decoder "github.com/next-exp/decoder_go/pkg"
)

func LoadConfiguration(filename string) (decoder.Configuration, error) {
	var config decoder.Configuration

	// Set default values
	config.MaxEvents = 1000000000
	config.Verbosity = 0
	config.ExtTrigger = 15
	config.TrgCode1 = 1
	config.TrgCode2 = 9
	config.ReadPMTs = true
	config.ReadSiPMs = true
	config.ReadTrigger = true
	config.SplitTrg = false
	config.NoDB = false
	config.Discard = true
	config.Skip = 0
	config.Host = "next.ific.uv.es"
	config.User = "nextreader"
	config.Passwd = "readonly"
	config.DBName = "NEXT100"
	config.NumWorkers = 1
	config.WriteData = true
	config.Parallel = false
	config.UseBlosc = false
	config.CompressionLevel = 4

	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}

func printConfiguration(config decoder.Configuration, logger Logger) {
	logger.Info(fmt.Sprintf("File in: %s", config.FileIn), "config")
	logger.Info(fmt.Sprintf("File out: %s", config.FileOut), "config")
	logger.Info(fmt.Sprintf("File out2: %s", config.FileOut2), "config")
	logger.Info(fmt.Sprintf("No DB: %t", config.NoDB), "config")
	logger.Info(fmt.Sprintf("Host: %s", config.Host), "config")
	logger.Info(fmt.Sprintf("DB name: %s", config.DBName), "config")
	logger.Info(fmt.Sprintf("Read PMTs: %t", config.ReadPMTs), "config")
	logger.Info(fmt.Sprintf("Read SiPMs: %t", config.ReadSiPMs), "config")
	logger.Info(fmt.Sprintf("Read trigger: %t", config.ReadTrigger), "config")
	logger.Info(fmt.Sprintf("Skip: %d", config.Skip), "config")
	logger.Info(fmt.Sprintf("Max events: %d", config.MaxEvents), "config")
	logger.Info(fmt.Sprintf("Verbosity: %d", config.Verbosity), "config")
	logger.Info(fmt.Sprintf("Split trigger: %t", config.SplitTrg), "config")
	logger.Info(fmt.Sprintf("Trigger code 1: %d", config.TrgCode1), "config")
	logger.Info(fmt.Sprintf("Trigger code 2: %d", config.TrgCode2), "config")
	logger.Info(fmt.Sprintf("Discard: %t", config.Discard), "config")
	logger.Info(fmt.Sprintf("Write data: %t", config.WriteData), "config")
	logger.Info(fmt.Sprintf("Number of workers: %d", config.NumWorkers), "config")
	logger.Info(fmt.Sprintf("Parallel: %t", config.Parallel), "config")
}
