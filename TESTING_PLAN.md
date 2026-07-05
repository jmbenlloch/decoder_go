# Testing assessment and plan for decoder_go

Status date: 2026-07-05. This document is the living plan for building a proper
testing infrastructure for the `decoder/` and `pkg/` packages
(`measureAlgos/` and `_singularity/` are explicitly out of scope).
It is updated as work progresses; each item links to the commit that closed it.

---

## 1. Current status

### 1.1 What exists today

- **2 test files, 5+2 test functions, ~140 lines of tests for ~3900 lines of code:**
  - `pkg/dateReader_test.go` — `flipWords` (even/odd word counts). Regression
    tests for the crash fixed in `e645a06`.
  - `pkg/sipms_test.go` — `buildSipmData` (link merging, length-mismatch panic).
    Same commit.
- **No test runner target**: `mage` only builds; tests are run by hand.
- **No CI**, no coverage tracking, no testdata fixtures in the repo.
- Everything else — DATE header parsing, NEXT header parsing, Huffman
  decompression, PMT/SiPM/fiber payload decoding, trigger parsing, channel-map
  logic, HDF5 writing, configuration loading, the event loop — is untested.

### 1.2 Environment facts (verified)

- Tests must run inside the `duck-backend-test-base:latest` container:
  the host has hdf5 runtime libs but no dev headers, and `pkg` needs CGO +
  libhdf5 through `github.com/next-exp/hdf5-go`. Verified working:

  ```bash
  docker run --rm -v /home/jmbenlloch/next/demo/decoder_go/:/decoder \
    -v /home/jmbenlloch/next/daq/:/daq duck-backend-test-base:latest \
    bash -c "cd /decoder && go test ./..."
  ```

  (container Go: 1.24.13; host Go 1.25.6 exists but cannot link hdf5).
- The internal detector-control MySQL DB `HDDEMODB` is reachable from the
  lab network and contains `ChannelMapping`, `ChannelPosition`,
  `FecElecIDBase`, `HuffmanCodesPmt`, `HuffmanCodesSipm`. DB-dependent tests
  are possible but must be optional (skippable) so the suite does not
  require the network, and must not hardcode the real host or credentials
  since this repo is public.
- Sample raw data available:
  - `/daq/demo/run_15022...rd` (3 MB) and `run_14988...rd` (45 MB) — DEMO++,
    PMTs + SiPMs, DB `DEMOPPDB` at `next.ific.uv.es`.
  - `/daq/hddemo/run_616...rd` (353 MB), `run_672...rd` (58 MB) — HDDEMO,
    fibers + SiPMs, DB `HDDEMODB`. Known-good outputs exist
    (`run_616_db.h5`, `run_616_nodb.h5`, `run_672_*.h5`).

### 1.3 Testability obstacles in the code

1. **Package-level globals**: `configuration`, `logger`, `sensorsMap`,
   `huffmanCodesPmts`, `huffmanCodesSipms`, `fecElecIDBase` (in `pkg`) are set
   by the `decoder` main and by `LoadDatabase`. Tests must set them explicitly.
   A shared test helper (`testmain_test.go` with a no-op logger and a default
   `Configuration`) is needed; without it, most code paths panic on nil
   `logger`. Long term the globals should become parameters/struct fields, but
   the tests come first — refactoring untested decoding code is how detectors
   lose data.
2. **DB coupling**: Huffman trees and channel maps come from MySQL. But the
   structures themselves (`HuffmanNode`, `SensorsMap`, `fecElecIDBase` map)
   are trivial to construct in tests, so decoding logic is testable without DB.
   Only `database.go` itself needs a real (or skipped) DB.
3. **HDF5 output**: `writer.go`/`hdf5.go` call the C library and log errors
   instead of returning them in several paths. Unit-testable parts (sorting,
   ordering, trigger-channel matrix layout) are pure; full writer tests must
   create real files in a temp dir inside the container (cheap, no mocking).
