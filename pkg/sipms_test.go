package decoder

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSipmDataEqualLength(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222, 0x3333}
	dataB := []uint16{0xaaaa, 0xbbbb, 0xcccc}
	want := []uint16{0x1111, 0xaaaa, 0x2222, 0xbbbb, 0x3333, 0xcccc}

	got := buildSipmData(dataA, dataB)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSipmData(equal) = %#04x, want %#04x", got, want)
	}
}

// TestBuildSipmDataAOneLonger covers the normal hardware condition where the
// interleaved stream has an odd total word count, so link A (odd positions)
// ends up with exactly one more word than link B.
func TestBuildSipmDataAOneLonger(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222, 0x3333}
	dataB := []uint16{0xaaaa, 0xbbbb}
	want := []uint16{0x1111, 0xaaaa, 0x2222, 0xbbbb, 0x3333}

	got := buildSipmData(dataA, dataB)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSipmData(A one longer) = %#04x, want %#04x", got, want)
	}
}

// TestBuildSipmDataBOneLonger is the mirror case: link B ends up with the
// extra trailing word instead of link A.
func TestBuildSipmDataBOneLonger(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222}
	dataB := []uint16{0xaaaa, 0xbbbb, 0xcccc}
	want := []uint16{0x1111, 0xaaaa, 0x2222, 0xbbbb, 0xcccc}

	got := buildSipmData(dataA, dataB)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSipmData(B one longer) = %#04x, want %#04x", got, want)
	}
}

func TestBuildSipmDataEmpty(t *testing.T) {
	got := buildSipmData(nil, nil)

	if len(got) != 0 {
		t.Fatalf("buildSipmData(empty) = %#04x, want empty", got)
	}
}

// TestBuildSipmDataLargeMismatchPanics ensures a difference of more than one
// word is still treated as a real error, not silently tolerated.
func TestBuildSipmDataLargeMismatchPanics(t *testing.T) {
	dataA := []uint16{0x1111, 0x2222, 0x3333, 0x4444}
	dataB := []uint16{0xaaaa}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("buildSipmData did not panic on a >1 word length mismatch")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "must have the same length") {
			t.Fatalf("panic value = %v, want message about mismatched link length", r)
		}
	}()

	buildSipmData(dataA, dataB)
}
