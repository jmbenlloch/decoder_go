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
		fmt.Println("SiPM Huffman codes read from DB")
	case PMT:
		query = "SELECT value, code from HuffmanCodesPmt WHERE MinRun <= %d and MaxRun >= %d"
		fmt.Println("PMTHuffman codes read from DB")
	}

	query = fmt.Sprintf(query, runNumber, runNumber)
	fmt.Println("Query: ", query)
	rows, err := db.Queryx(query)
	if err != nil {
		fmt.Println("Error querying database: ", err)
	}

	huffman := &HuffmanNode{
		NextNodes: [2]*HuffmanNode{nil, nil},
	}

	for rows.Next() {
		result := HuffmanCode{}
		err := rows.StructScan(&result)
		if err != nil {
			fmt.Println("Error scanning DB row:", err)
		}
		parse_huffman_line(int32(result.Value), result.Code, huffman)
	}
	//printfHuffman(huffman, 1)
	return huffman, nil
}

func getSensorsFromDB(db *sqlx.DB, runNumber int) SensorsMap {
	query := "SELECT ElecID, SensorID FROM ChannelMapping WHERE MinRun <= %d and MaxRun >= %d ORDER BY SensorID"
	query = fmt.Sprintf(query, runNumber, runNumber)
	fmt.Println("Query: ", query)
	fmt.Println("Channel mapping read from DB")

	rows, err := db.Queryx(query)
	if err != nil {
		fmt.Println("Error querying database: ", err)
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
			fmt.Println("Error scanning DB row:", err)
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
	return sensorsMap
}
