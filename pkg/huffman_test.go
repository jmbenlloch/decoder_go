package decoder

import "testing"

// testHuffmanTree builds a small tree resembling the real DB codes: short
// codes for small deltas plus a long code for the control (escape) value.
//
//	"1"    -> 0
//	"01"   -> +1
//	"001"  -> -1
//	"0001" -> control code (escape to 12-bit raw value)
func testHuffmanTree(controlCode int32) *HuffmanNode {
	root := &HuffmanNode{}
	parse_huffman_line(0, "1", root)
	parse_huffman_line(1, "01", root)
	parse_huffman_line(-1, "001", root)
	parse_huffman_line(controlCode, "0001", root)
	return root
}

func TestParseHuffmanLineTreeShape(t *testing.T) {
	root := testHuffmanTree(123456)

	// "1" is a leaf directly under the root
	leaf1 := root.NextNodes[1]
	if leaf1 == nil || leaf1.NextNodes[0] != nil || leaf1.NextNodes[1] != nil {
		t.Fatal("node for code \"1\" is not a leaf")
	}
	if leaf1.Value != 0 {
		t.Errorf("value for code \"1\" = %d, want 0", leaf1.Value)
	}
	// "001" walks 0 -> 0 -> 1
	n := root.NextNodes[0].NextNodes[0].NextNodes[1]
	if n == nil || n.Value != -1 {
		t.Fatalf("value for code \"001\" = %v, want -1", n)
	}
	// shared prefix nodes must not have been duplicated: "0001" continues
	// from the same "00" node
	n = root.NextNodes[0].NextNodes[0].NextNodes[0].NextNodes[1]
	if n == nil || n.Value != 123456 {
		t.Fatalf("value for code \"0001\" = %v, want 123456", n)
	}
}

func TestDecodeHuffman(t *testing.T) {
	root := testHuffmanTree(123456)
	cases := []struct {
		name     string
		code     uint32
		startBit int
		wantVal  int32
		wantPos  int
	}{
		// 1-bit code "1" starting at bit 31: consumes 1 bit
		{"code 1", 0x80000000, 31, 0, 30},
		// "01" starting at bit 31
		{"code 01", 0x40000000, 31, 1, 29},
		// "001" starting at bit 31
		{"code 001", 0x20000000, 31, -1, 28},
		// same code lower in the word: "01" at bits 20..19
		{"code 01 offset", 0x00080000, 20, 1, 18},
	}
	for _, c := range cases {
		var result int32
		pos := decode_huffman(root, c.code, c.startBit, &result)
		if result != c.wantVal || pos != c.wantPos {
			t.Errorf("%s: decode_huffman = (%d, pos %d), want (%d, pos %d)",
				c.name, result, pos, c.wantVal, c.wantPos)
		}
	}
}

func TestDecodeCompressedValueDelta(t *testing.T) {
	root := testHuffmanTree(123456)

	// "01" at bit 31 -> delta +1 on previous value
	startBit := 31
	got := decode_compressed_value(100, 0x40000000, 123456, &startBit, root)
	if got != 101 {
		t.Errorf("delta decode = %d, want 101", got)
	}
	if startBit != 29 {
		t.Errorf("startBit after delta decode = %d, want 29", startBit)
	}
}

func TestDecodeCompressedValueControlCode(t *testing.T) {
	root := testHuffmanTree(123456)

	// "0001" at bits 31..28 selects the control code, then the next 12 bits
	// (27..16) carry the raw value 0xABC. Previous value must be ignored.
	data := uint32(0x1<<28) | uint32(0xABC)<<16
	startBit := 31
	got := decode_compressed_value(999, data, 123456, &startBit, root)
	if got != 0xABC {
		t.Errorf("control-code decode = %#x, want 0xABC", got)
	}
	if startBit != 15 {
		t.Errorf("startBit after control-code decode = %d, want 15", startBit)
	}
}

// Consecutive decodes from one 32-bit word: "1" (delta 0), "01" (+1),
// "001" (-1) packed MSB-first.
func TestDecodeCompressedValueSequence(t *testing.T) {
	root := testHuffmanTree(123456)

	// bits: 1 01 001 ... -> 0b101001 at bits 31..26
	data := uint32(0b101001) << 26
	startBit := 31

	v1 := decode_compressed_value(10, data, 123456, &startBit, root)
	v2 := decode_compressed_value(v1, data, 123456, &startBit, root)
	v3 := decode_compressed_value(v2, data, 123456, &startBit, root)

	if v1 != 10 || v2 != 11 || v3 != 10 {
		t.Errorf("sequence = %d, %d, %d, want 10, 11, 10", v1, v2, v3)
	}
	if startBit != 25 {
		t.Errorf("startBit after sequence = %d, want 25", startBit)
	}
}
