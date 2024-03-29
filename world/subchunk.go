package world

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"

	"github.com/danhale-git/mine/nbt"
	"github.com/danhale-git/nbt2json"
)

const subChunkBlockCount = 4096
const chunkSize = 16

// subChunkData is the parsed data for one 16x16 subchunk. A palette including all block states in the subchunk is indexed
// by a slice of integers (one for each block) to determine the state and block id for each block in the palette.
type subChunkData struct {
	Blocks      blockStorage
	WaterLogged blockStorage
}

type blockStorage struct {
	Indices []int        // An index into the palette for each block in the sub chunk
	Palette []nbt.NBTTag // A palette of block types and states
}

// subChunkOrigin returns the origin of the chunk containing the given coordinates. This is the corner block with the
// lowest x, y and z values.
func subChunkOrigin(x, y, z, d int) struct{ x, y, z, d int } {
	return struct{ x, y, z, d int }{
		int(math.Floor(float64(x) / 16)),
		int(math.Floor(float64(y) / 16)),
		int(math.Floor(float64(z) / 16)),
		d,
	}
}

// worldVoxelToSubChunk returns the coordinates relative to sub chunk origin, from the given world coordinates.
func worldVoxelToSubChunk(x, y, z int) (sx, sy, sz int) {
	return x % chunkSize, y % chunkSize, z % chunkSize
}

// voxelToIndex returns the block storage index from the given sub chunk x y and z coordinates.
func subChunkVoxelToIndex(x, y, z int) int {
	if x > 15 || y > 15 || z > 15 {
		log.Panicf("coordinates %d %d %d are invalid: sub chunk cooridnates may not exceed 0-15", x, y, z)
	}
	return y + z*16 + x*16*16
}

// indexToVoxel returns the world x y z offset from the sub chunk root for the given block storage index.
func subChunkIndexToVoxel(i int) (x, y, z int) {
	x = (i >> 8) & 15
	y = i & 15
	z = (i >> 4) & 15

	return
}

func parseSubChunk(data []byte) (*subChunkData, error) {
	r := bytes.NewReader(data)
	s := subChunkData{}

	var version int8
	if err := readLittleEndian(r, &version); err != nil {
		return nil, fmt.Errorf("reading version byte: %w", err)
	}

	var storageCount int8

	switch version {
	case 1:
		storageCount = 1
	case 8:
		if err := readLittleEndian(r, &storageCount); err != nil {
			return nil, fmt.Errorf("reading storage count: %w", err)
		}
	default:
		return nil, fmt.Errorf("unhandled subchunk block storage version: '%d'", version)
	}

	var err error

	s.Blocks.Indices, s.Blocks.Palette, err = parseBlockStorage(r)
	if err != nil {
		return nil, fmt.Errorf("parsing water logged: %s", err)
	}

	// https://minecraft.fandom.com/wiki/Bedrock_Edition_level_format
	// In the majority of cases, there is only one storage record.
	// A second record may be present to indicate block water-logging.
	switch storageCount {
	case 0:
		panic("block storage count is 0")
	case 1:
		// Block storage has already been parsed above
	case 2:
		// Parse second block storage as water logged if it exists
		s.WaterLogged.Indices, s.WaterLogged.Palette, err = parseBlockStorage(r)
		if err != nil {
			return nil, fmt.Errorf("parsing water logged: %s", err)
		}
		// Added some panicking here as the Minecraft level format seems changeable.

		if len(s.WaterLogged.Palette) > 2 {
			log.Panicf(`
second block storage palette exceeded known max length of 2
found these states - %+v`, s.WaterLogged.Palette)
		}
		if len(s.WaterLogged.Palette) > 1 && s.WaterLogged.Palette[1].BlockID() != waterID {
			log.Panicf(`
second block storage palette did not have '%s' at index 1 to indicate water logged blocks
found id '%s' unexpectedly`, waterID, s.WaterLogged.Palette[1].BlockID())
		}

	default:
		log.Panicf("unhandled storage count: %d", storageCount)
	}

	return &s, nil
}

func parseBlockStorage(r *bytes.Reader) ([]int, []nbt.NBTTag, error) {
	var indices []int
	var palette []nbt.NBTTag

	indices, err := stateIndices(r)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing water logged indices: %s", err)
	}

	palette, err = statePalette(r)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing nbt data: %s", err)
	}

	return indices, palette, nil
}

// stateIndices reads a single block storage record as the integer indices into the palette. It should be called
// the number of times returned by blockStorageCount, after calling blockStorageCount.
func stateIndices(r *bytes.Reader) ([]int, error) {
	var bitsPerBlockAndVersion byte
	if err := readLittleEndian(r, &bitsPerBlockAndVersion); err != nil {
		log.Fatalf("reading version byte: %s", err)
	}

	bitsPerBlock := int(bitsPerBlockAndVersion >> 1)

	storageVersion := int(bitsPerBlockAndVersion & 1)
	if storageVersion != 0 {
		return nil, fmt.Errorf("invalid block storage version %d: 0 is expected for save files", storageVersion)
	}

	blocksPerWord := int(math.Floor(32.0 / float64(bitsPerBlock)))
	wordCount := int(math.Ceil(subChunkBlockCount / float64(blocksPerWord)))

	indices := make([]int, subChunkBlockCount)

	i := 0

	for w := 0; w < wordCount; w++ {
		var word int32
		if err := readLittleEndian(r, &word); err != nil {
			return nil, fmt.Errorf("reading word %d from raw data: %s", w, err)
		}

		for b := 0; b < blocksPerWord && i < subChunkBlockCount; b++ {
			indices[i] = int((word >> ((i % blocksPerWord) * bitsPerBlock)) & ((1 << bitsPerBlock) - 1))
			i++
		}
	}

	return indices, nil
}

// statePalette reads the remainder of a subchunk record and returns a slice of tags. It should be called after blockStorageCount and
// the resulting call(s) to stateIndices.
func statePalette(r *bytes.Reader) ([]nbt.NBTTag, error) {
	var paletteSize int32
	if err := readLittleEndian(r, &paletteSize); err != nil {
		return nil, fmt.Errorf("reading palette size bytes: %w", err)
	}

	j, err := nbt2json.ReadNbt2Json(r, "", int(paletteSize))
	if err != nil {
		return nil, fmt.Errorf("calling nbt2json, %w", err)
	}

	nbtData := struct {
		NBT []nbt.NBTTag
	}{}
	if err := json.Unmarshal(j, &nbtData); err != nil {
		return nil, fmt.Errorf("unmarshaling json, %w", err)
	}

	if len(nbtData.NBT) != int(paletteSize) {
		return nil, fmt.Errorf("%d nbt records returned for palette size of %d", len(nbtData.NBT), paletteSize)
	}

	return nbtData.NBT, nil
}

func readLittleEndian(r io.Reader, data interface{}) error {
	return binary.Read(r, binary.ByteOrder(binary.LittleEndian), data)
}
