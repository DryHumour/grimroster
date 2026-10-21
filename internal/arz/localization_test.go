package arz

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	arzlib "codeberg.org/alex-ilchukov/arz/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeLocalizationARCReadsTagsFromTextEntries(t *testing.T) {
	t.Parallel()

	archive := makeLocalizationARC("text/factions.txt", []byte("tagFactionUser2=Homestead\notherTag=Other faction\n"))
	got, err := decodeLocalizationARC(archive)
	require.NoError(t, err)
	assert.Equal(t, "Homestead", got["tagFactionUser2"])
	assert.Equal(t, "Other faction", got["otherTag"])
}

func TestDecodeLocalizationARCRejectsUnsupportedHeaders(t *testing.T) {
	t.Parallel()

	_, err := decodeLocalizationARC([]byte("bad"))
	assert.ErrorContains(t, err, "decode ARC archive")

	archive := makeLocalizationARC("text/factions.txt", []byte("tagFactionUser2=Homestead\n"))
	binary.LittleEndian.PutUint32(archive[4:8], 4)
	_, err = decodeLocalizationARC(archive)
	assert.ErrorContains(t, err, "version 4")
}

func TestOpenGameDirUsesExpansionFactionNames(t *testing.T) {
	t.Parallel()

	gameDir := t.TempDir()
	baseARZ := filepath.Join(gameDir, "database", "database.arz")
	baseARC := filepath.Join(gameDir, "resources", "Text_EN.arc")
	expansionARZ := filepath.Join(gameDir, "gdx1", "database", "GDX1.arz")
	expansionARC := filepath.Join(gameDir, "gdx1", "resources", "Text_EN.arc")
	for _, path := range []string{baseARZ, baseARC, expansionARZ, expansionARC} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	}
	require.NoError(t, os.WriteFile(baseARZ, arzlib.Encode([]arzlib.EncRec{{Name: "records/base.dbr"}}), 0o600))
	require.NoError(t, os.WriteFile(expansionARZ, arzlib.Encode([]arzlib.EncRec{{Name: "records/expansion.dbr"}}), 0o600))
	require.NoError(t, os.WriteFile(baseARC, makeLocalizationARC("text/factions.txt", []byte("tagFactionUser2=Base name\n")), 0o600))
	require.NoError(t, os.WriteFile(expansionARC, makeLocalizationARC("text/factions.txt", []byte("tagFactionUser2=Expansion name\n")), 0o600))

	database, err := OpenGameDir(gameDir)
	require.NoError(t, err)
	name, found := database.LocalizedText("tagFactionUser2")
	assert.True(t, found)
	assert.Equal(t, "Expansion name", name)
}

func TestFactionCatalogLocalizesNamesWithoutChangingFactionIDs(t *testing.T) {
	t.Parallel()

	database, err := New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/game/gamefactions.dbr",
			Ints: map[string][]int32{
				"factionValue1": {-20000},
				"factionValue2": {-8000},
				"factionValue3": {-1500},
				"factionValue4": {0},
				"factionValue5": {1500},
				"factionValue6": {5000},
				"factionValue7": {10000},
				"factionValue8": {25000},
			},
			Strings: map[string][]string{
				"factionUser8": {"records/controllers/factions/kymons.dbr"},
				"factionTag1":  {"tagFactionStateEnemy5"},
				"factionTag2":  {"tagFactionStateEnemy4"},
				"factionTag3":  {"tagFactionStateEnemy2"},
				"factionTag4":  {"tagFactionStateNeutral"},
				"factionTag5":  {"tagFactionStateFriend1"},
				"factionTag6":  {"tagFactionStateFriend2"},
				"factionTag7":  {"tagFactionStateFriend4"},
				"factionTag8":  {"tagFactionStateFriend5"},
			},
		},
		{
			Name: "records/controllers/factions/kymons.dbr",
			Strings: map[string][]string{
				"myFaction": {"User8"},
			},
		},
	}))
	require.NoError(t, err)
	database.localization["tagFactionUser8"] = "Kymon's Chosen"
	database.localization["tagFactionStateNeutral"] = "Neutral"

	catalog, err := database.FactionCatalog()
	require.NoError(t, err)
	faction := catalog.Definitions["User8"]
	assert.Equal(t, "User8", faction.ID)
	assert.Equal(t, "Kymon's Chosen", faction.Name)
	assert.Equal(t, "tagFactionUser8", faction.NameTag)
	assert.Equal(t, "Neutral", catalog.StandingThresholds[3].Name)
}

func makeLocalizationARC(filename string, contents []byte) []byte {
	const headerSize = 28
	const chunkHeaderSize = 12
	const fileHeaderSize = 44
	filenameBytes := []byte(filename)
	contentOffset := headerSize
	tablesOffset := contentOffset + len(contents)
	filenameOffset := tablesOffset + chunkHeaderSize
	fileHeaderOffset := filenameOffset + len(filenameBytes)
	result := make([]byte, fileHeaderOffset+fileHeaderSize)

	copy(result[0:4], []byte{'A', 'R', 'C', 0})
	binary.LittleEndian.PutUint32(result[4:8], 3)
	binary.LittleEndian.PutUint32(result[8:12], 1)
	binary.LittleEndian.PutUint32(result[12:16], 1)
	binary.LittleEndian.PutUint32(result[16:20], chunkHeaderSize)
	binary.LittleEndian.PutUint32(result[20:24], uint32(len(filenameBytes)))
	binary.LittleEndian.PutUint32(result[24:28], uint32(tablesOffset))
	copy(result[contentOffset:tablesOffset], contents)

	binary.LittleEndian.PutUint32(result[tablesOffset:tablesOffset+4], uint32(contentOffset))
	binary.LittleEndian.PutUint32(result[tablesOffset+4:tablesOffset+8], uint32(len(contents)))
	binary.LittleEndian.PutUint32(result[tablesOffset+8:tablesOffset+12], uint32(len(contents)))
	copy(result[filenameOffset:fileHeaderOffset], filenameBytes)
	binary.LittleEndian.PutUint32(result[fileHeaderOffset+28:fileHeaderOffset+32], 1)
	binary.LittleEndian.PutUint32(result[fileHeaderOffset+32:fileHeaderOffset+36], 0)
	binary.LittleEndian.PutUint32(result[fileHeaderOffset+36:fileHeaderOffset+40], uint32(len(filenameBytes)))
	binary.LittleEndian.PutUint32(result[fileHeaderOffset+40:fileHeaderOffset+44], 0)
	return result
}
