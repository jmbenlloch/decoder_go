package main

import (
	"fmt"
)

type WorkerData struct {
	Data   []byte
	Header EventHeaderStruct
}

func worker(id int, jobs <-chan WorkerData, results chan<- EventType) {
	for event := range jobs {
		fmt.Printf("Worker %d processing event %d\n", id, event.Header.EventId)
		//fmt.Println("Data size:", len(event.Data), "Header: ", event.Header)
		event := readGDC(event.Data, event.Header)
		results <- event
	}
}
