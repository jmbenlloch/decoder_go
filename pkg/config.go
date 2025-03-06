package decoder

import (
	"encoding/json"
	"fmt"

	"github.com/next-exp/hdf5-go"
)

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
}

type BloscAlgorithm struct {
	Name string
	Code hdf5.BloscFilter
}

const (
	BLOSC_BLOSCLZ = hdf5.BLOSC_BLOSCLZ
	BLOSC_LZ4     = hdf5.BLOSC_LZ4
	BLOSC_LZ4HC   = hdf5.BLOSC_LZ4HC
	BLOSC_SNAPPY  = hdf5.BLOSC_SNAPPY
	BLOSC_ZLIB    = hdf5.BLOSC_ZLIB
	BLOSC_ZSTD    = hdf5.BLOSC_ZSTD
)

var bloscAlgorithmStrings = []string{
	"blosclz",
	"lz4",
	"lz4hc",
	"snappy",
	"zlib",
	"zstd",
}

func (b BloscAlgorithm) String() string {
	if b.Code < BLOSC_BLOSCLZ || b.Code > BLOSC_ZSTD {
		return "UNKNOWN"
	}
	return bloscAlgorithmStrings[b.Code]
}

func (b BloscAlgorithm) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

func (b *BloscAlgorithm) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for i, v := range bloscAlgorithmStrings {
		if v == s {
			*b = BloscAlgorithm{Name: s, Code: hdf5.BloscFilter(i)}
			return nil
		}
	}
	return fmt.Errorf("invalid BloscAlgorithm: %s", s)
}

var configuration Configuration

func GetConfiguration() Configuration {
	return configuration
}

func SetConfiguration(config Configuration) {
	configuration = config
}