4. **`decoder/` is `package main`** but is testable directly (`go test ./decoder`)
   for `LoadConfiguration`, `countEvents`, `numberOfEventsToProcess`,
   `FileReader.getNextEvent`.
5. **Panics as error handling** in hot decode paths (`buildSipmData`,
   slice indexing on truncated payloads). `processEvent` recovers, so tests
   for malformed input should assert panics/recovery behaviour explicitly.

### 1.4 Suspected defects found while reading the code (to verify with tests)

| # | Location | Suspicion |
|---|----------|-----------|
| B1 | `pkg/nextHeaders.go:38` `readSeqCounter` | `(uint32(data[position+1]) & 0xffff) + (uint32(data[position+1]) << 16)` uses word `position+1` twice; `data[position]` is never read. Sequence counter is wrong whenever the two words differ; the `sequenceCounter == 0` gate works only by accident. |
| B2 | `pkg/writer.go` `WriteEvent` | In DB mode (`no_db: false`) `blrSorted` is never populated (only in the NoDB branch), yet it is used as the channel order for `writeWaveforms(w.BlrWaveforms, ...)` and for the `DataBLR` mapping table → BLR waveforms would be written as all-zeros with an empty mapping whenever dual-mode PMT data is decoded in DB mode. Also `writeBaselines(w.BlrBaselines, ..., pmtSorted, ..., nPmts)` mixes PMT ordering with BLR data. |
| B3 | `decoder/main.go` `numberOfEventsToProcess` + parallel mode | `evtsToRead = maxEvents - skip`, capped by total file events, but the reader delivers at most `min(maxEvents, fileEvtCount) - skip` events. With `skip > 0` and large `max_events`, `processWorkerResults` waits for more results than will ever arrive → deadlock (hang) in `parallel: true` mode. |
| B4 | `decoder/workers.go` `sendEventsToWorkers` | `err == io.EOF` branch is dead code (first `err != nil` breaks); harmless but masks intent. Worker error events are sent with `Error: true` but `processWorkerResults` still counts them against `evtsToRead` — inconsistent with serial mode counting. |
| B5 | `pkg/sipms.go` `ReadSipmFEC` | `delete(sipmPayloads, ...)` executes inside the per-time-bin loop instead of once after decoding; harmless today (idempotent) but fragile. |

Each of these gets: a test that demonstrates current behaviour, then a fix (if
confirmed) in its own commit with the test flipped to the correct expectation.

---

## 2. Target testing infrastructure

### 2.1 Layers

1. **Pure unit tests** (fast, no container-external deps, run on every change):
   bit/word manipulation, header field extraction, elecID/position arithmetic,
   Huffman tree construction + decode, channel-mask expansion, ID
   post-processing (dual mode, HG/LG, X17/X19 fiber swap, ext-trigger/PMT-sum
   extraction), pedestal mapping, raw + compressed charge decoding against
   hand-built payloads, writer ordering functions, JSON config marshalling.
2. **Fixture-based tests** (real binary data, small, committed to repo):
   `pkg/testdata/` holding individual DATE events (~a few hundred KB) extracted
   from `run_15022` (PMTs+SiPMs) and `run_672` (fibers+SiPMs), plus the channel
   map / Huffman codes / FecElecIDBase for those runs dumped to JSON so decoding
   tests do not need MySQL. Tests: `ReadEvent`, `ReadGDC` end-to-end into
   `EventType`, asserting channel counts, waveform lengths, spot-checked sample
   values against the known-good `.h5` outputs.
3. **Writer/round-trip tests** (container, temp files): `NewWriter` →
   `WriteEvent` → `Close` → reopen with hdf5-go and verify groups, dataset
   shapes, mapping tables, trigger tables.
4. **Optional integration tests** (skipped unless enabled):
   - `DECODER_TEST_DB=1` → `database.go` against the real `HDDEMODB`.
   - `DECODER_TEST_BIGDATA=1` → full-file decode of a real run + comparison
     against the known-good `.h5` (golden-file regression, catches any change
     in output semantics).
