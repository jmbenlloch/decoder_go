#!/usr/bin/env python3
"""Extract the first N events of a DATE .rd file into a smaller .rd file.

The .rd format is a plain concatenation of GDC super-events, each one an
80-byte EventHeaderStruct (see pkg/dateHeaders.go) followed by its payload;
EventSize (first uint32) includes the header. Used to produce the committed
test fixtures under pkg/testdata/.

Usage: extract_rd_events.py <input.rd> <output.rd> <n_events>
"""
import struct
import sys

HEADER_SIZE = 80


def main() -> None:
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    src, dst, n_events = sys.argv[1], sys.argv[2], int(sys.argv[3])

    copied = 0
    with open(src, "rb") as fin, open(dst, "wb") as fout:
        while copied < n_events:
            header = fin.read(HEADER_SIZE)
            if len(header) < HEADER_SIZE:
                break
            size, magic = struct.unpack_from("<2I", header)
            if magic != 0xDA1E5AFE:
                sys.exit(f"bad magic {magic:#x} at event {copied}")
            payload = fin.read(size - HEADER_SIZE)
            if len(payload) != size - HEADER_SIZE:
                sys.exit(f"truncated event {copied}")
            fout.write(header)
            fout.write(payload)
            copied += 1

    print(f"wrote {copied} events to {dst}")


if __name__ == "__main__":
    main()
