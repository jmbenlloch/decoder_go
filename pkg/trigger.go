package decoder

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
			//fmt.Printf("trigger ch %d: %t\n", channelNumber, activePMT)
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

	if configuration.Verbosity > 2 {
		message := fmt.Sprintf("TriggerType: %d", trgInfo.TriggerType)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerLost1: %d", trgInfo.TriggerLost1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerLost2: %d", trgInfo.TriggerLost2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerMask: %d", trgInfo.TriggerMask)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerDiff1: %d", trgInfo.TriggerDiff1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerDiff2: %d", trgInfo.TriggerDiff2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("AutoTrigger: %d", trgInfo.AutoTrigger)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("DualTrigger: %d", trgInfo.DualTrigger)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("ExternalTrigger: %d", trgInfo.ExternalTrigger)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("Mask: %d", trgInfo.Mask)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerB2: %d", trgInfo.TriggerB2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerB1: %d", trgInfo.TriggerB1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("ChanA1: %d", trgInfo.ChanA1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("ChanA2: %d", trgInfo.ChanA2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("ChanB1: %d", trgInfo.ChanB1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("ChanB2: %d", trgInfo.ChanB2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("WindowA1: %d", trgInfo.WindowA1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("WindowB1: %d", trgInfo.WindowB1)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("WindowA2: %d", trgInfo.WindowA2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("WindowB2: %d", trgInfo.WindowB2)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerIntN: %d", trgInfo.TriggerIntN)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TriggerExtN: %d", trgInfo.TriggerExtN)
		logger.Info(message, "trigger")
		message = fmt.Sprintf("TrgChannel: %v", trgInfo.TrgChannels)
		logger.Info(message, "trigger")
	}

}

func CheckBit(mask uint16, pos uint16) bool {
	return (mask & (1 << pos)) != 0
}