5. **Runner**: `mage Test` (unit+fixture), `mage TestAll` (everything),
   `test.sh` wrapper that runs them inside the container so a single host
   command runs the suite. CI-ready.

### 2.2 Conventions

- Tests live next to the code (`*_test.go`, same package, to reach unexported
  functions — most of the API surface is unexported).
- A single `testmain_test.go` per package installs a silent logger and a
  default `Configuration` in `TestMain`, so individual tests only override
  what they need.
- Table-driven tests; fixture provenance documented in a comment (run number,
  event ID, extraction command).
- Every bug fix ships with the regression test in the same commit.

---

## 3. Work plan and status

Phases are ordered so that decoding logic gets locked down by tests *before*
any refactoring or bug-fixing that could change output.

- [x] **P0 — This assessment document.**
- [ ] **P1 — Infrastructure**: `mage Test` / `mage TestAll` targets, `test.sh`
      container wrapper, `testmain_test.go` helpers in `pkg` and `decoder`,
      HOWTO section on running tests.
- [ ] **P2 — Pure unit tests, decoding side**: `CheckBit`, `ReadTriggerFEC`,
      Huffman (`parse_huffman_line`, `decode_huffman`,
      `decode_compressed_value` incl. control-code escape), `ReadCommonHeader`
      and every `read*` sub-header (FormatID flags, Juliett event conf,
      baselines bit-packing, FecID, timestamp/FTBit, TriggerFT),
      `computePmtElecID/Position`, `computeFiberElecID/Position`,
      `computeSipmPosition/IDFromPosition`, `sipmChannelMask`,
      `pmtsChannelMask`/`fibersChannelMask` (incl. missing-FEC error),
      `computeNextFThm`, `computeSipmTime` (ZS ring-buffer wrap).
- [ ] **P3 — Pure unit tests, event post-processing**: `processPmtIds`
      (ext-trigger, PMT-sum, dual-mode remap, HG/LG), `processFiberIds`
      (HG remap incl. X17/X19 hardware swap), `writePmtPedestals`/
      `writeFiberPedestals`, `decodeCharge` (4-channel 12-bit packing),
      `decodeChargeIndiaSipmCompressed`/`...PmtCompressed` with a synthetic
      Huffman tree, `initializeWaveforms`.
- [ ] **P4 — Writer + config unit tests**: `sortSensorsBySensorID`,
      `sortSensorsByElecID`, `sortSensorsBySensorIDForWaveforms` (0xFFFF
      fallback), `buildSortedElecIDs`/`buildSortedSensorIDs`,
      `writeTriggerChannels`/`writeTriggerChannelsNoDB` ordering (via real
      temp-file datasets), `BloscAlgorithm`/`BloscShuffle` JSON round-trip,
      `convertToHdf5String`, `LoadConfiguration` defaults + overrides,
      `numberOfEventsToProcess`.
- [ ] **P5 — Fixtures + real-data tests**: extraction tool
      (`cmd or script writing pkg/testdata/*.bin`), DB snapshot to JSON for the
      fixture runs, `ReadEvent`/`ReadGDC` tests asserting decoded waveforms
      against known-good `.h5` values; `countEvents`/`getNextEvent`
      (skip/max_events) tests on a small fixture file.
- [ ] **P6 — Writer round-trip + end-to-end NoDB golden test** in container;
      optional `DECODER_TEST_DB` / `DECODER_TEST_BIGDATA` gated tests.
- [ ] **P7 — Bug verification & fixes** (B1–B5 above), one commit each,
      regression test included. Only after the relevant area is under test.
- [ ] **P8 — Nice-to-have (later)**: dependency-inject globals, error returns
      instead of logged-and-ignored hdf5 errors, CI workflow, coverage target
      (aim: >70% of `pkg`).

## 4. Progress log

| Date | Commit | What |
|------|--------|------|
| 2026-07-05 | (this commit) | P0: assessment and plan. |
