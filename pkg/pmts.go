package decoder

import (
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

	// Reading the payload
	var nextFT int32 = -1 //At start we don't know next FT value
	var nextFThm int32 = -1

	channelMask, chPositions := pmtsChannelMask(evtFormat)
	initializeWaveforms(event.PmtWaveforms, channelMask, bufferSamples)
	wfPointers := computePmtWaveformPointerArray(event.PmtWaveforms, channelMask, chPositions)

	// Write pedestal
	if Baseline {
		//fmt.Println("Baselines: ", event.Baselines)
		writePmtPedestals(evtFormat, channelMask, event.Baselines)
	}

	//TODO maybe size of payload could be used here to stop, but the size is
	//2x size per link and there are many FFFF at the end, which are the actual
	//stop condition...
	for true {
		time++

		// Stop reading if we reach the last time bin of the waveform
		if time == int(bufferSamples) {
			break
		}

		if Compression {
			// Skip FTm
			if time == 0 {
				position++
			}
			position = decodeChargeIndiaPmtCompressed(data, position, wfPointers,
				&current_bit, huffmanCodesPmts, chPositions, uint32(time))
		} else {
			var FT int32 = int32(data[position]) & 0x0FFFF
			position++

			//If not ZS check next FT value, if not expected (0xffff) end of data
			computeNextFThm(&nextFT, &nextFThm, evtFormat)
			if FT != (nextFThm & 0x0FFFF) {
				// Check with run 13868 DEMO.
				errMessage := fmt.Errorf("evt %d, fecID: %d, nextFThm != FT: 0x%04x, 0x%04x",
					EventIdGetNbInRun(dateHeader.EventId), fFecId, (nextFThm & 0x0ffff), FT)
				logger.Error(errMessage.Error())
				break
			}
			position = decodeCharge(data, position, wfPointers, chPositions, uint32(time))
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

	if configuration.Verbosity > 3 {
		message := fmt.Sprintf("nextFThm: 0x%05x\tnextFT: 0x%05x", *nextFThm, *nextFT)
		logger.Info(message, "pmts")
	}
}

func decodeChargeIndiaPmtCompressed(data []uint16, position int, waveforms []*[]int16,
	current_bit *int, huffman *HuffmanNode, channelMask []uint16, time uint32) int {
	var dataword uint32 = 0

	for _, channelID := range channelMask {
		if *current_bit < 16 {
			position++
			*current_bit += 16
		}
		// Pack two 16-bit words into a 32-bit word in the correct order
		dataword = (uint32(data[position]) << 16) | uint32(data[position+1])

		// Get previous value
		waveform := *waveforms[channelID%100]
		var previous int16 = 0
		if time > 0 {
			previous = waveform[time-1]
		}

		var control_code int32 = 123456
		wfvalue := int16(decode_compressed_value(int32(previous), dataword, control_code, current_bit, huffman))

		if configuration.Verbosity > 3 {
			message := fmt.Sprintf("ElecID %d, time %d, charge 0x%04x", channelID, time, wfvalue)
			logger.Info(message, "pmts")
		}

		waveform[time] = wfvalue
	}
	return position
}

func computePmtWaveformPointerArray(waveforms map[uint16][]int16, chmask []uint16, positions []uint16) []*[]int16 {
	MAX_PMTs_PER_FEC := 12
	wfPointers := make([]*[]int16, MAX_PMTs_PER_FEC)
	for i, elecID := range chmask {
		position := positions[i]
		wf := waveforms[elecID]
		wfPointers[position] = &wf
	}
	return wfPointers
}

func pmtsChannelMask(evtFormat *EventFormat) ([]uint16, []uint16) {
	var elecID uint16

	channelMaskVec := make([]uint16, 0)
	// To avoid using the map for every waveform sample we are keeping another
	// vector with the pointers to the waveforms. This positions vector indicates
	// the position of the waveform in the waveforms pointer array.
	positions := make([]uint16, 0)

	var t uint16
	for t = 0; t < 16; t++ {
		active := CheckBit(evtFormat.ChannelMask, t)
		if active {
			elecID = computePmtElecID(evtFormat.FecID, t, evtFormat.FWVersion)
			channelMaskVec = append(channelMaskVec, elecID)
			positions = append(positions, computePmtPosition(elecID))
		}
	}

	if configuration.Verbosity > 2 {
		message := fmt.Sprintf("Channel mask: %v", channelMaskVec)
		logger.Info(message, "pmts")
	}
	return channelMaskVec, positions
}

func computePmtPosition(elecID uint16) uint16 {
	position := elecID % 100 / 2
	//fmt.Println("ElecID is ", elecID, " Position is ", position)
	return position
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
		baseline_index := ((elecID % 100) % 12) / 2
		baselines[elecID] = evtFormat.Baselines[baseline_index]
	}
}

func processPmtIds(event *EventType, configuration Configuration) {
	extTriggerCh := configuration.ExtTrigger
	for elecID, waveform := range event.PmtWaveforms {
		// Check external trigger
		if elecID == uint16(extTriggerCh) {
			event.ExtTrgWaveform = &waveform
			delete(event.PmtWaveforms, elecID)
			delete(event.Baselines, elecID)
		}

		// Check PMT sum waveform
		if elecID == uint16(configuration.PmtSumCh) {
			event.PmtSumWaveform = &waveform
			event.PmtSumBaseline = event.Baselines[elecID]
			delete(event.PmtWaveforms, elecID)
			delete(event.Baselines, elecID)
		}

		// Check dual channels
		// dual channels are used to send the original waveform and the BLR one
		// They appear as two different channels, but the signal comes from the same
		// physical channel
		// 100 - 111 -> 100 - 111 Real
		// 112 - 123 -> 100 - 111 Dual
		// 200 - 211 -> 200 - 211 Real
		// 212 - 223 -> 200 - 211 Dual
		// 300 - 311 -> 300 - 311 Real
		// 312 - 323 -> 300 - 311 Dual
		// 400 - 411 -> 400 - 411 Real
		// 412 - 423 -> 400 - 411 Dual
		// 500 - 511 -> 500 - 511 Real
		// 512 - 523 -> 500 - 511 Dual
		// 600 - 611 -> 600 - 611 Real
		// 612 - 623 -> 600 - 611 Dual
		// 700 - 711 -> 700 - 711 Real
		// 712 - 723 -> 700 - 711 Dual

		// Dual channel
		if elecID%100 >= 12 {
			newid := elecID - 12
			event.BlrWaveforms[newid] = waveform
			event.BlrBaselines[newid] = event.Baselines[elecID]
			delete(event.PmtWaveforms, elecID)
			delete(event.Baselines, elecID)
		}
	}
}
