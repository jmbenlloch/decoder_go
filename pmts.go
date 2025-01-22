package main

import (
	"encoding/binary"
	"fmt"
)

func ReadPmtFEC(data []uint16, evtFormat *EventFormat, dateHeader *EventHeaderStruct, event *EventType) {
	position := 0
	var time int = -1
	var current_bit int = 31

	fFecId := evtFormat.FecID
	// PMTs do not have ZS, only compression mode
	Compression := evtFormat.ZeroSuppression
	Baseline := evtFormat.Baseline
	bufferSamples := evtFormat.BufferSamples
	if evtFormat.FWVersion == 10 {
		if evtFormat.TriggerType >= 8 {
			bufferSamples = evtFormat.BufferSamples2
		}
	}

	if Compression {
		if huffmanCodesPmts == nil {
			var err error
			huffmanCodesPmts, err = getHuffmanCodesFromDB(dbConn, int(dateHeader.EventRunNb), PMT)
			if err != nil {
				fmt.Println("Error getting huffman codes from database:", err)
				return
			}
		}
	}

	// TODO: Deal with error bit
	if evtFormat.ErrorBit {
		evtNumber := dateHeader.EventId
		fmt.Printf("Event %d ErrorBit is %t, fec: 0x%x", evtNumber, evtFormat.ErrorBit, fFecId)
		panic("Error bit is set")
		//fileError_ = true;
		//eventError_ = true;
		//if(discard_){
		//	return;
		//}
	}

	// Map elecID -> last_value (for decompression)
	//lastValues := make(map[uint16]int32)
	//chMasks := make(map[uint16][]uint16)

	// Reading the payload
	var nextFT int32 = -1 //At start we don't know next FT value
	var nextFThm int32 = -1

	channelMask := pmtsChannelMask(evtFormat)
	initializeWaveforms(event.PmtWaveforms, channelMask, bufferSamples)

	// Write pedestal
	if Baseline {
		fmt.Println("Baselines: ", event.Baselines)
		writePmtPedestals(evtFormat, channelMask, event.Baselines)
	}

	//TODO maybe size of payload could be used here to stop, but the size is
	//2x size per link and there are many FFFF at the end, which are the actual
	//stop condition...
	for true {
		// timeinmus = timeinmus + CLOCK_TICK_;
		time++
		//fmt.Println("Time is ", time)

		if Compression {
			// Skip FTm
			if time == 0 {
				position++
			}
			if time == int(bufferSamples) {
				break
			}
			position = decodeChargeIndiaPmtCompressed(data, position, event.PmtWaveforms,
				&current_bit, huffmanCodesPmts, channelMask, uint32(time))
		} else {
			var FT int32 = int32(data[position]) & 0x0FFFF
			position++

			//If not ZS check next FT value, if not expected (0xffff) end of data
			if !Compression {
				computeNextFThm(&nextFT, &nextFThm, evtFormat)
				if FT != (nextFThm & 0x0FFFF) {
					fmt.Printf("nextFThm != FT: 0x%04x, 0x%04x\n", (nextFThm & 0x0ffff), FT)
					break
				}
			}
			position = decodeCharge(data, position, event.PmtWaveforms, channelMask, uint32(time))
		}
	}
}

func computeNextFThm(nextFT *int32, nextFThm *int32, evtFormat *EventFormat) {
	PreTrgSamples := int32(evtFormat.PreTrigger)
	BufferSamples := int32(evtFormat.BufferSamples)
	if evtFormat.FWVersion == 10 {
		BufferSamples = int32(evtFormat.BufferSamples2)
		if evtFormat.TriggerType >= 8 {
			PreTrgSamples = int32(evtFormat.PreTrigger2)
		}
	}
	FTBit := evtFormat.FTBit
	TriggerFT := evtFormat.TriggerFT

	//Compute actual FT taking into account FTh bit
	// FTm = FT - PreTrigger
	if *nextFT == -1 {
		*nextFT = (FTBit << 16) + (int32(TriggerFT) & 0x0FFFF)
		// Decrease one due to implementation in FPGA
		// if (PreTrgSamples > *nextFT){
		//     *nextFT -= 1;
		// }
	} else {
		*nextFT = (*nextFT + 1) % int32(BufferSamples)
	}

	//Compute FTm (with high (17) bit)
	if PreTrgSamples > *nextFT {
		*nextFThm = BufferSamples - PreTrgSamples + *nextFT
	} else {
		*nextFThm = *nextFT - int32(PreTrgSamples)
	}

	//if( verbosity_ >= 3 ){
	//	_log->debug("nextFThm: 0x{:05x}", *nextFThm);
	//	_log->debug("nextFT: 0x{:05x}", *nextFT);
	//}
}

