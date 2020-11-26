package mcdata

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"

	"github.com/midnightfreddie/nbt2json"
)

// subChunk is block data for a 16x16x16 area of the map.
type subChunk struct {
	data         []byte         // The raw sub chunk data
	Version      int            // The version of the data format (may be 1 or 8)
	StorageCount int            // Count of Block storage records (unused if version is set to 1)
	BlockStorage []BlockStorage // Zero or more concatenated Block Storage records, as specified by the count
	// (or 1 if version is set to 1).
}

type BlockStorage struct {
	version int

	BlockStateIndices []int // The block states as indices into the palette, packed into
	// ceil(4096 / blocksPerWord) 32-bit little-endian unsigned integers.

	paletteSize uint32 // A 32-bit little-endian integer specifying the number of block states in the
	// palette.

	blockStates []Tag // The specified number of block states in little-endian NBT format, concatenated.
}

type Tag struct {
	TagType int
	Name    string
	Value   interface{}
}

func newTag(data interface{}) Tag {
	d := data.(map[string]interface{})
	return Tag{
		TagType: int(d["tagType"].(float64)),
		Name:    d["name"].(string),
		Value:   d["value"],
	}
}

// Columns a slice of slices where each sub slice is a column covering the extents of the Y axis within this sub chunk.
func (b *BlockStorage) Columns() ([][]int, error) {
	blockSize := 16

	for y := 0; y < blockSize; y++ {

		for z := 0; z < blockSize; z++ {

			for x := 0; x < blockSize; x++ {

			}
		}
	}
	// Blocks are stored column-by-column (incrementing Y first, incrementing Z at the end of the column, incrementing
	// X at the end of the cross-section).
	return nil, nil
}

// BlockName returns the name of the block associated with the block state
func (b *BlockStorage) BlockName(index int) (string, error) {
	if tag, ok := b.tag("name", index); ok {
		return tag["value"].(string), nil
	}

	return "", fmt.Errorf("reading block name: no tag found with name 'name'")
}

// BlockState returns all state tags associated with the block state
func (b *BlockStorage) BlockStateTags(index int) ([]Tag, error) {
	s, ok := b.tag("states", index)

	if !ok {
		return nil, fmt.Errorf("block has no 'states' tag")
	}

	states := s["value"].([]interface{})
	stateTags := make([]Tag, len(states))

	for i, s := range states {
		stateTags[i] = newTag(s)
	}

	return stateTags, nil
}

func (b *BlockStorage) tag(name string, index int) (map[string]interface{}, bool) {
	state := b.blockStates[index]

	for _, t := range state.Value.([]interface{}) {
		tag := t.(map[string]interface{})
		if tag["name"] == name {
			return tag, true
		}
	}

	return nil, false
}

func NewSubChunk(data []byte) (subChunk, error) {
	r := bytes.NewReader(data)

	version := int(readByte(r))

	switch version {
	case 1:
		log.Fatal("HANDLE SUBCHUNK TYPE 1")
		return subChunk{}, nil
	case 8:
		// Number of BlockStorage objects to read
		storageCount := int(readBytes(r, 1)[0])

		blocks := make([]BlockStorage, storageCount)

		// Read BlockStorage data and create objects
		for i := 0; i < storageCount; i++ {
			b, err := readBlockStorage(r)
			if err != nil {
				return subChunk{}, fmt.Errorf("creating new block: %s", err)
			}

			blocks[i] = b
		}

		return subChunk{
			data:         data,
			Version:      version,
			BlockStorage: blocks,
		}, nil
	default:
		panic("sub chunk had version other than 1 or 8")
	}
}

