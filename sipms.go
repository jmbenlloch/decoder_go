package main

import (
	"encoding/binary"
	"fmt"
	"sort"
)

func ReadSipmFEC(data []uint16, evtFormat *EventFormat, dateHeader *EventHeaderStruct, event *EventType) {
	FecID := evtFormat.FecID
	ZeroSuppression := evtFormat.ZeroSuppression
	CompressedData := evtFormat.CompressedData
	numberOfFEB := evtFormat.NumberOfChannels
	bufferSamples := evtFormat.BufferSamples
	if evtFormat.FWVersion == 10 {
		if evtFormat.TriggerType >= 8 {
			bufferSamples = evtFormat.BufferSamples2
		}
	}

	if huffmanCodesSipms == nil {
		var err error
		huffmanCodesSipms, err = getHuffmanCodesFromDB(dbConn, int(dateHeader.EventRunNb), SiPM)
		if err != nil {
			fmt.Println("Error getting huffman codes from database:", err)
			return
		}
	}

	// Map elecID -> last_value (for decompression)
	lastValues := make(map[uint16]int16)
	chMasks := make(map[uint16][]uint16)

	// Store the payload until we have the two links
	// Each SiPM FEC has two links (with its own FEC ID)
	// Odd words are the first link, even words are the second link
	sipmPayloads[FecID] = data

	// Find partner
	var channelA, channelB uint16
	if FecID%2 == 0 {
		channelA = FecID
		channelB = FecID + 1
	} else {
		channelA = FecID - 1
		channelB = FecID
	}

	// Check if we already have read 2i and 2i+1 channels
	payloadChanA, chanAFound := sipmPayloads[channelA]
	payloadChanB, chanBFound := sipmPayloads[channelB]

	if chanAFound && chanBFound {
		fmt.Println("A pair of SIPM FECs has been read, decoding... ", channelA, channelB)
		// Rebuild payload from the two links
		payload := buildSipmData(payloadChanA, payloadChanB)
		position := 0
		// Print the first 100 bytes in hex from payload separated by spaces
		fmt.Println("sipm data: ")
		for i := 0; i < 50; i++ {
			fmt.Printf("%04x ", payload[i])
		}
		fmt.Println()

		// Print the lastest 100 bytes in hex from payload separated by spaces
		fmt.Println("sipm data end: ")
		for i := len(payload) - 50; i < len(payload); i++ {
			fmt.Printf("%04x ", payload[i])
		}
		fmt.Println()

		// Read data
		time := -1

		var previousFT uint32 = 0
		var nextFT uint32 = 0
		endOfData := false
		for !endOfData {
			time = time + 1
			var j uint16
			for j = 0; j < numberOfFEB; j++ {
				// Stop condition for while and for
				// Before FAFAFAFA there is and FFFFFFFF block signaling the end of the data
				// Sometimes there are some extra words between the end of the data and FAFAFAFA
				// Like this: 4892 ed51 7fff ffff ffff ffff ffff ffff 09c0 2efc fafa fafa fafa fafa
				if (payload[position] == 0xFFFF) && (payload[position+1] == 0xFFFF) {
					endOfData = true
					break
				}

				febID := (payload[position] & 0x0FC00) >> 10
				febInfo := payload[position] & 0x03FF
				emptyFeb := (febInfo & 0x0002) >> 1

				// If there is no data, stop processing this FEB
				if emptyFeb != 0 {
					position++
					continue
				}

				position++
				//fmt.Printf("FEB ID: %x\n", febID)
				//fmt.Printf("j=%d, numberOfFEBs %d, previousFT %x, nextFT %x\n", j, numberOfFEB, previousFT, nextFT)

				FT := uint32(payload[position]) & 0x0FFFF
				if !ZeroSuppression {
					if time < 1 {
						previousFT = FT
					} else {
						BufferSamplesFT := evtFormat.BufferSamples
						if evtFormat.FWVersion == 10 {
							BufferSamplesFT = evtFormat.BufferSamples2
						}

						//New FT only after reading all FEBs in the FEC
						if j == 0 {
							nextFT = ((previousFT + 1) & 0x0FFFF) % (BufferSamplesFT / 40)
						} else {
							nextFT = previousFT
						}
						if nextFT != FT {
							evtNumber := dateHeader.EventId
							fmt.Printf("SiPM Error! Event %d, FECs (0x%x, 0x%x), FEB ID (0x%x, %d), expected FT was 0x%x, current FT is 0x%x, time %d", evtNumber, channelA, channelB, febID, febID, nextFT, FT, time)
							panic("SiPM Error!")
							//fileError_ = true
							//eventError_ = true
							//if discard_ {
							//	return
							//}
						}
						previousFT = nextFT
					}
				}

				var timeinmus uint32
				timeinmus, position = computeSipmTime(payload, position, evtFormat)
				//fmt.Printf("Time in mus: %d\n", timeinmus)

				// If RAW mode, channel mask will appear the first time
				// If ZS mode, channel mask will appear each time
				if time < 1 || ZeroSuppression {
					var chMask []uint16
					chMask, position = sipmChannelMask(payload, position, febID)
					chMasks[febID] = chMask
					fmt.Printf("Channel mask: %v\n", chMask)
					initializeLastValues(lastValues, chMask)
					initializeWaveforms(event.SipmWaveforms, chMask, bufferSamples)
				}

				if ZeroSuppression {
					if CompressedData {
						current_bit := 31
						position = decodeChargeIndiaSipmCompressed(payload, position, event.SipmWaveforms,
							&current_bit, huffmanCodesSipms, chMasks[febID], lastValues, timeinmus)
					} else {
						position = decodeCharge(data, position, event.SipmWaveforms, chMasks[febID], timeinmus)
					}
				} else {
					if CompressedData {
						current_bit := 31
						position = decodeChargeIndiaSipmCompressed(payload, position, event.SipmWaveforms,
							&current_bit, huffmanCodesSipms, chMasks[febID], lastValues, uint32(time))
					} else {
						position = decodeCharge(data, position, event.SipmWaveforms, chMasks[febID], uint32(time))
					}
				}
			}

			// Remove the already processed payloads from the map
			delete(sipmPayloads, channelA)
			delete(sipmPayloads, channelB)
		}

	}
}

