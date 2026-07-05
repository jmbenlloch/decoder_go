package decoder

import (
	"os"
	"testing"
)

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestLoadDatabaseHddemo checks database.go against a live MySQL instance
// (a disposable local/CI container seeded from
// pkg/testdata/hddemo_616_seed.sql by default) and cross-validates the
// result with the committed JSON snapshot for run 616.
// Gated: needs network access to MySQL; enable with DECODER_TEST_DB=1.
// Connection details are overridable via DECODER_TEST_DB_{HOST,USER,PASS,NAME}
// so this never needs real production credentials hardcoded in source.
func TestLoadDatabaseHddemo(t *testing.T) {
	if os.Getenv("DECODER_TEST_DB") == "" {
		t.Skip("set DECODER_TEST_DB=1 to run DB integration tests")
	}

	fixture := loadDBFixture(t, "testdata/hddemo_616_db.json")

	host := getenvDefault("DECODER_TEST_DB_HOST", "127.0.0.1")
	user := getenvDefault("DECODER_TEST_DB_USER", "root")
	pass := getenvDefault("DECODER_TEST_DB_PASS", "decoder_test_password")
	dbname := getenvDefault("DECODER_TEST_DB_NAME", "HDDEMODB")

	dbConn, err := ConnectToDatabase(user, pass, host, dbname)
	if err != nil {
		t.Fatalf("connecting to %s: %v", dbname, err)
	}
	defer dbConn.Close()

	// Fixture loading has installed the snapshot globals; LoadDatabase
	// overwrites them from the live DB. The t.Cleanup registered by
	// loadDBFixture restores the originals afterwards.
	snapshotBase := make(map[uint16]uint16, len(fecElecIDBase))
	for k, v := range fecElecIDBase {
		snapshotBase[k] = v
	}

	if err := LoadDatabase(dbConn, 616); err != nil {
		t.Fatalf("LoadDatabase: %v", err)
	}

	if huffmanCodesPmts == nil || huffmanCodesSipms == nil {
		t.Fatal("huffman trees not loaded")
	}
	// The PMT huffman tree must decode something: walk code "1" or "0..."
	if huffmanCodesPmts.NextNodes[0] == nil && huffmanCodesPmts.NextNodes[1] == nil {
		t.Error("PMT huffman tree is empty")
	}

	if len(fecElecIDBase) != len(snapshotBase) {
		t.Errorf("FecElecIDBase entries = %d, snapshot has %d",
			len(fecElecIDBase), len(snapshotBase))
	}
	for fec, base := range snapshotBase {
		if fecElecIDBase[fec] != base {
			t.Errorf("FecElecIDBase[%d] = %d, snapshot says %d", fec, fecElecIDBase[fec], base)
		}
	}

	// Channel mapping counts must match the snapshot per sensor type
	var wantSipms, wantFibers, wantPmts int
	for _, cm := range fixture.ChannelMapping {
		switch {
		case len(cm.Label) >= 4 && cm.Label[:4] == "SiPM":
			wantSipms++
		case len(cm.Label) >= 5 && cm.Label[:5] == "Fiber":
			wantFibers++
		case len(cm.Label) >= 3 && cm.Label[:3] == "PMT":
			wantPmts++
		}
	}
	if len(sensorsMap.Sipms.ToSensorID) != wantSipms {
		t.Errorf("SiPM mappings = %d, snapshot has %d", len(sensorsMap.Sipms.ToSensorID), wantSipms)
	}
	if len(sensorsMap.Fibers.ToSensorID) != wantFibers {
		t.Errorf("Fiber mappings = %d, snapshot has %d", len(sensorsMap.Fibers.ToSensorID), wantFibers)
	}
	if len(sensorsMap.Pmts.ToSensorID) != wantPmts {
		t.Errorf("PMT mappings = %d, snapshot has %d", len(sensorsMap.Pmts.ToSensorID), wantPmts)
	}
}
