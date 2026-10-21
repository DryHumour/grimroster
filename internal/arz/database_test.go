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

func TestInt32FieldValidatesFieldShape(t *testing.T) {
	t.Parallel()

	db, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/test/valid.dbr",
			Ints: map[string][]int32{"affinityRequired1": {6}},
		},
		{
			Name:    "records/test/wrong-type.dbr",
			Strings: map[string][]string{"affinityRequired1": {"six"}},
		},
		{
			Name: "records/test/multiple-values.dbr",
			Ints: map[string][]int32{"affinityRequired1": {1, 2}},
		},
	}))
	require.NoError(t, err)

	points, found, err := db.Int32Field("records/test/valid.dbr", "affinityRequired1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, int32(6), points)

	_, _, err = db.Int32Field("records/test/wrong-type.dbr", "affinityRequired1")
	assert.ErrorContains(t, err, "is not an int32")
	_, _, err = db.Int32Field("records/test/multiple-values.dbr", "affinityRequired1")
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
			Ints: map[string][]int32{
				"affinityRequired1": {1},
				"affinityRequired2": {0},
				"affinityRequired3": {0},
				"affinityGiven1":    {3},
				"affinityGiven2":    {2},
				"affinityGiven3":    {0},
			},
			Strings: map[string][]string{
				"constellationDisplayTag": {"tagConstellation"},
				"constellationBackground": {"records/ui/skills/devotion/backgrounds/constellation001.dbr"},
				"devotionButton1":         {"records/ui/skills/devotion/constellations/button001.dbr"},
				"affinityRequiredName1":   {"Chaos"},
				"affinityGivenName1":      {"Ascendant"},
				"affinityGivenName2":      {"Eldritch"},
			},
		},
		{
			Name: "records/ui/skills/devotion/constellations/constellation002.dbr",
			Ints: map[string][]int32{
				"affinityRequired1": {1},
				"affinityRequired2": {0},
				"affinityRequired3": {0},
				"affinityGiven1":    {0},
				"affinityGiven2":    {0},
				"affinityGiven3":    {0},
			},
			Strings: map[string][]string{
				"constellationDisplayTag": {"tagConstellation"},
				"devotionButton2":         {"records/ui/skills/devotion/constellations/button002.dbr"},
				"affinityRequiredName1":   {"Chaos"},
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
	assert.True(t, constellations[0].AffinityRequirements.AllKnown())
	assert.Equal(t, uint32(1), constellations[0].AffinityRequirements.Chaos.Points)
	assert.True(t, constellations[0].AffinityRewards.AllKnown())
	assert.Equal(t, uint32(3), constellations[0].AffinityRewards.Ascendant.Points)
	assert.Equal(t, uint32(2), constellations[0].AffinityRewards.Eldritch.Points)
	assert.Equal(t, arz.AffinityRewardModePerStar, constellations[0].AffinityRewardMode)
	assert.Equal(t, 2, constellations[0].StarAffinityRecordCount)
	assert.True(t, constellations[0].StarAffinityRewardsKnown)
	assert.Len(t, constellations[0].StarAffinityRewardsByRecordID, 2)
	assert.Equal(t, map[uint32][]string{
		1: {"records/skills/devotion/tier1_01a.dbr"},
		2: {"records/skills/devotion/tier1_01b.dbr"},
	}, constellations[0].Stars)
}

func TestFactionCatalogUsesStableARZIdentitiesAndThresholdTags(t *testing.T) {
	t.Parallel()

	gameFactions := arzlib.EncRec{
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
			"factionUser8":               {"records/controllers/factions/kymons.dbr"},
			"factionAetherials":          {"records/controllers/factions/aetherials.dbr"},
			"factionTag1":                {"tagFactionStateEnemy5"},
			"factionTag2":                {"tagFactionStateEnemy4"},
			"factionTag3":                {"tagFactionStateEnemy2"},
			"factionTag4":                {"tagFactionStateNeutral"},
			"factionTag5":                {"tagFactionStateFriend1"},
			"factionTag6":                {"tagFactionStateFriend2"},
			"factionTag7":                {"tagFactionStateFriend4"},
			"factionTag8":                {"tagFactionStateFriend5"},
			"factionAlternateNeutralTag": {"tagFactionStateEnemy1"},
		},
	}
	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		gameFactions,
		{
			Name:    "records/controllers/factions/kymons.dbr",
			Strings: map[string][]string{"myFaction": {"User8"}},
		},
		{
			Name:    "records/controllers/factions/aetherials.dbr",
			Strings: map[string][]string{"myFaction": {"Aetherials"}},
		},
	}))
	require.NoError(t, err)

	catalog, err := database.FactionCatalog()
	require.NoError(t, err)
	assert.True(t, catalog.StandingThresholdsKnown)
	assert.True(t, catalog.AlternateNeutralKnown)
	assert.Equal(t, "enemy1", catalog.AlternateNeutralTier)
	assert.Equal(t, "tagFactionStateEnemy1", catalog.AlternateNeutralTag)
	require.Len(t, catalog.StandingThresholds, 8)
	assert.Equal(t, float32(-8000), catalog.StandingThresholds[1].Minimum)
	assert.Equal(t, "enemy4", catalog.StandingThresholds[1].Tier)
	assert.Equal(t, "tagFactionStateEnemy4", catalog.StandingThresholds[1].Tag)
	assert.Equal(t, "Aetherials", catalog.Definitions["Aetherials"].ID)
	assert.Equal(t, "records/controllers/factions/aetherials.dbr", catalog.Definitions["Aetherials"].RecordID)
	assert.Equal(t, "User8", catalog.Definitions["User8"].ID)
	assert.Equal(t, "tagFactionUser8", catalog.Definitions["User8"].NameTag)
	assert.Equal(t, "records/controllers/factions/kymons.dbr", catalog.Definitions["User8"].RecordID)
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
