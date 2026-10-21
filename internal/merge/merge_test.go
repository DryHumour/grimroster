package merge

import (
	"encoding/json/jsontext"
	"os"
	"path/filepath"
	"testing"

	arzlib "codeberg.org/alex-ilchukov/arz/v2"
	"github.com/DryHumour/grimroster/internal/arz"
	"github.com/DryHumour/grimroster/internal/character"
	"github.com/DryHumour/grimroster/internal/gdedit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFilesLoadsAndMergesBothSources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	grimPath := filepath.Join(dir, "grim.json")
	htmlPath := filepath.Join(dir, "gd-edit.html")
	require.NoError(t, os.WriteFile(grimPath, []byte(`{"Header":{"Name":"Shared","Level":10},"Block1":{"DecodingError":"unable to decode"},"Block2":{},"Block3":{},"Block8":{"Skills":[]}}`), 0o600))
	require.NoError(t, os.WriteFile(htmlPath, []byte(`<script>window.DATA={"sheet":{"character":{"name":"Shared","level":10},"stats":{"health":100}}};</script>`), 0o600))

	got, err := ReadFiles(grimPath, htmlPath)
	require.NoError(t, err)
	assert.Equal(t, "Shared", got.Name)
	require.NotNil(t, got.Calculated)
	assert.Equal(t, float64(100), got.Calculated.Health)
	require.Len(t, got.Diagnostics, 1)
	assert.Equal(t, character.DiagnosticCodeGrimBlockDecodingError, got.Diagnostics[0].Code)
}

func TestMergeKeepsGrimValuesAndRecordsDisagreements(t *testing.T) {
	t.Parallel()

	grim := character.Character{
		Name:  "Grim Name",
		Level: 50,
		Inventory: character.Inventory{
			Equipment: []character.EquippedItem{{
				Slot: character.SlotHead,
				Item: character.Item{RecordID: "records/items/helm.dbr", Seed: 42},
			}},
			Bags: []character.Bag{},
		},
	}
	sheet := gdedit.Data{
		Name:       "gd-edit Name",
		Level:      51,
		Masteries:  []string{"Soldier", "Arcanist"},
		Difficulty: "Ultimate",
		Expansion:  "Ashes of Malmouth",
		Calculated: character.Calculated{
			Health:           9000,
			DefensiveAbility: 1400,
			Armor:            500,
			ArmorBySlot:      map[string]float64{"Head": 500},
			Resistances:      []character.Resistance{{Field: "defensiveFire", Label: "Fire", Shown: 80, Cap: 80}},
		},
		Equipment: []gdedit.Equipment{
			{Slot: "Head", Name: "Localized Helm", Seed: 42, Raw: jsontext.Value(`{"slot":"Head","name":"Localized Helm","seed":42}`)},
			{Slot: "Medal", Name: "Unmatched Medal", Seed: 99, Raw: jsontext.Value(`{"slot":"Medal","name":"Unmatched Medal","seed":99}`)},
		},
	}

	got, err := Merge(grim, sheet)
	require.NoError(t, err)
	assert.Equal(t, "Grim Name", got.Name)
	assert.Equal(t, uint32(50), got.Level)
	assert.Equal(t, "Soldier + Arcanist", got.ClassName)
	assert.Equal(t, []string{"Soldier", "Arcanist"}, got.Masteries)
	assert.Equal(t, "Ultimate", got.Difficulty)
	assert.Equal(t, "Ashes of Malmouth", got.Expansion)
	require.NotNil(t, got.Calculated)
	assert.Equal(t, float64(9000), got.Calculated.Health)
	assert.Equal(t, float64(1400), got.Calculated.DefensiveAbility)
	assert.Equal(t, float64(80), got.Calculated.Resistances[0].Shown)
	require.Len(t, got.Inventory.Equipment, 1)
	assert.Equal(t, "records/items/helm.dbr", got.Inventory.Equipment[0].Item.RecordID)
	assert.Equal(t, "Localized Helm", got.Inventory.Equipment[0].Item.DisplayName)
	assert.JSONEq(t, `{"slot":"Head","name":"Localized Helm","seed":42}`, string(got.Inventory.Equipment[0].Item.GDEditDetails))
	require.Len(t, got.Diagnostics, 3)
	assert.Equal(t, []string{
		"name",
		"level",
		`inventory.equipment[slot="Medal",seed=99]`,
	}, []string{got.Diagnostics[0].Field, got.Diagnostics[1].Field, got.Diagnostics[2].Field})
	assert.Equal(t, character.SeverityError, got.Diagnostics[0].Severity)
	assert.Equal(t, character.DiagnosticCodeCharacterNameMismatch, got.Diagnostics[0].Code)
	assert.Equal(t, "Character name differs between grim and gd-edit.", got.Diagnostics[0].Message)
	assert.Equal(t, character.SeverityError, got.Diagnostics[1].Severity)
	assert.Equal(t, character.DiagnosticCodeCharacterLevelMismatch, got.Diagnostics[1].Code)
	assert.Equal(t, character.DiagnosticCodeEquipmentUnmatched, got.Diagnostics[2].Code)
	assert.Equal(t, "gd-edit equipment item did not match a grim equipment item.", got.Diagnostics[2].Message)
	assert.JSONEq(t, `"Grim Name"`, string(got.Diagnostics[0].GrimValue))
	assert.JSONEq(t, `"gd-edit Name"`, string(got.Diagnostics[0].GDEditValue))
	assert.JSONEq(t, `50`, string(got.Diagnostics[1].GrimValue))
	assert.JSONEq(t, `51`, string(got.Diagnostics[1].GDEditValue))
	assert.JSONEq(t, `null`, string(got.Diagnostics[2].GrimValue))
	assert.JSONEq(t, `{"slot":"Medal","name":"Unmatched Medal","seed":99}`, string(got.Diagnostics[2].GDEditValue))

	assert.Empty(t, grim.Diagnostics)
	assert.Empty(t, grim.Inventory.Equipment[0].Item.DisplayName)
	assert.Nil(t, grim.Calculated)
}