// Odd words are in ptrA and even words in ptrB
func buildSipmData(dataA []uint16, dataB []uint16) []uint16 {
	size := len(dataA) + len(dataB)
	data := make([]uint16, size)

	if len(dataA) != len(dataB) {
		panic("Data from both SiPM links must have the same length")
	}

	for i := 0; i < len(dataA); i++ {
		data[i*2] = dataA[i]
		data[i*2+1] = dataB[i]
	}
	return data
}

// Returns FT and new position
func computeSipmTime(data []uint16, position int, evtFormat *EventFormat) (uint32, int) {
	FTBit := evtFormat.FTBit
	TriggerFT := int32(evtFormat.TriggerFT)
	PreTrgSamples := int32(evtFormat.PreTrigger)
	BufferSamples := int32(evtFormat.BufferSamples)
	if evtFormat.FWVersion == 10 {
		BufferSamples = int32(evtFormat.BufferSamples2)
		if evtFormat.TriggerType >= 8 {
			PreTrgSamples = int32(evtFormat.PreTrigger2)
		}
	}
	ZeroSuppression := evtFormat.ZeroSuppression

	FT := int32(data[position]) & 0x0FFFF
	position++ // Send this to main function, somehow

	if ZeroSuppression {
		ringBufferSize := int32(float32(BufferSamples) * CLOCK_TICK)
		var startPosition int32 = ((FTBit << 16) + TriggerFT - PreTrgSamples + BufferSamples) / 40 % ringBufferSize

		// Due to FPGA implementation. To be removed in the future
		// if (((FTBit<<16)+TriggerFT) < PreTrgSamples){
		//     startPosition -= 1;
		// }

		FT = FT - startPosition
		if FT < 0 {
			FT += ringBufferSize
		}
	}

	//fmt.Printf("FT is 0x%04x\n", FT)
	return uint32(FT), position
}