func decodeChargeIndiaPmtCompressed(data []uint16, position int, waveforms map[uint16][]int16,
	current_bit *int, huffman *HuffmanNode, channelMask []uint16, time uint32) int {
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
		var previous int16 = 0
		if time > 0 {
			previous = waveforms[channelID][time-1]
		}

		var control_code int32 = 123456
		wfvalue := int16(decode_compressed_value(int32(previous), dataword, control_code, current_bit, huffman))

		//fmt.Printf("ElecID is %d\t Time is %d\t Charge is 0x%04x\n", channelID, time, wfvalue)

		waveforms[channelID][time] = wfvalue
	}
	return position
}

func pmtsChannelMask(evtFormat *EventFormat) []uint16 {
	var elecID uint16

	channelMaskVec := make([]uint16, 0)

	var t uint16
	for t = 0; t < 16; t++ {
		active := CheckBit(evtFormat.ChannelMask, t)
		if active {
			elecID = computePmtElecID(evtFormat.FecID, t, evtFormat.FWVersion)
			// printf("channelmask: elecid: %d\tpmtid: %d\n", ElecID, pmtID);
			channelMaskVec = append(channelMaskVec, elecID)
		}
	}

	fmt.Printf("Channel mask is %v\n", channelMaskVec)
	return channelMaskVec
}

func computePmtElecID(fecID uint16, channel uint16, fwversion uint16) uint16 {
	var elecID uint16

	if fwversion >= 10 {
		// fec: 02: 100, 102, 104, 106, 108, 110, 112, 114, 116, 118, 120, 122
		// fec: 03: 101, 103, 105, 107, 109, 111, 113, 115, 117, 119, 121, 123
		// fec: 06: 200, 202, 204, 206, 208, 210, 212, 214, 216, 218, 220, 222
		// fec: 07: 201, 203, 205, 207, 209, 211, 213, 215, 217, 219, 221, 223
		// fec: 10: 300, 302, 304, 306, 308, 310, 312, 314, 316, 318, 320, 322
		// fec: 11: 301, 303, 305, 307, 309, 311, 313, 315, 317, 319, 321, 323
		// fec: 14: 400, 402, 404, 406, 408, 410, 412, 414, 416, 418, 420, 422
		// fec: 15: 401, 403, 405, 407, 409, 411, 413, 415, 417, 419, 421, 423
		// fec: 18: 500, 502, 504, 506, 508, 510, 512, 514, 516, 518, 520, 522
		// fec: 19: 501, 503, 505, 507, 509, 511, 513, 515, 517, 519, 521, 523
		// fec: 22: 600, 602, 604, 606, 608, 610, 612, 614, 616, 618, 620, 622
		// fec: 23: 601, 603, 605, 607, 609, 611, 613, 615, 617, 619, 621, 623
		// fec: 26: 700, 702, 704, 706, 708, 710, 712, 714, 716, 718, 720, 722
		// fec: 27: 701, 703, 705, 707, 709, 711, 713, 715, 717, 719, 721, 723

		// Test code
		//var fecs = []uint16{2, 3, 6, 7, 10, 11, 14, 15, 18, 19, 22, 23, 26, 27}
		//var i, j uint16
		//for i = 0; i < 14; i++ {
		//	for j = 0; j < 12; j++ {
		//		fecid := fecs[i]
		//		channel = j
		//		elecID = channel*2 + (fecid % 2)
		//		elecID += (((fecid - 2) / 4) + 1) * 100
		//		fmt.Printf("fec: %d\tchannel: %d\t elecid: %d\n", fecid, channel, elecID)
		//	}
		//}
		elecID = channel*2 + (fecID % 2)
		elecID += (((fecID - 2) / 4) + 1) * 100
	}

	return elecID
}

func writePmtPedestals(evtFormat *EventFormat, channelMask []uint16, baselines map[uint16]uint16) {
	for _, elecID := range channelMask {
		// Only 6 baselines are sent per FEC
		//  2 -> 0,2,4,6,8,10,12,14,16,18,20,22
		//       0,2,4,6,8,10      -> 0,1,2,3,4,5
		//       12,14,16,18,20,22 -> 0,1,2,3,4,5
		//  3 -> 1,3,5,7,9,11,13,15,17,19,21,23
		//       1,3,5,7,9,11      -> 0,1,2,3,4,5
		//       13,15,17,19,21,23 -> 0,1,2,3,4,5
		// 10 -> 24,26,28, ..., 46
		//       24,26,28,...      -> 0,1,2,3,4,5
		//       36,38,40,...      -> 0,1,2,3,4,5
		// 11 -> 25,27,29, ..., 47
		//       25,27,29,...      -> 0,1,2,3,4,5
		//       37,39,41,...      -> 0,1,2,3,4,5
		baseline_index := (elecID % 12) / 2
		baselines[elecID] = evtFormat.Baselines[baseline_index]
	}
}