func TestMergeMatchesUniqueSlotWhenSeedIsMissing(t *testing.T) {
	t.Parallel()

	grim := character.Character{
		Inventory: character.Inventory{
			Equipment: []character.EquippedItem{{
				Slot: character.SlotTorso,
				Item: character.Item{RecordID: "records/items/chest.dbr"},
			}},
		},
	}
	sheet := gdedit.Data{
		Equipment: []gdedit.Equipment{{
			Slot: "Chest",
			Name: "Localized Chest",
			Raw:  jsontext.Value(`{"slot":"Chest","name":"Localized Chest"}`),
		}},
	}

	got, err := Merge(grim, sheet)
	require.NoError(t, err)
	require.Len(t, got.Inventory.Equipment, 1)
	assert.Equal(t, "Localized Chest", got.Inventory.Equipment[0].Item.DisplayName)
	assert.Empty(t, got.Diagnostics)
}

func TestMergeEnrichesSkillsAndDevotionBindingsFromARZ(t *testing.T) {
	t.Parallel()

	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/skills/playerclass01/cadence.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {"ui/skills/icons/cadence.tex"},
			},
		},
		{
			Name: "records/skills/devotion/tier1_01a.dbr",
		},
		{
			Name: "records/ui/skills/devotion/constellations/constellation001.dbr",
			Strings: map[string][]string{
				"constellationDisplayTag": {"tagConstellation"},
				"constellationBackground": {"records/ui/skills/devotion/backgrounds/constellation001.dbr"},
				"devotionButton1":         {"records/ui/skills/devotion/constellations/button001.dbr"},
			},
		},
		{
			Name: "records/ui/skills/devotion/backgrounds/constellation001.dbr",
			Strings: map[string][]string{
				"bitmapName": {"ui/skills/devotion/fiend.tex"},
			},
		},
		{
			Name: "records/ui/skills/devotion/constellations/button001.dbr",
			Strings: map[string][]string{
				"skillName": {"records/skills/devotion/tier1_01a.dbr"},
			},
		},
	}))
	require.NoError(t, err)

	grim := character.Character{
		Skills: []character.Skill{
			{
				RecordID:         "records/skills/playerclass01/cadence.dbr",
				Level:            5,
				Enabled:          true,
				AutocastRecordID: "records/skills/devotion/tier1_01a.dbr",
			},
			{
				RecordID:      "records/skills/devotion/tier1_01a.dbr",
				Level:         1,
				DevotionLevel: 1,
				Enabled:       true,
				IsDevotion:    true,
			},
		},
	}
	sheet := gdedit.Data{
		Skills: []gdedit.SkillObservation{{
			Mastery:  "Soldier",
			Name:     "Cadence",
			Level:    4,
			MaxLevel: 16,
			JoinKey:  "ui/skills/icons/cadence.tex",
		}},
		Devotions: []gdedit.Devotion{{
			Name:       "Fiend",
			JoinKey:    "ui/skills/devotion/fiend.tex",
			StarsTaken: 1,
			StarsTotal: 1,
			Complete:   true,
			Stars: []gdedit.DevotionStar{{
				Name:    "Flame Torrent",
				Button:  1,
				Skill:   true,
				Taken:   false,
				Bound:   "Cadence",
				Trigger: "on Attack",
			}},
		}},
	}

	got, err := MergeWithDatabase(grim, sheet, database)
	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Equal(t, "Cadence", got.Skills[0].Name)
	assert.Equal(t, "Soldier", got.Skills[0].Mastery)
	assert.Equal(t, uint32(4), got.Skills[0].GDEditLevel)
	require.Len(t, got.Diagnostics, 2)
	assert.Equal(t, character.DiagnosticCodeSkillLevelMismatch, got.Diagnostics[0].Code)
	assert.Equal(t, character.DiagnosticCodeDevotionStateMismatch, got.Diagnostics[1].Code)
	require.Len(t, got.Devotions, 1)
	assert.Equal(t, "Fiend", got.Devotions[0].Name)
	assert.True(t, got.Devotions[0].Complete)
	assert.Equal(t, uint32(1), got.Devotions[0].StarsTaken)
	require.Len(t, got.Devotions[0].Stars, 1)
	assert.Equal(t, "records/skills/devotion/tier1_01a.dbr", got.Devotions[0].Stars[0].RecordID)
	assert.Equal(t, "records/skills/playerclass01/cadence.dbr", got.Devotions[0].Stars[0].BoundSkillRecordID)
	assert.Equal(t, "Cadence", got.Devotions[0].Stars[0].BoundSkill)
}

