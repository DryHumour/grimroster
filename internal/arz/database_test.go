package arz_test

import (
	"os"
	"path/filepath"
	"testing"

	arzlib "codeberg.org/alex-ilchukov/arz/v2"
	"github.com/DryHumour/grimroster/internal/arz"
	"github.com/DryHumour/grimroster/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrichAddsNameTagsWithoutReplacingDisplayNames(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/items/gearhead01.dbr",
			Strings: map[string][]string{
				"itemNameTag": {"tagGearHead01"},
			},
		},
		{
			Name: "records/items/potion01.dbr",
			Strings: map[string][]string{
				"itemNameTag": {"tagPotion01"},
			},
		},
	}))
	require.NoError(t, err)

	input := character.Character{
		Inventory: character.Inventory{
			Equipment: []character.EquippedItem{{
				Slot: character.SlotHead,
				Item: character.Item{
					RecordID:    "records/items/gearhead01.dbr",
					DisplayName: "Localized helm",
				},
			}},
			Bags: []character.Bag{{
				Items: []character.BagItem{{
					Item: character.Item{RecordID: "records/items/potion01.dbr"},
				}},
			}},
		},
	}

	got, err := db.Enrich(input)
	require.NoError(t, err)
	assert.Equal(t, "tagGearHead01", got.Inventory.Equipment[0].Item.NameTag)
	assert.Equal(t, "Localized helm", got.Inventory.Equipment[0].Item.DisplayName)
	assert.Equal(t, "tagPotion01", got.Inventory.Bags[0].Items[0].Item.NameTag)
	assert.Empty(t, input.Inventory.Equipment[0].Item.NameTag)
	assert.Empty(t, input.Inventory.Bags[0].Items[0].Item.NameTag)
}

func TestEnrichLeavesUnknownAndExistingTagsAlone(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{{
		Name: "records/items/known.dbr",
		Strings: map[string][]string{
			"itemNameTag": {"tagFromDatabase"},
		},
	}}))
	require.NoError(t, err)

	input := character.Character{
		Inventory: character.Inventory{
			Equipment: []character.EquippedItem{
				{Item: character.Item{RecordID: "records/items/missing.dbr"}},
				{Item: character.Item{RecordID: "records/items/known.dbr", NameTag: "tagAlreadySet"}},
			},
			Bags: []character.Bag{},
		},
	}

	got, err := db.Enrich(input)
	require.NoError(t, err)
	assert.Empty(t, got.Inventory.Equipment[0].Item.NameTag)
	assert.Equal(t, "tagAlreadySet", got.Inventory.Equipment[1].Item.NameTag)
	assert.NotNil(t, got.Inventory.Bags)
}

func TestStringFieldRejectsUnexpectedFieldShape(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{{
		Name: "records/items/wrong-type.dbr",
		Ints: map[string][]int32{"itemNameTag": {1}},
		Tag:  "item",
	}, {
		Name: "records/items/multiple-values.dbr",
		Strings: map[string][]string{
			"itemNameTag": {"tagOne", "tagTwo"},
		},
		Tag: "item",
	}}))
	require.NoError(t, err)

	_, _, err = db.StringField("records/items/wrong-type.dbr", "itemNameTag")
	assert.ErrorContains(t, err, "is not a string")

	_, _, err = db.StringField("records/items/multiple-values.dbr", "itemNameTag")
	assert.ErrorContains(t, err, "expected one")
}

func TestStringFieldsWithPrefixAndRecordIDsWithPrefix(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/skills/devotion/tier1_b.dbr",
			Strings: map[string][]string{
				"skillName": {"records/skills/devotion/tier1_b.dbr"},
			},
		},
		{
			Name: "records/skills/devotion/tier1_a.dbr",
			Strings: map[string][]string{
				"skillName": {"records/skills/devotion/tier1_a.dbr"},
			},
		},
	}))
	require.NoError(t, err)

	fields, err := db.StringFieldsWithPrefix("records/skills/devotion/tier1_a.dbr", "skill")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"skillName": "records/skills/devotion/tier1_a.dbr",
	}, fields)
	assert.Equal(t, []string{
		"records/skills/devotion/tier1_a.dbr",
		"records/skills/devotion/tier1_b.dbr",
	}, db.RecordIDsWithPrefix("records/skills/devotion/"))
}

