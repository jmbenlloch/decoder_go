package main

func ReadCommonHeader(data []uint16) EventFormat {
	position := 0
	evtFormat := EventFormat{}
	sequenceCounter, position := readSeqCounter(data, position)
	//fmt.Println("Sequence Counter:", sequenceCounter)
	if sequenceCounter == 0 {
		position = readFormatID(data, position, &evtFormat)
		position = readWordCount(data, position, &evtFormat)
		position = readEventID(data, position, &evtFormat)
		if evtFormat.FWVersion == 10 {
			position = readEventConfJuliett(data, position, &evtFormat)
		}
		if evtFormat.FWVersion >= 9 {
			if evtFormat.Baseline {
				position = readIndiaBaselines(data, position, &evtFormat)
			}
			position = readIndiaFecID(data, position, &evtFormat)
		}
		position = readCTandFTh(data, position, &evtFormat)
		position = readFTl(data, position, &evtFormat)
	}

	evtFormat.HeaderSize = uint16(position)
	return evtFormat

}

func readSeqCounter(data []uint16, position int) (uint32, int) {
	sequenceCounter := (uint32(data[position+1]) & 0x0ffff) + (uint32(data[position+1]) << 16)
	position += 2
	return sequenceCounter, position
}

type EventFormat struct {
	FecType          uint16
	ZeroSuppression  bool
	CompressedData   bool
	Baseline         bool
	DualModeBit      bool
	ErrorBit         bool
	FWVersion        uint16
	WordCount        uint16
	TriggerType      uint16
	TriggerCounter   uint32
	BufferSamples    uint32
	PreTrigger       uint32
	BufferSamples2   uint32
	PreTrigger2      uint32
	ChannelMask      uint16
	TriggerFT        uint16
	Timestamp        uint64
	FTBit            int32
	NumberOfChannels uint16
	FecID            uint16
	Baselines        []uint16
	HeaderSize       uint16
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

func readFormatID(data []uint16, position int, evtFormat *EventFormat) int {
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

	//fmt.Println("FecType:", FecType)
	//fmt.Println("ZeroSuppression:", ZeroSuppression)
	//fmt.Println("CompressedData:", CompressedData)
	//fmt.Println("Baseline:", Baseline)
	//fmt.Println("DualModeBit:", DualModeBit)
	//fmt.Println("ErrorBit:", ErrorBit)
	//fmt.Println("FWVersion:", FWVersion)

	evtFormat.FecType = FecType
	evtFormat.ZeroSuppression = ZeroSuppression > 0
	evtFormat.CompressedData = CompressedData > 0
	evtFormat.Baseline = Baseline > 0
	evtFormat.DualModeBit = DualModeBit > 0
	evtFormat.ErrorBit = ErrorBit > 0
	evtFormat.FWVersion = FWVersion

	return position
}

func readWordCount(data []uint16, position int, evtFormat *EventFormat) int {
	WordCounter := data[position] & 0x0FFFF
	position++
	//fmt.Println("Word Counter:", WordCounter)
	evtFormat.WordCount = WordCounter
	return position
}

func readEventID(data []uint16, position int, evtFormat *EventFormat) int {
	TriggerType := data[position] & 0x000F
	TriggerCounter := (uint32(data[position]&0x0FFF0) << 12) + (uint32(data[position+1]) & 0x0FFFF)
	position += 2
	//fmt.Println("Trigger Type:", TriggerType)
	//fmt.Println("Trigger Counter:", TriggerCounter)
	evtFormat.TriggerType = TriggerType
	evtFormat.TriggerCounter = TriggerCounter
	return position
}

func readEventConfJuliett(data []uint16, position int, evtFormat *EventFormat) int {
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

	evtFormat.BufferSamples = BufferSamples
	evtFormat.PreTrigger = PreTriggerSamples
	evtFormat.BufferSamples2 = BufferSamples2
	evtFormat.PreTrigger2 = PreTriggerSamples2
	evtFormat.ChannelMask = ChannelMask

	//fmt.Println("Buffer Samples:", BufferSamples)
	//fmt.Println("PreTrigger Samples:", PreTriggerSamples)
	//fmt.Println("Buffer Samples2:", BufferSamples2)
	//fmt.Println("PreTrigger Samples2:", PreTriggerSamples2)
	//fmt.Println("Channel Mask:", ChannelMask)
	return position
}

func readIndiaBaselines(data []uint16, position int, evtFormat *EventFormat) int {
	// Baselines
	// Pattern goes like this:
	// 0xFFF0, 0x000F, 12 bits,  4 bits; ch0, ch1
	// 0xFF00, 0x00FF,  8 bits,  8 bits; ch1, ch2
	// 0x000F, 0x0FFF,  4 bits, 12 bits; ch2, ch3
	// 0xFFF0, 0x000F, 12 bits,  4 bits; ch4, ch5
	// 0xFF00, 0x0000,  8 bits,  8 bits; ch5
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
	baselineTemp = baselineTemp + ((data[position] & 0xFF00) >> 8)
	baselines = append(baselines, baselineTemp)
	position++

	evtFormat.Baselines = baselines

	//fmt.Println("Baselines:", baselines)
	return position
}

func readIndiaFecID(data []uint16, position int, evtFormat *EventFormat) int {
	NumberOfChannels := data[position] & 0x001F
	FecID := (data[position] & 0x0FFE0) >> 5
	position++

	//fmt.Println("Number of Channels:", NumberOfChannels)
	//fmt.Println("Fec ID:", FecID)

	evtFormat.NumberOfChannels = NumberOfChannels
	evtFormat.FecID = FecID
	return position
}

func readCTandFTh(data []uint16, position int, evtFormat *EventFormat) int {
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

	// We use 32 bits to avoid overflow later
	FTBit := int32((data[position] & 0x8000) >> 15)
	position++

	//fmt.Println("Timestamp:", Timestamp)
	//fmt.Println("FTBit:", FTBit)

	evtFormat.Timestamp = Timestamp
	evtFormat.FTBit = FTBit

	return position
}

func readFTl(data []uint16, position int, evtFormat *EventFormat) int {
	TriggerFT := data[position] & 0x0FFFF
	position++
	//fmt.Println("TriggerFT:", TriggerFT)

	evtFormat.TriggerFT = TriggerFT

	return position
}
