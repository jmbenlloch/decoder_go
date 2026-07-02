package decoder

import (
	"reflect"
	"testing"
)

// flipWords swaps each adjacent pair of 16-bit words (undoing the FPGA's
// byte-order layout on the wire). These tests assume a little-endian host,
// same as the unsafe.Pointer reinterpretation inside flipWords itself.

func TestFlipWordsEven(t *testing.T) {
	// 4 words / 8 bytes, well below the sequence-counter skip threshold.
	input := []byte{
		0x11, 0x11, // word0 = 0x1111
		0x22, 0x22, // word1 = 0x2222
		0x33, 0x33, // word2 = 0x3333
		0x44, 0x44, // word3 = 0x4444
	}
	want := []uint16{0x2222, 0x1111, 0x4444, 0x3333}

	got := flipWords(input)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flipWords(even) = %#04x, want %#04x", got, want)
	}
}

// TestFlipWordsOdd uses the real, previously-crashing FEC 7 (empty PMT
// stub) equipment payload from run_15022.gdc1demo.demopp.00000, 27 words /
// 54 bytes:
//
//  0. 00000000 0050000a 0f960000 00017d00
//  16. 3e80fa00 7d000000 b0fb68ba b8038057
//  32. fc0000e0 67c79ef0 0177fa00 0000ffff
//  48. fffffafa fafa
//
// The dump above is already in flipWords' output order (verified by
// decoding EventFormat from it field by field). The input bytes below are
// reconstructed by undoing that pairwise word swap, leaving the trailing
// unpaired word (0xfafa) untouched.
func TestFlipWordsOdd(t *testing.T) {
	input := []byte{
		0x00, 0x00, 0x00, 0x00, 0x0a, 0x00, 0x50, 0x00,
		0x00, 0x00, 0x96, 0x0f, 0x00, 0x7d, 0x01, 0x00,
		0x00, 0xfa, 0x80, 0x3e, 0x00, 0x00, 0x00, 0x7d,
		0xba, 0x68, 0xfb, 0xb0, 0x57, 0x80, 0x03, 0xb8,
		0xe0, 0x00, 0x00, 0xfc, 0xf0, 0x9e, 0xc7, 0x67,
		0x00, 0xfa, 0x77, 0x01, 0xff, 0xff, 0x00, 0x00,
		0xfa, 0xfa, 0xff, 0xff, 0xfa, 0xfa,
	}
	want := []uint16{
		0x0000, 0x0000, 0x0050, 0x000a, 0x0f96, 0x0000, 0x0001, 0x7d00,
		0x3e80, 0xfa00, 0x7d00, 0x0000, 0xb0fb, 0x68ba, 0xb803, 0x8057,
		0xfc00, 0x00e0, 0x67c7, 0x9ef0, 0x0177, 0xfa00, 0x0000, 0xffff,
		0xffff, 0xfafa, 0xfafa,
	}

	got := flipWords(input)

	if len(got) != 27 {
		t.Fatalf("flipWords(odd) returned %d words, want 27", len(got))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flipWords(odd) = %#04x, want %#04x", got, want)
	}
}
