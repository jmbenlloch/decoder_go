package decoder

import (
	"fmt"
)

func ReadPmtFEC(data []uint16, evtFormat *EventFormat, dateHeader *EventHeaderStruct, event *EventType) error {
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

	channelMask, chPositions, err := pmtsChannelMask(evtFormat)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
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
	return nil
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

func pmtsChannelMask(evtFormat *EventFormat) ([]uint16, []uint16, error) {
	channelMaskVec := make([]uint16, 0)
	// To avoid using the map for every waveform sample we are keeping another
	// vector with the pointers to the waveforms. This positions vector indicates
	// the position of the waveform in the waveforms pointer array.
	positions := make([]uint16, 0)

	var t uint16
	for t = 0; t < 16; t++ {
		active := CheckBit(evtFormat.ChannelMask, t)
		if active {
			elecID, err := computePmtElecID(evtFormat.FecID, t, evtFormat.FWVersion)
			if err != nil {
				return nil, nil, err
			}
			channelMaskVec = append(channelMaskVec, elecID)
			positions = append(positions, computePmtPosition(elecID))
		}
	}

	if configuration.Verbosity > 2 {
		message := fmt.Sprintf("Channel mask: %v", channelMaskVec)
		logger.Info(message, "pmts")
	}
	return channelMaskVec, positions, nil
}

func computePmtPosition(elecID uint16) uint16 {
	position := elecID % 100 / 2
	//fmt.Println("ElecID is ", elecID, " Position is ", position)
	return position
}

func computePmtElecID(fecID uint16, channel uint16, fwversion uint16) (uint16, error) {
	var elecID uint16

	if fwversion >= 10 {
		base, ok := fecElecIDBase[fecID]
		if !ok {
			return 0, fmt.Errorf("no elecID base found for FEC %d", fecID)
		}
		elecID = channel*2 + (fecID % 2) + base
	}

	return elecID, nil
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
		if event.PmtConfig.DualMode {
			if elecID%100 >= 12 {
				newid := elecID - 12
				event.BlrWaveforms[newid] = waveform
				event.BlrBaselines[newid] = event.Baselines[elecID]
				delete(event.PmtWaveforms, elecID)
				delete(event.Baselines, elecID)
			}
		}

		// High-low gain channels
		if event.PmtConfig.ChannelsHG {
			if elecID%2 == 1 {
				event.BlrWaveforms[elecID] = waveform
				event.BlrBaselines[elecID] = event.Baselines[elecID]
				delete(event.PmtWaveforms, elecID)
				delete(event.Baselines, elecID)
			}
		}
	}
}
