package main

import "fmt"

type HuffmanNode struct {
	NextNodes [2]*HuffmanNode
	Value     int32
}

func parse_huffman_line(value int32, code string, huffman *HuffmanNode) {
	current_node := huffman
	// printf("code: %s, value: %d\n", code, value);

	var bit uint8
	for bitcount := 0; bitcount < len(code); bitcount++ {
		bit = code[bitcount] - 0x30 // ascii to int
		// printf("code[%d]: %c\n", bitcount, code[bitcount]);

		if current_node.NextNodes[bit] != nil {
			//fmt.Printf("bit already set\n")
			current_node = current_node.NextNodes[bit]
			//fmt.Printf("huffman-debug: %v\n", current_node)
		} else {
			//fmt.Printf("bit not set\n")
			newNode := HuffmanNode{
				NextNodes: [2]*HuffmanNode{nil, nil},
			}
			//fmt.Printf("new_node: %v\n", newNode)

			current_node.NextNodes[bit] = &newNode
			//fmt.Printf("huffman-debug-2-1: %v\n", current_node)
			current_node = &newNode
			//fmt.Printf("huffman-debug-2-2: %v\n", current_node)
		}
	}

	// printf("bitcount: %d, %d\n", bitcount, bit);
	current_node.Value = value
}

func printfHuffman(huffman *HuffmanNode, code int) {
	// printf("code: %d\n", code);
	// printf("next[0]: 0x%x\n", huffman->next[0]);
	// printf("next[1]: 0x%x\n", huffman->next[1]);
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

	// printf("call decode_huffman: data 0x%08x, current_bit: %d\n", data, current_bit);
	current_bit = decode_huffman(huffman, data, current_bit, &wfvalue)
	// printf("value: 0x%04x\n", wfvalue);

	if wfvalue == control_code {
		wfvalue = (int32(data) >> (current_bit - 11)) & 0x0FFF
		current_bit -= 12
		// printf("12-bit wfvalue: %d\n", wfvalue);
	} else {
		wfvalue = previous_value + wfvalue
	}
	*start_bit = current_bit
	// printf("curren bit: %d\n", current_bit);

	return wfvalue
}

func decode_huffman(huffman *HuffmanNode, code uint32, position int, result *int32) int {
	bit := (code >> position) & 0x01
	// printf("pos: %d, bit: %d\n", position, bit);

	final_pos := position

	// printf("next[0]: 0x%x\n", huffman->next[0]);
	// printf("next[1]: 0x%x\n", huffman->next[1]);

	if (huffman.NextNodes[0] == nil) && (huffman.NextNodes[1] == nil) {
		*result = huffman.Value
	} else {
		final_pos = decode_huffman(huffman.NextNodes[bit], code, position-1, result)
	}
	// printf("code: 0x%04x, pos: %d, result: %d\n", code, position, *result);
	// printf("final_pos: %d\n", final_pos);

	return final_pos
}
