package subchunk

import (
	"bytes"
	"testing"

	"github.com/danhale-git/mine/mock"
)

func mockBlockStorage() *blockStorage {
	statePalette := mock.StatePaletteIDs()

	states := make([]BlockState, len(statePalette))
	for i, s := range states {
		s.Value = map[string]interface{}{
			"name":  "name",
			"value": statePalette[i],
		}
	}

	return &blockStorage{
		Indices: mock.BlockStateIndices,
		Palette: states,
	}
}

func TestWriteStateIndices(t *testing.T) {
	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	storage := mockBlockStorage()

	if err := writeStateIndices(buf, storage); err != nil {
		t.Errorf("unexpected error returned: %s", err)
	}

	/*indices, err := readStateIndices(buf)
	if err != nil {
		t.Errorf("unexpected error returned: %s", err)
	}

	palette := mock.StatePaletteIDs()

	expected := mock.BlockStateIndices

	for i, stateIndex := range indices {
		if stateIndex >= len(palette) {
			t.Fatalf("block state index %d is out of range of state palette with length %d", stateIndex, len(palette))
		}

		if stateIndex != expected[i] {
			t.Fatalf("expected palette index '%d' but got '%d'", expected[i], stateIndex)
		}
	}

	if len(indices) != BlockCount {
		t.Errorf("expected %d blocks state indices: got %d", BlockCount, len(indices))
	}*/
}
