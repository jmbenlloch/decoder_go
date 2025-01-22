package main

import (
	"fmt"
)

type TriggerData struct {
	TriggerType     uint16   `hdf5:"triggerType"`
	TriggerLost1    uint32   `hdf5:"triggerLost1"`
	TriggerLost2    uint32   `hdf5:"triggerLost2"`
	TriggerMask     uint32   `hdf5:"triggerMask"`
	TriggerDiff1    uint16   `hdf5:"triggerDiff1"`
	TriggerDiff2    uint16   `hdf5:"triggerDiff2"`
	AutoTrigger     uint16   `hdf5:"autoTrigger"`
	DualTrigger     uint16   `hdf5:"dualTrigger"`
	ExternalTrigger uint16   `hdf5:"externalTrigger"`
	Mask            uint16   `hdf5:"mask"`
	TriggerB2       uint16   `hdf5:"triggerB2"`
	TriggerB1       uint16   `hdf5:"triggerB1"`
	ChanA1          uint16   `hdf5:"chanA1"`
	ChanA2          uint16   `hdf5:"chanA2"`
	ChanB1          uint16   `hdf5:"chanB1"`
	ChanB2          uint16   `hdf5:"chanB2"`
	WindowA1        uint16   `hdf5:"windowA1"`
	WindowB1        uint16   `hdf5:"windowB1"`
	WindowA2        uint16   `hdf5:"windowA2"`
	WindowB2        uint16   `hdf5:"windowB2"`
	TriggerIntN     uint16   `hdf5:"triggerIntN"`
	TriggerExtN     uint16   `hdf5:"triggerExtN"`
	TrgChannels     []uint16 `hdf5:"trgChannels"`
}

