package decoder

import (
	"reflect"
	"testing"
)

func TestComputeFiberElecID(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{6: 500, 7: 500})

	cases := []struct {
		fecID   uint16
		channel uint16
		want    uint16
	}{
		{6, 0, 500},
		{6, 4, 508},
		{7, 0, 501},
		{7, 9, 519},
	}
	for _, c := range cases {
		got, err := computeFiberElecID(c.fecID, c.channel, 10)
		if err != nil {
			t.Fatalf("computeFiberElecID(%d, %d): %v", c.fecID, c.channel, err)
		}
		if got != c.want {
			t.Errorf("computeFiberElecID(%d, %d) = %d, want %d", c.fecID, c.channel, got, c.want)
		}
	}

	if _, err := computeFiberElecID(42, 0, 10); err == nil {
		t.Fatal("computeFiberElecID with unknown FEC: expected error, got nil")
	}
}

func TestComputeFiberPosition(t *testing.T) {
	cases := []struct{ elecID, want uint16 }{
		{500, 0}, {501, 0}, {510, 5}, {519, 9}, {611, 5},
	}
	for _, c := range cases {
		if got := computeFiberPosition(c.elecID); got != c.want {
			t.Errorf("computeFiberPosition(%d) = %d, want %d", c.elecID, got, c.want)
		}
	}
}

func TestFibersChannelMask(t *testing.T) {
	setFecElecIDBase(t, map[uint16]uint16{6: 500})

	evtFormat := EventFormat{
		FecID:       6,
		FWVersion:   10,
		ChannelMask: 0b0000100000000011, // channels 0, 1, 11
	}
	chMask, positions, err := fibersChannelMask(&evtFormat)
	if err != nil {
		t.Fatalf("fibersChannelMask: %v", err)
	}
	if !reflect.DeepEqual(chMask, []uint16{500, 502, 522}) {
		t.Errorf("channel mask = %v, want [500 502 522]", chMask)
	}
	if !reflect.DeepEqual(positions, []uint16{0, 1, 11}) {
		t.Errorf("positions = %v, want [0 1 11]", positions)
	}
}
