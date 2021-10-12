package subchunk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/icza/bitio"
)

// Encode converts the data in a subchunk.Data and returns bytes that may be written to the leveldb database.
func Encode(d *Data) ([]byte, error) {
	// TODO: work out the length of data and create it with the correct length?
	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	// The first byte is the version which is always either 1 or 8. Always using 8 should be fine as it only
	// seems to dictate whether the storage count can be more than 1. We can still have a storage count of 1 with
	// version 8.
	if err := writeLittleEndian(buf, int8(8)); err != nil {
		return nil, fmt.Errorf("writing storage version: %w", err)
	}

	// If waterLogged blocks are defined we need to write two storage blocks and tell the reader to read them.
	var storageCount int8
	if len(d.Water.Indices) > 0 {
		storageCount = 2
	} else {
		storageCount = 1
	}

	if err := writeLittleEndian(buf, storageCount); err != nil {
		return nil, fmt.Errorf("writing storage version: %w", err)
	}

	return nil, nil
}

func writeStateIndices(w io.Writer, storage *blockStorage) error {
	// The least significant bit of the storage version is always a flag which is always 0 when dealing with local files
	// as opposed to live server data. Bits per block = storageVersion >> 1 to we start with a bitsPerBlock uint8 and
	// shift left one.
	var bitsPerBlock uint8

	// https://slideplayer.com/slide/5767461/19/images/26/%23+different+numbers+-+General+Rule.jpg
	bitsKeyOrder := []int{2, 4, 8, 16, 32, 64, 256, 65536}
	requiredBitsMap := map[int]uint8{
		2:     1,
		4:     2,
		8:     3,
		16:    4,
		32:    5,
		64:    6,
		256:   8,
		65536: 16,
	}
	for _, k := range bitsKeyOrder {
		if k >= len(storage.Palette) {
			bitsPerBlock = requiredBitsMap[k]
		}
	}

	if err := writeLittleEndian(w, bitsPerBlock<<1); err != nil {
		return fmt.Errorf("writing bits per block and storage version byte: %w", err)
	}

	blocksPerWord := int(math.Floor(32.0 / float64(bitsPerBlock)))
	wordCount := int(math.Ceil(BlockCount / float64(blocksPerWord)))

	// Fill wordCount 32 bit integers (bytes slices of length 4) with block state indices
	i := 0
	for wc := 0; wc < wordCount; wc++ {
		buf := bytes.NewBuffer(make([]byte, 4))
		bw := bitio.NewWriter(buf)
		//var word uint32
		for b := 0; b < blocksPerWord; b++ {
			if err := bw.WriteBits(uint64(storage.Indices[i]), bitsPerBlock); err != nil {
				return fmt.Errorf("writing block number %d: %w to 32 bit integer", i, err)
			}
			i++
		}

		if err := writeLittleEndian(w, buf.Bytes()); err != nil {
			return fmt.Errorf("writing 32 bit integer to block storage byte slice: %w", err)
		}

		if wc == 0 {
			fmt.Print("write: ")
			for _, bt := range buf.Bytes() {
				fmt.Printf("%08b", bt)
			}
			fmt.Println()
		}
	}

	return nil
}

func encodeWord(indices []int, bitsPerBlock, blocksPerWord int) uint32 {
	var word uint32
	for i := len(indices) - 1; i >= 0; i-- {
		idx := uint32(indices[i])

		word = word << bitsPerBlock
		word = word | idx

		//s := strconv.FormatInt(int64(idx), 2)
		//fmt.Printf("%s: %03s", "index:", s)
		//PrintNum32(word, "   word:")
	}

	// go test -v -run TestBitStuff
	// go test -v -run TestReadStateIndices

	return word
}

func PrintNum32(n uint32, msg string) {
	s := strconv.FormatInt(int64(n), 2)
	fmt.Printf("%s: %032s\n", msg, s)
}

func PrintNum(n int, msg string) {
	s := strconv.FormatInt(int64(n), 2)
	fmt.Printf("%s: %032s\n", msg, s)
}

func writeLittleEndian(w io.Writer, data interface{}) error {
	return binary.Write(w, binary.ByteOrder(binary.LittleEndian), data)
}