func ReadTriggerFEC(data []uint16, event *EventType) {
	position := 0

	for dbg := 0; dbg < 9; dbg++ {
		fmt.Printf("TrgConf[%d] = 0x%04x\n", dbg, data[dbg])
	}
	for dbg := 0; dbg < 4; dbg++ {
		fmt.Printf("Ch info[%d] = 0x%04x\n", dbg, data[dbg+9])
	}
	for dbg := 0; dbg < 4; dbg++ {
		fmt.Printf("Trigger lost[%d] = 0x%04x\n", dbg, data[dbg+13])
	}

	//TRG conf 8
	triggerMask := uint32(data[position]&0x003FF) << 16
	position++

	//TRG conf 7
	triggerMask = triggerMask | (uint32(data[position]) & 0x0FFFF)
	position++

	//TRG conf 6
	triggerDiff1 := data[position] & 0x0FFFF
	position++

	//TRG conf 5
	triggerDiff2 := data[position] & 0x0FFFF
	position++

	//TRG conf 4
	triggerWindowA1 := data[position] & 0x003f
	triggerChanA1 := (data[position] & 0x01FC0) >> 6
	autoTrigger := (data[position] & 0x02000) >> 13
	dualTrigger := (data[position] & 0x04000) >> 14
	externalTrigger := (data[position] & 0x08000) >> 15
	position++

	//TRG conf 3
	triggerWindowB1 := data[position] & 0x003f
	triggerChanB1 := (data[position] & 0x01FC0) >> 6
	mask := (data[position] & 0x02000) >> 13
	triggerB2 := (data[position] & 0x04000) >> 14
	triggerB1 := (data[position] & 0x08000) >> 15
	position++

	//TRG conf 2
	triggerWindowA2 := data[position] & 0x003f
	triggerChanA2 := (data[position] & 0x01FC0) >> 6
	position++

	//TRG conf 1
	triggerWindowB2 := data[position] & 0x003f
	triggerChanB2 := (data[position] & 0x01FC0) >> 6
	position++

	//TRG conf 0
	triggerExtN := data[position] & 0x000F
	triggerIntN := (data[position] & 0x0FFF0) >> 4
	position++

	//Trigger type
	triggerType := (data[position] & 0x0FFFF) >> 15
	position++

	//Channels producing trigger
	// Max 48 channels available, 0-47
	// 47-32, 31-16, 15-0
	trgChannels := make([]uint16, 0)
	channelNumber := uint16(47)
	for chinfo := 0; chinfo < 3; chinfo++ {
		for j := 15; j >= 0; j-- {
			activePMT := CheckBit(data[position]&0x0FFFF, uint16(j))
			fmt.Printf("trigger ch %d: %t\n", channelNumber, activePMT)
			if activePMT {
				trgChannels = append(trgChannels, channelNumber)
			}
			channelNumber--
		}
		position++
	}

	//Trigger lost type 2
	triggerLost2 := uint32(data[position]&0x0FFFF) << 16
	position++
	triggerLost2 = triggerLost2 | (uint32(data[position]) & 0x0FFFF)
	position++

	//Trigger lost type 1
	triggerLost1 := uint32(data[position]&0x0FFFF) << 16
	position++
	triggerLost1 = triggerLost1 | (uint32(data[position]) & 0x0FFFF)
	position++

	trgInfo := &event.TriggerConfig
	trgInfo.TriggerType = triggerType
	trgInfo.TriggerLost1 = triggerLost1
	trgInfo.TriggerLost2 = triggerLost2
	trgInfo.TriggerMask = triggerMask
	trgInfo.TriggerDiff1 = triggerDiff1
	trgInfo.TriggerDiff2 = triggerDiff2
	trgInfo.AutoTrigger = autoTrigger
	trgInfo.DualTrigger = dualTrigger
	trgInfo.ExternalTrigger = externalTrigger
	trgInfo.Mask = mask
	trgInfo.TriggerB2 = triggerB2
	trgInfo.TriggerB1 = triggerB1
	trgInfo.ChanA1 = triggerChanA1
	trgInfo.ChanA2 = triggerChanA2
	trgInfo.ChanB1 = triggerChanB1
	trgInfo.ChanB2 = triggerChanB2
	trgInfo.WindowA1 = triggerWindowA1
	trgInfo.WindowB1 = triggerWindowB1
	trgInfo.WindowA2 = triggerWindowA2
	trgInfo.WindowB2 = triggerWindowB2
	trgInfo.TriggerIntN = triggerIntN
	trgInfo.TriggerExtN = triggerExtN
	trgInfo.TrgChannels = trgChannels

	fmt.Printf("trgInfo.TriggerType: %d\n", trgInfo.TriggerType)
	fmt.Printf("trgInfo.TriggerLost1: %d\n", trgInfo.TriggerLost1)
	fmt.Printf("trgInfo.TriggerLost2: %d\n", trgInfo.TriggerLost2)
	fmt.Printf("trgInfo.TriggerMask: %d\n", trgInfo.TriggerMask)
	fmt.Printf("trgInfo.TriggerDiff1: %d\n", trgInfo.TriggerDiff1)
	fmt.Printf("trgInfo.TriggerDiff2: %d\n", trgInfo.TriggerDiff2)
	fmt.Printf("trgInfo.AutoTrigger: %d\n", trgInfo.AutoTrigger)
	fmt.Printf("trgInfo.DualTrigger: %d\n", trgInfo.DualTrigger)
	fmt.Printf("trgInfo.ExternalTrigger: %d\n", trgInfo.ExternalTrigger)
	fmt.Printf("trgInfo.Mask: %d\n", trgInfo.Mask)
	fmt.Printf("trgInfo.TriggerB2: %d\n", trgInfo.TriggerB2)
	fmt.Printf("trgInfo.TriggerB1: %d\n", trgInfo.TriggerB1)
	fmt.Printf("trgInfo.ChanA1: %d\n", trgInfo.ChanA1)
	fmt.Printf("trgInfo.ChanA2: %d\n", trgInfo.ChanA2)
	fmt.Printf("trgInfo.ChanB1: %d\n", trgInfo.ChanB1)
	fmt.Printf("trgInfo.ChanB2: %d\n", trgInfo.ChanB2)
	fmt.Printf("trgInfo.WindowA1: %d\n", trgInfo.WindowA1)
	fmt.Printf("trgInfo.WindowB1: %d\n", trgInfo.WindowB1)
	fmt.Printf("trgInfo.WindowA2: %d\n", trgInfo.WindowA2)
	fmt.Printf("trgInfo.WindowB2: %d\n", trgInfo.WindowB2)
	fmt.Printf("trgInfo.TriggerIntN: %d\n", trgInfo.TriggerIntN)
	fmt.Printf("trgInfo.TriggerExtN: %d\n", trgInfo.TriggerExtN)
	fmt.Printf("trgInfo.TrgChannel: %v\n", trgInfo.TrgChannels)
}

func CheckBit(mask uint16, pos uint16) bool {
	return (mask & (1 << pos)) != 0
}
