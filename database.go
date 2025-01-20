package main

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	sqlx "github.com/jmoiron/sqlx" //make alias name the package to sqlx
)

func ConnectToDatabase() (*sqlx.DB, error) {
	user := "nextreader"
	password := "readonly"
	host := "next.ific.uv.es"
	port := "3306"
	database := "NEXT100DB"
	dbURI := fmt.Sprintf("%s:%s@(%s:%s)/%s?parseTime=true", user, password, host, port, database)
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
	//	parse_huffman_line(std::stoi(row[0]), row[1], huffman);

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
	printfHuffman(huffman, 1)
	return huffman, nil
}