func TestMergeWarnsForUnmatchedInvestedSkillsButNotUninvestedNodes(t *testing.T) {
	t.Parallel()

	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{{
		Name: "records/skills/playerclass01/known.dbr",
		Strings: map[string][]string{
			"skillUpBitmapName": {"ui/skills/icons/known.tex"},
		},
	}}))
	require.NoError(t, err)

	grim := character.Character{
		Skills: []character.Skill{{
			RecordID: "records/skills/playerclass01/known.dbr",
			Level:    2,
		}},
	}
	sheet := gdedit.Data{Skills: []gdedit.SkillObservation{
		{Mastery: "Soldier", Name: "Known", Level: 2, JoinKey: "ui/skills/icons/known.tex"},
		{Mastery: "Soldier", Name: "Unknown", Level: 1, JoinKey: "ui/skills/icons/missing.tex"},
		{Mastery: "Soldier", Name: "Empty node", Level: 0, JoinKey: "ui/skills/icons/empty.tex"},
	}}

	got, err := MergeWithDatabase(grim, sheet, database)
	require.NoError(t, err)
	require.Len(t, got.Skills, 3)
	assert.False(t, got.Skills[0].GDEditOnly)
	assert.True(t, got.Skills[1].GDEditOnly)
	assert.True(t, got.Skills[2].GDEditOnly)
	require.Len(t, got.Diagnostics, 1)
	assert.Equal(t, character.SeverityWarning, got.Diagnostics[0].Severity)
	assert.Equal(t, character.DiagnosticCodeSkillUnmatchedInvested, got.Diagnostics[0].Code)
	assert.Equal(t, `skills["Soldier":"Unknown"].level`, got.Diagnostics[0].Field)
	assert.JSONEq(t, `null`, string(got.Diagnostics[0].GrimValue))
	assert.JSONEq(t, `1`, string(got.Diagnostics[0].GDEditValue))
}

func TestMergeLeavesAmbiguousInvestedSkillUncorrelated(t *testing.T) {
	t.Parallel()

	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/skills/playerclass01/first.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {"ui/skills/icons/shared.tex"},
			},
		},
		{
			Name: "records/skills/playerclass01/second.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {"ui/skills/icons/shared.tex"},
			},
		},
	}))
	require.NoError(t, err)

	got, err := MergeWithDatabase(character.Character{
		Skills: []character.Skill{
			{RecordID: "records/skills/playerclass01/first.dbr", Level: 1},
			{RecordID: "records/skills/playerclass01/second.dbr", Level: 1},
		},
	}, gdedit.Data{Skills: []gdedit.SkillObservation{{
		Mastery: "Soldier",
		Name:    "Ambiguous",
		Level:   1,
		JoinKey: "ui/skills/icons/shared.tex",
	}}}, database)
	require.NoError(t, err)
	require.Len(t, got.Skills, 3)
	assert.Equal(t, "records/skills/playerclass01/first.dbr", got.Skills[0].RecordID)
	assert.Equal(t, "records/skills/playerclass01/second.dbr", got.Skills[1].RecordID)
	assert.True(t, got.Skills[2].GDEditOnly)
	assert.Empty(t, got.Skills[2].RecordID)
	require.Len(t, got.Diagnostics, 1)
	assert.Equal(t, character.DiagnosticCodeSkillUnmatchedInvested, got.Diagnostics[0].Code)
}

