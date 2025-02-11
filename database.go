package main

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	sqlx "github.com/jmoiron/sqlx" //make alias name the package to sqlx
)

func ConnectToDatabase(user string, pass string, host string, dbname string) (*sqlx.DB, error) {
	port := "3306"
	dbURI := fmt.Sprintf("%s:%s@(%s:%s)/%s?parseTime=true", user, pass, host, port, dbname)
	db, err := sqlx.Connect("mysql", dbURI)
	return db, err
}

type SensorType int

const (
	SiPM SensorType = iota
	PMT
)

func (s SensorType) String() string {
	switch s {
	case SiPM:
		return "SiPM"
	case PMT:
		return "PMT"
	default:
		return "Unknown"
	}
}

type HuffmanCode struct {
	Value int
	Code  string
}

type SensorMappingEntry struct {
	ElecID   int `db:"ElecID"`
	SensorID int `db:"SensorID"`
}

func getHuffmanCodesFromDB(db *sqlx.DB, runNumber int, sensor SensorType) (*HuffmanNode, error) {
	var query string
	switch sensor {
	case SiPM:
		query = "SELECT value, code from HuffmanCodesSipm WHERE MinRun <= %d and MaxRun >= %d"
	case PMT:
		query = "SELECT value, code from HuffmanCodesPmt WHERE MinRun <= %d and MaxRun >= %d"
	}

	query = fmt.Sprintf(query, runNumber, runNumber)
	if VerbosityLevel > 0 {
		message := fmt.Sprintf("Reading %v Huffman Codes from database", sensor)
		InfoLog.Info(message, "module", "database")
	}
	if VerbosityLevel > 2 {
		message := fmt.Sprintf("Query: %s", query)
		InfoLog.Info(message, "module", "database")
	}
	rows, err := db.Queryx(query)
	if err != nil {
		errMessage := fmt.Errorf("error querying database: %w", err)
		return nil, errMessage
	}

	huffman := &HuffmanNode{
		NextNodes: [2]*HuffmanNode{nil, nil},
	}

	for rows.Next() {
		result := HuffmanCode{}
		err := rows.StructScan(&result)
		if err != nil {
			errMessage := fmt.Errorf("error scanning DB row: %w", err)
			return nil, errMessage
		}
		parse_huffman_line(int32(result.Value), result.Code, huffman)
	}
	//printfHuffman(huffman, 1)
	return huffman, nil
}

func getSensorsFromDB(db *sqlx.DB, runNumber int) (SensorsMap, error) {
	query := "SELECT ElecID, SensorID FROM ChannelMapping WHERE MinRun <= %d and MaxRun >= %d ORDER BY SensorID"
	query = fmt.Sprintf(query, runNumber, runNumber)

	if VerbosityLevel > 0 {
		InfoLog.Info("Channel mapping read from DB", "module", "database")
	}
	if VerbosityLevel > 2 {
		message := fmt.Sprintf("Query: %s", query)
		InfoLog.Info(message, "module", "database")
	}

	rows, err := db.Queryx(query)
	if err != nil {
		errMessage := fmt.Errorf("error querying database: %w", err)
		return SensorsMap{}, errMessage
	}

	npmts := 0
	nsipms := 0
	threshold := 999
	sensorsMap := SensorsMap{
		Pmts: SensorMapping{
			ToElecID:   make(map[uint16]uint16),
			ToSensorID: make(map[uint16]uint16),
		},
		Sipms: SensorMapping{
			ToElecID:   make(map[uint16]uint16),
			ToSensorID: make(map[uint16]uint16),
		},
	}

	for rows.Next() {
		result := SensorMappingEntry{}
		err := rows.StructScan(&result)
		if err != nil {
			errMessage := fmt.Errorf("error scanning DB row: %w", err)
			return SensorsMap{}, errMessage
		}
		if result.ElecID < threshold {
			npmts += 1
			sensorsMap.Pmts.ToElecID[uint16(result.SensorID)] = uint16(result.ElecID)
			sensorsMap.Pmts.ToSensorID[uint16(result.ElecID)] = uint16(result.SensorID)
		} else {
			nsipms += 1
			sensorsMap.Sipms.ToElecID[uint16(result.SensorID)] = uint16(result.ElecID)
			sensorsMap.Sipms.ToSensorID[uint16(result.ElecID)] = uint16(result.SensorID)
		}
	}
	return sensorsMap, nil
}
