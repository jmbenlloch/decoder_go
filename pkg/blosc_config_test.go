package decoder

import (
	"encoding/json"
	"testing"
)

func TestBloscAlgorithmJSONRoundTrip(t *testing.T) {
	for _, name := range []string{"blosclz", "lz4", "lz4hc", "snappy", "zlib", "zstd"} {
		var alg BloscAlgorithm
		if err := json.Unmarshal([]byte(`"`+name+`"`), &alg); err != nil {
			t.Fatalf("unmarshal %q: %v", name, err)
		}
		if alg.String() != name {
			t.Errorf("String() = %q, want %q", alg.String(), name)
		}
		out, err := json.Marshal(alg)
		if err != nil {
			t.Fatalf("marshal %q: %v", name, err)
		}
		if string(out) != `"`+name+`"` {
			t.Errorf("marshal = %s, want %q", out, name)
		}
	}
}

func TestBloscAlgorithmInvalid(t *testing.T) {
	var alg BloscAlgorithm
	if err := json.Unmarshal([]byte(`"gzip"`), &alg); err == nil {
		t.Error("unmarshal invalid algorithm: expected error, got nil")
	}
}

func TestBloscShuffleJSONRoundTrip(t *testing.T) {
	for _, name := range []string{"no-shuffle", "byte-shuffle", "bit-shuffle"} {
		var sh BloscShuffle
		if err := json.Unmarshal([]byte(`"`+name+`"`), &sh); err != nil {
			t.Fatalf("unmarshal %q: %v", name, err)
		}
		if sh.String() != name {
			t.Errorf("String() = %q, want %q", sh.String(), name)
		}
		out, err := json.Marshal(sh)
		if err != nil {
			t.Fatalf("marshal %q: %v", name, err)
		}
		if string(out) != `"`+name+`"` {
			t.Errorf("marshal = %s, want %q", out, name)
		}
	}
}

func TestBloscShuffleInvalid(t *testing.T) {
	var sh BloscShuffle
	if err := json.Unmarshal([]byte(`"random-shuffle"`), &sh); err == nil {
		t.Error("unmarshal invalid shuffle: expected error, got nil")
	}
}
