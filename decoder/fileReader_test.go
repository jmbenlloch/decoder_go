package main

import (
	"os"
	"testing"

	decoder "github.com/next-exp/decoder_go/pkg"
)

const demoFixture = "../pkg/testdata/run_15022_2evt.rd"

// setMainConfig swaps the main-package configuration used by FileReader and
// restores it afterwards.
func setMainConfig(t *testing.T, config decoder.Configuration) {
	t.Helper()
	old := configuration
	configuration = config
	t.Cleanup(func() { configuration = old })
}

func TestCountEvents(t *testing.T) {
	file, err := os.Open(demoFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	evtCount, runNumber := countEvents(file)

	if evtCount != 2 {
		t.Errorf("event count = %d, want 2", evtCount)
	}
	if runNumber != 15022 {
		t.Errorf("run number = %d, want 15022", runNumber)
	}
	// countEvents must rewind the file for the decoding pass
	if pos, _ := file.Seek(0, 1); pos != 0 {
		t.Errorf("file position after countEvents = %d, want 0", pos)
	}
}

func TestGetNextEventReadsAll(t *testing.T) {
	setMainConfig(t, decoder.Configuration{MaxEvents: 100})
	file, err := os.Open(demoFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := NewFileReader(file)

	for i, want := range []uint32{1, 2} {
		header, data, err := reader.getNextEvent()
		if err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
		if decoder.EventIdGetNbInRun(header.EventId) != want {
			t.Errorf("event %d ID = %d, want %d", i, decoder.EventIdGetNbInRun(header.EventId), want)
		}
		if len(data) != int(header.EventSize)-80 {
			t.Errorf("event %d payload = %d, want %d", i, len(data), int(header.EventSize)-80)
		}
	}
	if _, _, err := reader.getNextEvent(); err == nil {
		t.Error("expected error after last event, got nil")
	}
}

func TestGetNextEventMaxEvents(t *testing.T) {
	setMainConfig(t, decoder.Configuration{MaxEvents: 1})
	file, err := os.Open(demoFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := NewFileReader(file)

	if _, _, err := reader.getNextEvent(); err != nil {
		t.Fatalf("first event: %v", err)
	}
	if _, _, err := reader.getNextEvent(); err == nil {
		t.Error("expected EOF after max_events reached, got nil")
	}
}

func TestGetNextEventSkip(t *testing.T) {
	setMainConfig(t, decoder.Configuration{MaxEvents: 100, Skip: 1})
	file, err := os.Open(demoFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := NewFileReader(file)

	header, _, err := reader.getNextEvent()
	if err != nil {
		t.Fatalf("getNextEvent with skip: %v", err)
	}
	if got := decoder.EventIdGetNbInRun(header.EventId); got != 2 {
		t.Errorf("first delivered event ID = %d, want 2 (event 1 skipped)", got)
	}
}