func TestMergeEnrichesEquippedAndBagItemsFromARZ(t *testing.T) {
	t.Parallel()

	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/items/helm.dbr",
			Strings: map[string][]string{
				"itemNameTag": {"tagHelm"},
			},
		},
		{
			Name: "records/items/potion.dbr",
			Strings: map[string][]string{
				"itemNameTag": {"tagPotion"},
			},
		},
	}))
	require.NoError(t, err)

	grim := character.Character{
		Inventory: character.Inventory{
			Equipment: []character.EquippedItem{{
				Slot: character.SlotHead,
				Item: character.Item{
					RecordID:        "records/items/helm.dbr",
					Seed:            42,
					DisplayName:     "Localized helm",
					PrefixID:        "records/affixes/prefix.dbr",
					SuffixID:        "records/affixes/suffix.dbr",
					CraftingBonusID: "records/affixes/crafting.dbr",
					IllusionID:      "records/illusions/illusion.dbr",
					ComponentID:     "records/components/component.dbr",
					RelicBonusID:    "records/affixes/relic_bonus.dbr",
					AugmentID:       "records/augments/augment.dbr",
					AscendedAffixID: "records/affixes/ascended.dbr",
				},
			}},
			Bags: []character.Bag{{
				Items: []character.BagItem{{
					X: 2,
					Y: 3,
					Item: character.Item{
						RecordID: "records/items/potion.dbr",
						Seed:     73,
						Amount:   5,
					},
				}},
			}},
		},
	}
	sheet := gdedit.Data{Equipment: []gdedit.Equipment{{
		Slot: "Head",
		Name: "Localized helm",
		Seed: 42,
		Raw:  jsontext.Value(`{"slot":"Head","name":"Localized helm","seed":42}`),
	}}}

	got, err := MergeWithDatabase(grim, sheet, database)
	require.NoError(t, err)
	assert.Equal(t, "tagHelm", got.Inventory.Equipment[0].Item.NameTag)
	assert.Equal(t, "records/items/helm.dbr", got.Inventory.Equipment[0].Item.RecordID)
	assert.Equal(t, "Localized helm", got.Inventory.Equipment[0].Item.DisplayName)
	assert.Equal(t, uint32(42), got.Inventory.Equipment[0].Item.Seed)
	assert.Equal(t, "records/affixes/prefix.dbr", got.Inventory.Equipment[0].Item.PrefixID)
	assert.Equal(t, "records/affixes/suffix.dbr", got.Inventory.Equipment[0].Item.SuffixID)
	assert.Equal(t, "records/affixes/crafting.dbr", got.Inventory.Equipment[0].Item.CraftingBonusID)
	assert.Equal(t, "records/illusions/illusion.dbr", got.Inventory.Equipment[0].Item.IllusionID)
	assert.Equal(t, "records/components/component.dbr", got.Inventory.Equipment[0].Item.ComponentID)
	assert.Equal(t, "records/affixes/relic_bonus.dbr", got.Inventory.Equipment[0].Item.RelicBonusID)
	assert.Equal(t, "records/augments/augment.dbr", got.Inventory.Equipment[0].Item.AugmentID)
	assert.Equal(t, "records/affixes/ascended.dbr", got.Inventory.Equipment[0].Item.AscendedAffixID)
	assert.JSONEq(t, `{"slot":"Head","name":"Localized helm","seed":42}`, string(got.Inventory.Equipment[0].Item.GDEditDetails))
	assert.Equal(t, "tagPotion", got.Inventory.Bags[0].Items[0].Item.NameTag)
	assert.Equal(t, uint32(73), got.Inventory.Bags[0].Items[0].Item.Seed)
	assert.Equal(t, uint32(5), got.Inventory.Bags[0].Items[0].Item.Amount)
	assert.Empty(t, grim.Inventory.Equipment[0].Item.NameTag)
	assert.Empty(t, grim.Inventory.Bags[0].Items[0].Item.NameTag)
}

func TestMergeKeepsNamedSheetSkillsAndDevotionsWithoutARZ(t *testing.T) {
	t.Parallel()

	got, err := Merge(character.Character{}, gdedit.Data{
		Skills: []gdedit.SkillObservation{{
			Mastery: "Soldier",
			Name:    "Cadence",
			Level:   5,
		}},
		Devotions: []gdedit.Devotion{{
			Name:       "Fiend",
			StarsTaken: 1,
			StarsTotal: 1,
			Complete:   true,
			Stars: []gdedit.DevotionStar{{
				Name:   "Flame Torrent",
				Button: 5,
				Skill:  true,
				Taken:  true,
				Bound:  "Cadence",
			}},
		}},
	})
	require.NoError(t, err)
	require.Len(t, got.Skills, 1)
	assert.True(t, got.Skills[0].GDEditOnly)
	assert.Equal(t, "Cadence", got.Skills[0].Name)
	require.Len(t, got.Devotions, 1)
	assert.Equal(t, "Fiend", got.Devotions[0].Name)
	assert.Equal(t, "Cadence", got.Devotions[0].Stars[0].BoundSkill)
	assert.Empty(t, got.Devotions[0].Stars[0].RecordID)
	assert.Empty(t, got.Diagnostics)
}
