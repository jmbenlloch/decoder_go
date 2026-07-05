package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigurationDefaults(t *testing.T) {
	path := writeTempConfig(t, `{}`)
	config, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}

	if config.MaxEvents != 1000000000 {
		t.Errorf("MaxEvents = %d, want 1000000000", config.MaxEvents)
	}
	if config.ExtTrigger != 15 {
		t.Errorf("ExtTrigger = %d, want 15", config.ExtTrigger)
	}
	if config.TrgCode1 != 1 || config.TrgCode2 != 9 {
		t.Errorf("TrgCode1/2 = %d/%d, want 1/9", config.TrgCode1, config.TrgCode2)
	}
	if !config.ReadPMTs || !config.ReadSiPMs || !config.ReadFibers || !config.ReadTrigger {
		t.Error("Read* flags should all default to true")
	}
	if !config.Discard {
		t.Error("Discard should default to true")
	}
	if config.NoDB || config.SplitTrg || config.Parallel || config.UseBlosc {
		t.Error("NoDB/SplitTrg/Parallel/UseBlosc should default to false")
	}
	if !config.WriteData {
		t.Error("WriteData should default to true")
	}
	if config.CompressionLevel != 4 {
		t.Errorf("CompressionLevel = %d, want 4", config.CompressionLevel)
	}
	if config.NumWorkers != 1 {
		t.Errorf("NumWorkers = %d, want 1", config.NumWorkers)
	}
}

func TestLoadConfigurationOverrides(t *testing.T) {
	path := writeTempConfig(t, `{
		"file_in": "/daq/input.rd",
		"file_out": "/daq/output.h5",
		"no_db": true,
		"max_events": 2,
		"read_pmts": false,
		"verbosity": 3,
		"blosc_algorithm": "zstd",
		"blosc_shuffle": "bit-shuffle"
	}`)
	config, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}

	if config.FileIn != "/daq/input.rd" || config.FileOut != "/daq/output.h5" {
		t.Errorf("FileIn/FileOut = %q/%q", config.FileIn, config.FileOut)
	}
	if !config.NoDB || config.MaxEvents != 2 || config.ReadPMTs || config.Verbosity != 3 {
		t.Errorf("overrides not applied: NoDB=%t MaxEvents=%d ReadPMTs=%t Verbosity=%d",
			config.NoDB, config.MaxEvents, config.ReadPMTs, config.Verbosity)
	}
	// Untouched fields keep their defaults
	if !config.ReadSiPMs || config.TrgCode2 != 9 {
		t.Errorf("defaults clobbered: ReadSiPMs=%t TrgCode2=%d", config.ReadSiPMs, config.TrgCode2)
	}
	if config.BloscAlgorithm.String() != "zstd" || config.BloscShuffle.String() != "bit-shuffle" {
		t.Errorf("blosc = %s/%s, want zstd/bit-shuffle",
			config.BloscAlgorithm.String(), config.BloscShuffle.String())
	}
}

func TestLoadConfigurationMissingFile(t *testing.T) {
	if _, err := LoadConfiguration("/nonexistent/config.json"); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadConfigurationInvalidJSON(t *testing.T) {
	path := writeTempConfig(t, `{not json`)
	if _, err := LoadConfiguration(path); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestNumberOfEventsToProcess(t *testing.T) {
	cases := []struct {
		name                    string
		fileEvts, skip, maxEvts int
		want                    int
	}{
		{"max below file count", 100, 0, 10, 10},
		{"max above file count", 5, 0, 10, 5},
		{"skip reduces count", 100, 3, 10, 7},
		{"exact", 10, 0, 10, 10},
		// The reader delivers min(maxEvents, fileEvts) - skip events; the
		// old formula returned fileEvts here (97 and 1 are what actually
		// arrive), making processWorkerResults wait forever in parallel mode.
		{"skip with unlimited max", 100, 3, 1000000000, 97},
		{"skip with small file", 2, 1, 1000000000, 1},
		{"skip beyond file", 5, 10, 1000000000, 0},
	}
	for _, c := range cases {
		if got := numberOfEventsToProcess(c.fileEvts, c.skip, c.maxEvts); got != c.want {
			t.Errorf("%s: numberOfEventsToProcess(%d, %d, %d) = %d, want %d",
				c.name, c.fileEvts, c.skip, c.maxEvts, got, c.want)
		}
	}
}
