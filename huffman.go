package main

import "fmt"

type HuffmanNode struct {
	NextNodes [2]*HuffmanNode
	Value     int32
}

func parse_huffman_line(value int32, code string, huffman *HuffmanNode) {
	current_node := huffman

	var bit uint8
	for bitcount := 0; bitcount < len(code); bitcount++ {
		bit = code[bitcount] - 0x30 // ascii to int
		if current_node.NextNodes[bit] != nil {
			current_node = current_node.NextNodes[bit]
		} else {
			newNode := HuffmanNode{
				NextNodes: [2]*HuffmanNode{nil, nil},
			}
			current_node.NextNodes[bit] = &newNode
			current_node = &newNode
		}
	}
	current_node.Value = value
}

func printfHuffman(huffman *HuffmanNode, code int) {
	for i := 0; i < 2; i++ {
		if huffman.NextNodes[i] != nil {
			newCode := code*10 + i
			printfHuffman(huffman.NextNodes[i], newCode)
		}
	}

	if (huffman.NextNodes[0] == nil) && (huffman.NextNodes[1] == nil) {
		fmt.Printf("%d -> %d\n", code, huffman.Value)
	}
}

// start_bit will be modified to set the new position
func decode_compressed_value(previous_value int32, data uint32, control_code int32, start_bit *int, huffman *HuffmanNode) int32 {
	// Check data type (0 uncompressed, 1 huffman)
	current_bit := *start_bit

	var wfvalue int32
	current_bit = decode_huffman(huffman, data, current_bit, &wfvalue)

	if wfvalue == control_code {
		wfvalue = (int32(data) >> (current_bit - 11)) & 0x0FFF
		current_bit -= 12
	} else {
		wfvalue = previous_value + wfvalue
	}
	*start_bit = current_bit

	return wfvalue
}

func decode_huffman(huffman *HuffmanNode, code uint32, position int, result *int32) int {
	bit := (code >> position) & 0x01

	final_pos := position

	if (huffman.NextNodes[0] == nil) && (huffman.NextNodes[1] == nil) {
		*result = huffman.Value
	} else {
		final_pos = decode_huffman(huffman.NextNodes[bit], code, position-1, result)
	}
	return final_pos
}
