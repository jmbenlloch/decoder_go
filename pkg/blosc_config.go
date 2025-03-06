package decoder

import (
	"encoding/json"
	"fmt"

	"github.com/next-exp/hdf5-go"
)

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

type BloscShuffle struct {
	Name string
	Code hdf5.BloscShuffle
}

const (
	BLOSC_NOSHUFFLE  = hdf5.BLOSC_NOSHUFFLE
	BLOSC_SHUFFLE    = hdf5.BLOSC_SHUFFLE
	BLOSC_BITSHUFFLE = hdf5.BLOSC_BITSHUFFLE
)

var bloscShuffleStrings = []string{
	"no-shuffle",
	"byte-shuffle",
	"bit-shuffle",
}

func (b BloscShuffle) String() string {
	if b.Code < BLOSC_NOSHUFFLE || b.Code > BLOSC_BITSHUFFLE {
		return "UNKNOWN"
	}
	return bloscShuffleStrings[b.Code]
}

func (b BloscShuffle) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

func (b *BloscShuffle) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for i, v := range bloscShuffleStrings {
		if v == s {
			*b = BloscShuffle{Name: s, Code: hdf5.BloscShuffle(i)}
			return nil
		}
	}
	return fmt.Errorf("invalid BloscShuffle: %s", s)
}
