package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

type Configuration struct {
	MaxEvents   int    `json:"max_events"`
	Verbosity   int    `json:"verbosity"`
	ExtTrigger  int    `json:"ext_trigger"`
	FileIn      string `json:"file_in"`
	FileOut     string `json:"file_out"`
	FileOut2    string `json:"file_out2"`
	TrgCode1    int    `json:"trg_code1"`
	TrgCode2    int    `json:"trg_code2"`
	ReadPMTs    bool   `json:"read_pmts"`
	ReadSiPMs   bool   `json:"read_sipms"`
	ReadTrigger bool   `json:"read_trigger"`
	SplitTrg    bool   `json:"split_trg"`
	NoDB        bool   `json:"no_db"`
	Discard     bool   `json:"discard"`
	Skip        int    `json:"skip"`
	Host        string `json:"host"`
	User        string `json:"user"`
	Passwd      string `json:"pass"`
	DBName      string `json:"dbname"`
	NumWorkers  int    `json:"num_workers"`
	WriteData   bool   `json:"write_data"`
	Parallel    bool   `json:"parallel"`
}

func LoadConfiguration(filename string) (Configuration, error) {
	var config Configuration

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

func printConfiguration(config Configuration, logger *slog.Logger) {
	logger.Info(fmt.Sprintf("File in: %s", config.FileIn), "module", "config")
	logger.Info(fmt.Sprintf("File out: %s", config.FileOut), "module", "config")
	logger.Info(fmt.Sprintf("File out2: %s", config.FileOut2), "module", "config")
	logger.Info(fmt.Sprintf("No DB: %t", config.NoDB), "module", "config")
	logger.Info(fmt.Sprintf("Host: %s", config.Host), "module", "config")
	logger.Info(fmt.Sprintf("DB name: %s", config.DBName), "module", "config")
	logger.Info(fmt.Sprintf("Read PMTs: %t", config.ReadPMTs), "module", "config")
	logger.Info(fmt.Sprintf("Read SiPMs: %t", config.ReadSiPMs), "module", "config")
	logger.Info(fmt.Sprintf("Read trigger: %t", config.ReadTrigger), "module", "config")
	logger.Info(fmt.Sprintf("Skip: %d", config.Skip), "module", "config")
	logger.Info(fmt.Sprintf("Max events: %d", config.MaxEvents), "module", "config")
	logger.Info(fmt.Sprintf("Verbosity: %d", config.Verbosity), "module", "config")
	logger.Info(fmt.Sprintf("Split trigger: %t", config.SplitTrg), "module", "config")
	logger.Info(fmt.Sprintf("Trigger code 1: %d", config.TrgCode1), "module", "config")
	logger.Info(fmt.Sprintf("Trigger code 2: %d", config.TrgCode2), "module", "config")
	logger.Info(fmt.Sprintf("Discard: %t", config.Discard), "module", "config")
	logger.Info(fmt.Sprintf("Write data: %t", config.WriteData), "module", "config")
	logger.Info(fmt.Sprintf("Number of workers: %d", config.NumWorkers), "module", "config")
	logger.Info(fmt.Sprintf("Parallel: %t", config.Parallel), "module", "config")
}