func TestStringValuesFieldSupportsMultipleValues(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{{
		Name: "records/skills/itemskillsgdx3/example.dbr",
		Strings: map[string][]string{
			"skillUpBitmapName": {"ui/skill/one.tex", "ui/skill/two.tex"},
		},
	}}))
	require.NoError(t, err)

	values, found, err := db.StringValuesField("records/skills/itemskillsgdx3/example.dbr", "skillUpBitmapName")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []string{"ui/skill/one.tex", "ui/skill/two.tex"}, values)
}

func TestDevotionConstellationsJoinsBackgroundAndButtonRecords(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/ui/skills/devotion/constellations/constellation001.dbr",
			Strings: map[string][]string{
				"constellationDisplayTag": {"tagConstellation"},
				"constellationBackground": {"records/ui/skills/devotion/backgrounds/constellation001.dbr"},
				"devotionButton1":         {"records/ui/skills/devotion/constellations/button001.dbr"},
			},
		},
		{
			Name: "records/ui/skills/devotion/constellations/constellation002.dbr",
			Strings: map[string][]string{
				"constellationDisplayTag": {"tagConstellation"},
				"devotionButton2":         {"records/ui/skills/devotion/constellations/button002.dbr"},
			},
		},
		{
			Name: "records/ui/skills/devotion/backgrounds/constellation001.dbr",
			Strings: map[string][]string{
				"bitmapName": {"ui/skills/devotion/constellation001.tex"},
			},
		},
		{
			Name: "records/ui/skills/devotion/constellations/button001.dbr",
			Strings: map[string][]string{
				"skillName": {"records/skills/devotion/tier1_01a.dbr"},
			},
		},
		{
			Name: "records/ui/skills/devotion/constellations/button002.dbr",
			Strings: map[string][]string{
				"skillName": {"records/skills/devotion/tier1_01b.dbr"},
			},
		},
	}))
	require.NoError(t, err)

	constellations, err := db.DevotionConstellations()
	require.NoError(t, err)
	require.Len(t, constellations, 1)
	assert.Equal(t, "tagConstellation", constellations[0].DisplayTag)
	assert.Equal(t, "ui/skills/devotion/constellation001.tex", constellations[0].Bitmap)
	assert.Equal(t, map[uint32][]string{
		1: {"records/skills/devotion/tier1_01a.dbr"},
		2: {"records/skills/devotion/tier1_01b.dbr"},
	}, constellations[0].Stars)
}

func TestOpenGameDirUsesLaterExpansionRecords(t *testing.T) {
	t.Parallel()

	gameDir := t.TempDir()
	basePath := filepath.Join(gameDir, "database", "database.arz")
	expansionPath := filepath.Join(gameDir, "gdx1", "database", "GDX1.arz")
	require.NoError(t, os.MkdirAll(filepath.Dir(basePath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(expansionPath), 0o755))

	base := arzlib.Encode([]arzlib.EncRec{{
		Name: "records/items/shared.dbr",
		Strings: map[string][]string{
			"itemNameTag": {"tagBase"},
		},
	}})
	expansion := arzlib.Encode([]arzlib.EncRec{{
		Name: "records/items/shared.dbr",
		Strings: map[string][]string{
			"itemNameTag": {"tagExpansion"},
		},
	}})
	require.NoError(t, os.WriteFile(basePath, base, 0o600))
	require.NoError(t, os.WriteFile(expansionPath, expansion, 0o600))

	db, err := arz.OpenGameDir(gameDir)
	require.NoError(t, err)

	tag, found, err := db.StringField("records/items/shared.dbr", "itemNameTag")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "tagExpansion", tag)
}

func TestOpenWrapsFileErrors(t *testing.T) {
	t.Parallel()

	_, err := arz.Open(filepath.Join(t.TempDir(), "missing.arz"))
	assert.ErrorContains(t, err, "read ARZ database")
}
