package main

import "fmt"

func ReadCommonHeader(data []uint16) {
	position := 0
	sequenceCounter, position := readSeqCounter(data, position)
	fmt.Println("Sequence Counter:", sequenceCounter)
	if sequenceCounter == 0 {
		format, position := readFormatID(data, position)
		position = readWordCount(data, position)
		position = readEventID(data, position)
		if format.FWVersion == 10 {
			position = readEventConfJuliett(data, position)
		}
		if format.FWVersion >= 9 {
			if format.Baseline > 0 {
				position = readIndiaBaselines(data, position)
			}
			position = readIndiaFecID(data, position)
		}
		position = readCTandFTh(data, position)
		position = readFTl(data, position)
	}
}

func readSeqCounter(data []uint16, position int) (uint32, int) {
	var sequenceCounter uint32
	sequenceCounter = uint32((data[position+1] & 0x0ffff) + (data[position+1] << 16))
	position += 2
	return sequenceCounter, position
}

type FormatID struct {
	FecType         uint16
	ZeroSuppression uint16
	CompressedData  uint16
	Baseline        uint16
	DualModeBit     uint16
	ErrorBit        uint16
	FWVersion       uint16
}

func readFormatID(data []uint16, position int) (FormatID, int) {
	//Format ID H
	FecType := data[position] & 0x000F
	ZeroSuppression := (data[position] & 0x0010) >> 4
	CompressedData := (data[position] & 0x0020) >> 5
	Baseline := (data[position] & 0x0040) >> 6
	DualModeBit := (data[position] & 0x0080) >> 7
	ErrorBit := (data[position] & 0x4000) >> 14
	position++

	//Format ID L
	FWVersion := data[position] & 0x0FFFF
	position++

	formatData := FormatID{FecType, ZeroSuppression, CompressedData, Baseline, DualModeBit, ErrorBit, FWVersion}

	fmt.Println("FecType:", FecType)
	fmt.Println("ZeroSuppression:", ZeroSuppression)
	fmt.Println("CompressedData:", CompressedData)
	fmt.Println("Baseline:", Baseline)
	fmt.Println("DualModeBit:", DualModeBit)
	fmt.Println("ErrorBit:", ErrorBit)
	fmt.Println("FWVersion:", FWVersion)

	return formatData, position
}

func readWordCount(data []uint16, position int) int {
	WordCounter := data[position] & 0x0FFFF
	position++
	fmt.Println("Word Counter:", WordCounter)
	return position
}

func readEventID(data []uint16, position int) int {
	TriggerType := data[position] & 0x000F
	TriggerCounter := ((data[position] & 0x0FFF0) << 12) + (data[position+1] & 0x0FFFF)
	position += 2
	fmt.Println("Trigger Type:", TriggerType)
	fmt.Println("Trigger Counter:", TriggerCounter)
	return position
}

func readEventConfJuliett(data []uint16, position int) int {
	//Event conf0
	BufferSamples := 2 * uint32(data[position]&0x0FFFF)
	position++

	//Event conf1
	PreTriggerSamples := 2 * uint32(data[position]&0x0FFFF)
	position++

	//Event conf2
	BufferSamples2 := 2 * uint32(data[position]&0x0FFFF)
	position++

	//Event conf3
	PreTriggerSamples2 := 2 * uint32(data[position]&0x0FFFF)
	position++

	//Event conf4
	ChannelMask := data[position] & 0x0FFFF
	position++

	fmt.Println("Buffer Samples:", BufferSamples)
	fmt.Println("PreTrigger Samples:", PreTriggerSamples)
	fmt.Println("Buffer Samples2:", BufferSamples2)
	fmt.Println("PreTrigger Samples2:", PreTriggerSamples2)
	fmt.Println("Channel Mask:", ChannelMask)
	return position
}

func readIndiaBaselines(data []uint16, position int) int {
	// Baselines
	// Pattern goes like this (two times):
	// 0xFFF0, 0x000F, 12 bits,  4 bits; ch0, ch1
	// 0xFF00, 0x00FF,  8 bits,  8 bits; ch1, ch2
	// 0x000F, 0x0FFF,  4 bits, 12 bits; ch2, ch3
	var baselineTemp uint16
	baselines := make([]uint16, 0)

	//Baseline ch0
	baselineTemp = (data[position] & 0xFFF0) >> 4
	baselines = append(baselines, baselineTemp)

	//Baseline ch1
	baselineTemp = (data[position] & 0x000F) << 8
	position++
	baselineTemp = baselineTemp + ((data[position] & 0xFF00) >> 8)
	baselines = append(baselines, baselineTemp)

	//Baseline ch2
	baselineTemp = (data[position] & 0x00FF) << 4
	position++
	baselineTemp = baselineTemp + ((data[position] & 0xF000) >> 12)
	baselines = append(baselines, baselineTemp)

	//Baseline ch3
	baselineTemp = (data[position] & 0x0FFF)
	baselines = append(baselines, baselineTemp)

	//Baseline ch4
	position++
	baselineTemp = (data[position] & 0xFFF0) >> 4
	baselines = append(baselines, baselineTemp)
	baselineTemp = (data[position] & 0x000F) << 8

	//Baseline ch5
	position++
	// TODO: Check last line of this block. Maybe there is an error with last bits
	baselineTemp = baselineTemp + ((data[position] & 0xFF00) >> 8)
	baselines = append(baselines, baselineTemp)
	baselineTemp = (data[position] & 0x00FF) << 4
	position++

	fmt.Println("Baselines:", baselines)
	return position
}

func readIndiaFecID(data []uint16, position int) int {
	NumberOfChannels := data[position] & 0x001F
	FecId := (data[position] & 0x0FFE0) >> 5
	position++

	fmt.Println("Number of Channels:", NumberOfChannels)
	fmt.Println("Fec ID:", FecId)
	return position
}

func readCTandFTh(data []uint16, position int) int {
	//Timestamp high
	var Timestamp uint64
	Timestamp = uint64((data[position] & 0x0FFFF)) << 16
	position++

	//Timestamp Low
	Timestamp = Timestamp + uint64((data[position] & 0x0ffff))
	position++

	//FTH & CTms
	Timestamp = (Timestamp << 10) + uint64((data[position] & 0x03FF))
	Timestamp = Timestamp & 0x03FFFFFFFFFF

	//FTBit := CheckBit(data[position], 15)
	FTBit := (data[position] & 0x8000) > 0
	position++

	fmt.Println("Timestamp:", Timestamp)
	fmt.Println("FTBit:", FTBit)

	return position
}

func readFTl(data []uint16, position int) int {
	TriggerFT := data[position] & 0x0FFFF
	position++
	fmt.Println("TriggerFT:", TriggerFT)

	return position
}
