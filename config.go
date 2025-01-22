package main

import (
	"encoding/json"
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