func readBlockStorage(data *bytes.Reader) (BlockStorage, error) {
	// Version and bitsPerBlock in a single byte
	storageVersionByte := readByte(data)

	// The version (0 or 1)
	storageVersionFlag := int((storageVersionByte >> 1) & 1)

	// Number of bits used for one block state index
	bitsPerBlock := int(storageVersionByte >> 1)

	// Number of blocks per 32-bit integer
	blocksPerWord := math.Floor(float64(32 / bitsPerBlock))

	// Total count of block state indices
	indexCount := 4096 // int(math.Ceil(4096/blocksPerWord)) * int(blocksPerWord)

	if 32%int(blocksPerWord) != 0 { // TODO: Handle all blocksPerword amounts https://minecraft.gamepedia.com/Bedrock_Edition_level_format
		// "For the blocksPerWord values which are not factors of 32, each 32-bit integer contains two (high) bits of padding. Block state indices are not split across words."
		// Probably need to handle: "Block state indices are *not split across words*"
		// log.Fatalf("blocksPerWord value of %f is not a factor of 32", blocksPerWord)
		return BlockStorage{}, fmt.Errorf("blocksPerWord value of %f is not a factor of 32", blocksPerWord)
	}

	if bitsPerBlock != 4 { // TODO: Handle all bitsPerBlock amounts https://minecraft.gamepedia.com/Bedrock_Edition_level_format
		// log.Fatal("bitsPerBlock is not 4")
		return BlockStorage{}, fmt.Errorf("bitsPerBlock is not 4")
	}

	dataBits := NewBitReader(data)

	indices := make([]int, indexCount)
	for i := 0; i < indexCount; i++ {
		// Read one block
		idxBits, err := dataBits.ReadBits(bitsPerBlock)
		if err != nil {
			return BlockStorage{}, nil
		}

		// Index of this block's state in the palette
		idx := int(boolsToBytes(idxBits)[0] >> 4) // TODO: see if statement above, this is specific to a bitsPerBlock value of 4. Because we are converting 4 bits to a byte, we shift it 4 bits to the right to get the correct value.
		indices[i] = idx
	}

	if dataBits.Offset() != 8 { // TODO: This does not necessarily mean things are broken
		log.Fatalf("finished reading indices of size %d bits part way through a byte", bitsPerBlock)
	}

	// Number of blocks states in the palette
	paletteSize := binary.LittleEndian.Uint32(readBytes(data, 4))

	// Read all the remaining bytes. This is the NBT block states.
	remaining, err := ioutil.ReadAll(data)
	if err != nil {
		return BlockStorage{}, fmt.Errorf("reading remaining bytes: %s", err)
	}

	// Convert the BNT to JSON then unmarshal the JSON.
	jsn, err := nbt2json.Nbt2Json(remaining, "#")
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshal JSON NBT data
	var nbtJsonData struct {
		Nbt []interface{} `json:"nbt"`
	}
	err = json.Unmarshal(jsn, &nbtJsonData)

	if err != nil {
		return BlockStorage{}, fmt.Errorf("unmarshaling nbt json data: %s", err)
	}

	// Construct tags from empty interfaces
	blockStates := make([]Tag, paletteSize)
	for i, j := range nbtJsonData.Nbt {
		blockStates[i] = newTag(j)
	}

	blockStorage := BlockStorage{
		version:           storageVersionFlag,
		BlockStateIndices: indices,
		paletteSize:       paletteSize,
		blockStates:       blockStates,
	}

	return blockStorage, nil
}

// func reads count byte from reader and returns, or exits the program if reader.Read() returns an error.
func readBytes(reader *bytes.Reader, count int) []byte {
	b := make([]byte, count)
	_, err := reader.Read(b)

	if err != nil {
		log.Fatalf("attempting to read bytes for subchunk: %s", err)
	}

	return b
}
func readByte(reader *bytes.Reader) byte {
	return readBytes(reader, 1)[0]
}

func boolsToBytes(t []bool) []byte {
	b := make([]byte, (len(t)+7)/8)
	for i, x := range t {
		if x {
			b[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return b
}

func bytesToBools(b []byte) []bool {
	t := make([]bool, 8*len(b))
	for i, x := range b {
		for j := 0; j < 8; j++ {
			if (x<<uint(j))&0x80 == 0x80 {
				t[8*i+j] = true
			}
		}
	}
	return t
}
