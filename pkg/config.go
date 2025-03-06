package decoder

type Configuration struct {
	MaxEvents        int            `json:"max_events"`
	Verbosity        int            `json:"verbosity"`
	ExtTrigger       int            `json:"ext_trigger"`
	PmtSumCh         int            `json:"pmt_sum_ch"`
	FileIn           string         `json:"file_in"`
	FileOut          string         `json:"file_out"`
	FileOut2         string         `json:"file_out2"`
	TrgCode1         int            `json:"trg_code1"`
	TrgCode2         int            `json:"trg_code2"`
	ReadPMTs         bool           `json:"read_pmts"`
	ReadSiPMs        bool           `json:"read_sipms"`
	ReadTrigger      bool           `json:"read_trigger"`
	SplitTrg         bool           `json:"split_trg"`
	NoDB             bool           `json:"no_db"`
	Discard          bool           `json:"discard"`
	Skip             int            `json:"skip"`
	Host             string         `json:"host"`
	User             string         `json:"user"`
	Passwd           string         `json:"pass"`
	DBName           string         `json:"dbname"`
	NumWorkers       int            `json:"num_workers"`
	WriteData        bool           `json:"write_data"`
	Parallel         bool           `json:"parallel"`
	UseBlosc         bool           `json:"use_blosc"`
	CompressionLevel int            `json:"compression_level"`
	BloscAlgorithm   BloscAlgorithm `json:"blosc_algorithm"`
	BloscShuffle     BloscShuffle   `json:"blosc_shuffle"`
}

var configuration Configuration

func GetConfiguration() Configuration {
	return configuration
}

func SetConfiguration(config Configuration) {
	configuration = config
}