// There are 4 16-bit words with the channel mask for SiPMs
// MSB ch63, LSB ch0
// Data came after chmask, ordered from 0 to 63
// Returns vector with active ElecIDs and new position
func sipmChannelMask(data []uint16, position int, febID uint16) ([]uint16, int) {
	TotalNumberOfSiPMs := 0
	var ElecID uint16
	channelMaskVector := make([]uint16, 0)

	var l, t uint16
	for l = 4; l > 0; l-- {
		for t = 0; t < 16; t++ {
			active := CheckBit(data[position], 15-t)
			ElecID = (febID+1)*1000 + l*16 - t - 1

			if active {
				channelMaskVector = append(channelMaskVector, ElecID)
				TotalNumberOfSiPMs++
			}
		}
		position++
	}

	sort.Slice(channelMaskVector, func(i, j int) bool {
		return channelMaskVector[i] < channelMaskVector[j]
	})

	return channelMaskVector, position
}

func decodeChargeIndiaSipmCompressed(data []uint16, position int,
	waveforms map[uint16][]int16, current_bit *int, huffman *HuffmanNode,
	channelMask []uint16, last_values map[uint16]int16, time uint32) int {

	var dataword uint32 = 0

	for _, channelID := range channelMask {
		if *current_bit < 16 {
			position++
			*current_bit += 16
		}

		// Pack two 16-bit words into a 32-bit word in the correct order
		dataU8 := make([]byte, 4)
		binary.BigEndian.PutUint16(dataU8[0:2], data[position])
		binary.BigEndian.PutUint16(dataU8[2:4], data[position+1])
		dataword = binary.BigEndian.Uint32(dataU8)

		// Get previous value
		previous := last_values[channelID]

		var control_code int32 = 123456
		wfvalue := int16(decode_compressed_value(int32(previous), dataword, control_code, current_bit, huffman))
		last_values[channelID] = wfvalue
		//fmt.Printf("ElecID is %d\t Time is %d\t Charge is 0x%04x\n", channelID, time, wfvalue)

		//Save data in Digits
		waveforms[channelID][time] = wfvalue
	}

	if *current_bit < 15 {
		position += 2 // We have consumed part of the second word
	} else {
		position++ // We are in the first word
	}
	return position
}

func initializeLastValues(lastValues map[uint16]int16, channelMask []uint16) {
	for _, channelID := range channelMask {
		if _, exists := lastValues[channelID]; !exists {
			lastValues[channelID] = 0
		}
	}
}

func initializeWaveforms(waveforms map[uint16][]int16, channelMask []uint16, bufferSamples uint32) {
	for _, channelID := range channelMask {
		if _, exists := waveforms[channelID]; !exists {
			waveforms[channelID] = make([]int16, bufferSamples)
		}
	}
}

func decodeCharge(data []uint16, position int, waveforms map[uint16][]int16, channelMask []uint16, time uint32) int {
	//Raw Mode
	var charge int32 = 0
	positionCharge := position

	//We have 64 SiPM per FEB
	for _, channelID := range channelMask {
		// Pack two 16-bit words into a 32-bit word in the correct order
		dataU8 := make([]byte, 4)
		binary.BigEndian.PutUint16(dataU8[0:2], data[position])
		binary.BigEndian.PutUint16(dataU8[2:4], data[position+1])
		charge = int32(binary.BigEndian.Uint32(dataU8))

		switch channelID % 4 {
		case 0:
			charge = charge >> 20
			charge = charge & 0x0fff
			break
		case 1:
			charge = charge >> 8
			charge = charge & 0x0fff
			break
		case 2:
			charge = charge >> 12
			charge = charge & 0x0fff
			break
		case 3:
			charge = charge & 0x0fff
			break
		}
		if (channelID % 4) == 1 {
			positionCharge++
		}
		if (channelID % 4) == 3 {
			positionCharge += 2
		}

		fmt.Printf("ElecID is %d\t Time is %d\t Charge is 0x%04x", channelMask[channelID], time, charge)

		waveforms[channelID][time] = int16(charge)

		// Channel 3 does not add new words
		// 3 words (0123,4567,89AB) give 4 charges (012,345,678,9AB)
		// (012)(3 45)(67 8)(9AB)
		if (channelID % 4) != 3 {
			position++
		}
	}

	return position
}
