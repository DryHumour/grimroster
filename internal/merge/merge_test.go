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

func TestMergeAffinityTotalsCountCrossroadsAndCompletedConstellations(t *testing.T) {
	t.Parallel()

	current := character.KnownZeroAffinityAmounts()
	current.Set(character.AffinityAscendant, 1)
	current.Set(character.AffinityEldritch, 3)

	crossroadsReward := character.KnownZeroAffinityAmounts()
	crossroadsReward.Set(character.AffinityAscendant, 1)
	constellationReward := character.KnownZeroAffinityAmounts()
	constellationReward.Set(character.AffinityEldritch, 3)
	partialReward := character.KnownZeroAffinityAmounts()
	partialReward.Set(character.AffinityChaos, 4)

	sheet := gdedit.Data{
		Affinities: current,
		Devotions: []gdedit.Devotion{
			{
				Name:                 "Crossroads - Ascendant",
				StarsTaken:           1,
				StarsTotal:           1,
				Complete:             true,
				AffinityRequirements: character.KnownZeroAffinityAmounts(),
				AffinityRewards:      crossroadsReward,
				Stars:                []gdedit.DevotionStar{{Taken: true}},
			},
			{
				Name:                 "Completed constellation",
				StarsTaken:           1,
				StarsTotal:           1,
				Complete:             true,
				AffinityRequirements: character.KnownZeroAffinityAmounts(),
				AffinityRewards:      constellationReward,
				Stars:                []gdedit.DevotionStar{{Taken: true}},
			},
			{
				Name:                 "Partial constellation",
				StarsTaken:           1,
				StarsTotal:           2,
				Complete:             false,
				AffinityRequirements: character.KnownZeroAffinityAmounts(),
				AffinityRewards:      partialReward,
				Stars:                []gdedit.DevotionStar{{Taken: true}, {Taken: false}},
			},
		},
	}

	got, err := Merge(character.Character{}, sheet)
	require.NoError(t, err)
	assert.Equal(t, character.AffinitySourceGDEdit, got.CurrentAffinitiesSource)
	assert.True(t, got.CurrentAffinities.AllKnown())
	assert.Equal(t, uint32(1), got.CurrentAffinities.Ascendant.Points)
	assert.Equal(t, uint32(3), got.CurrentAffinities.Eldritch.Points)
	require.Len(t, got.Devotions, 3)
	assert.True(t, got.Devotions[0].Complete)
	assert.Equal(t, uint32(1), got.Devotions[0].AffinityRewards.Ascendant.Points)
	assert.Equal(t, character.AffinitySourceGDEdit, got.Devotions[0].AffinityRewardsSource)
	assert.True(t, got.Devotions[1].Complete)
	assert.Equal(t, uint32(3), got.Devotions[1].AffinityRewards.Eldritch.Points)
	assert.False(t, got.Devotions[2].Complete)
	assert.Equal(t, uint32(4), got.Devotions[2].AffinityRewards.Chaos.Points)
	assert.Empty(t, got.Diagnostics)
}

func TestMergeWarnsWhenDirectAffinityTotalsConflictWithCompletedRewards(t *testing.T) {
	t.Parallel()

	current := character.KnownZeroAffinityAmounts()
	current.Set(character.AffinityEldritch, 2)
	rewards := character.KnownZeroAffinityAmounts()
	rewards.Set(character.AffinityEldritch, 3)
	sheet := gdedit.Data{
		Affinities: current,
		Devotions: []gdedit.Devotion{{
			Name:                 "Completed constellation",
			StarsTaken:           1,
			StarsTotal:           1,
			AffinityRequirements: character.KnownZeroAffinityAmounts(),
			AffinityRewards:      rewards,
			Stars:                []gdedit.DevotionStar{{Taken: true}},
		}},
	}

	got, err := Merge(character.Character{}, sheet)
	require.NoError(t, err)
	require.Len(t, got.Diagnostics, 1)
	assert.Equal(t, character.DiagnosticCodeDevotionAffinityTotalMismatch, got.Diagnostics[0].Code)
	assert.Equal(t, "current_affinities.eldritch", got.Diagnostics[0].Field)
	assert.JSONEq(t, `{"source":"completed_constellation_rewards","points":3}`, string(got.Diagnostics[0].GrimValue))
	assert.JSONEq(t, `{"source":"gd_edit.devo.affinities","points":2}`, string(got.Diagnostics[0].GDEditValue))
}

func TestMergeKeepsMissingAffinitySourcesUnknown(t *testing.T) {
	t.Parallel()

	got, err := Merge(character.Character{}, gdedit.Data{Devotions: []gdedit.Devotion{{
		Name:       "Partial constellation",
		StarsTotal: 2,
		Stars:      []gdedit.DevotionStar{{Taken: true}, {Taken: false}},
	}}})
	require.NoError(t, err)
	assert.Equal(t, character.AffinitySourceUnknown, got.CurrentAffinitiesSource)
	assert.False(t, got.CurrentAffinities.AllKnown())
	assert.False(t, got.CurrentAffinities.Ascendant.Known)
	require.Len(t, got.Devotions, 1)
	assert.Equal(t, character.AffinitySourceUnknown, got.Devotions[0].AffinityRequirementsSource)
	assert.Equal(t, character.AffinitySourceUnknown, got.Devotions[0].AffinityRewardsSource)
	assert.False(t, got.Devotions[0].AffinityRequirements.AllKnown())
	assert.False(t, got.Devotions[0].AffinityRewards.AllKnown())
	assert.Empty(t, got.Diagnostics)
}

func TestMergeUsesGDeditAffinityDataBeforeARZFallback(t *testing.T) {
	t.Parallel()

	const bitmap = "ui/skills/devotion/test_constellation.tex"
	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/ui/skills/devotion/constellations/constellation007.dbr",
			Ints: map[string][]int32{
				"affinityRequired1": {5},
				"affinityRequired2": {0},
				"affinityRequired3": {0},
				"affinityGiven1":    {2},
				"affinityGiven2":    {0},
				"affinityGiven3":    {0},
			},
			Strings: map[string][]string{
				"constellationDisplayTag": {"tagTestConstellation"},
				"constellationBackground": {"records/ui/skills/devotion/backgrounds/test.dbr"},
				"affinityRequiredName1":   {"Order"},
				"affinityGivenName1":      {"Primordial"},
			},
		},
		{
			Name: "records/ui/skills/devotion/backgrounds/test.dbr",
			Strings: map[string][]string{
				"bitmapName": {bitmap},
			},
		},
	}))
	require.NoError(t, err)

	observation := gdedit.Devotion{
		Name:       "Test constellation",
		JoinKey:    bitmap,
		StarsTaken: 1,
		StarsTotal: 1,
		Complete:   true,
		Stars:      []gdedit.DevotionStar{{Taken: true}},
	}
	got, err := MergeWithDatabase(character.Character{}, gdedit.Data{
		Devotions: []gdedit.Devotion{observation},
	}, database)
	require.NoError(t, err)
	require.Len(t, got.Devotions, 1)
	assert.Equal(t, character.AffinitySourceARZ, got.Devotions[0].AffinityRequirementsSource)
	assert.Equal(t, uint32(5), got.Devotions[0].AffinityRequirements.Order.Points)
	assert.Equal(t, character.AffinitySourceARZ, got.Devotions[0].AffinityRewardsSource)
	assert.Equal(t, uint32(2), got.Devotions[0].AffinityRewards.Primordial.Points)

	htmlRequirements := character.KnownZeroAffinityAmounts()
	htmlRequirements.Set(character.AffinityChaos, 7)
	htmlRewards := character.KnownZeroAffinityAmounts()
	htmlRewards.Set(character.AffinityChaos, 3)
	observation.AffinityRequirements = htmlRequirements
	observation.AffinityRewards = htmlRewards
	got, err = MergeWithDatabase(character.Character{}, gdedit.Data{
		Devotions: []gdedit.Devotion{observation},
	}, database)
	require.NoError(t, err)
	require.Len(t, got.Devotions, 1)
	assert.Equal(t, character.AffinitySourceGDEdit, got.Devotions[0].AffinityRequirementsSource)
	assert.Equal(t, uint32(7), got.Devotions[0].AffinityRequirements.Chaos.Points)
	assert.Equal(t, character.AffinitySourceGDEdit, got.Devotions[0].AffinityRewardsSource)
	assert.Equal(t, uint32(3), got.Devotions[0].AffinityRewards.Chaos.Points)
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
	assert.Equal(t, character.SkillMatchStatusUnmatchedInvested, got.Skills[1].MatchStatus)
	assert.Equal(t, character.SkillMatchStatusUnusedHTMLNode, got.Skills[2].MatchStatus)
	require.Len(t, got.Diagnostics, 1)
	assert.Equal(t, character.SeverityWarning, got.Diagnostics[0].Severity)
	assert.Equal(t, character.DiagnosticCodeSkillUnmatchedInvested, got.Diagnostics[0].Code)
	assert.Equal(t, `skills["Soldier":"Unknown"].level`, got.Diagnostics[0].Field)
	assert.JSONEq(t, `null`, string(got.Diagnostics[0].GrimValue))
	assert.JSONEq(t, `1`, string(got.Diagnostics[0].GDEditValue))
}

func TestMergeClassifiesSaveSkillsAndResolvesMasteryBars(t *testing.T) {
	t.Parallel()

	const (
		demolitionistSkill = "records/skills/playerclass02/flametouched1.dbr"
		inquisitorSkill    = "records/skills/playerclass07/inquisitorseal1.dbr"
		defaultPotion      = "records/skills/default/defaulthealthpotion.dbr"
		potionModifier     = "records/skills/itemskillsgdx3/potionmodifiers/healthpotion_healovertime.dbr"
		potionContainer    = "records/skills/itemskillsgdx3/potionmodifiers/container_healthpotion_flask.dbr"
	)
	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: demolitionistSkill,
			Strings: map[string][]string{
				"skillUpBitmapName": {"ui/skills/icons/flametouched.tex"},
			},
		},
		{
			Name: inquisitorSkill,
			Strings: map[string][]string{
				"skillUpBitmapName": {"ui/skills/icons/inquisitorseal.tex"},
			},
		},
		{
			Name: defaultPotion,
			Strings: map[string][]string{
				"skillDisplayName": {"tagDefaultHealthPotion"},
			},
		},
		{
			Name: potionModifier,
			Strings: map[string][]string{
				"skillDisplayName": {"tagHealthPotionHealOverTime"},
			},
		},
		{Name: potionContainer},
	}))
	require.NoError(t, err)

	grim := character.Character{Skills: []character.Skill{
		{
			RecordID:        "records/skills/playerclass02/_classtraining_class02.dbr",
			Level:           36,
			LevelObserved:   true,
			Enabled:         true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        "records/skills/playerclass07/_classtraining_class07.dbr",
			Level:           50,
			LevelObserved:   true,
			Enabled:         true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        demolitionistSkill,
			Level:           3,
			LevelObserved:   true,
			Enabled:         true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        inquisitorSkill,
			Level:           2,
			LevelObserved:   true,
			Enabled:         true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        "records/skills/default/defaultpetattack.dbr",
			Level:           1,
			LevelObserved:   true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        defaultPotion,
			Level:           1,
			LevelObserved:   true,
			Enabled:         true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        potionModifier,
			Level:           1,
			LevelObserved:   true,
			Enabled:         true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
		{
			RecordID:        potionContainer,
			LevelObserved:   true,
			EnabledObserved: true,
			SaveBacked:      true,
		},
	}}
	sheet := gdedit.Data{Skills: []gdedit.SkillObservation{
		{
			Mastery: "Demolitionist",
			Name:    "Flame Touched",
			Level:   3,
			JoinKey: "ui/skills/icons/flametouched.tex",
		},
		{
			Mastery: "Inquisitor",
			Name:    "Inquisitor Seal",
			Level:   2,
			JoinKey: "ui/skills/icons/inquisitorseal.tex",
		},
		{
			Mastery:   "Demolitionist",
			Name:      "Uninvested gear-bonused node",
			GearBonus: 2,
			JoinKey:   "ui/skills/icons/unmatched.tex",
		},
	}}

	got, err := MergeWithDatabase(grim, sheet, database)
	require.NoError(t, err)
	require.Len(t, got.Skills, 9)
	assert.Equal(t, "Demolitionist", got.Skills[0].Name)
	assert.Equal(t, "Demolitionist", got.Skills[0].Mastery)
	assert.Equal(t, character.SkillCategoryMasteryBar, got.Skills[0].Category)
	assert.Equal(t, uint32(36), got.Skills[0].Level)
	assert.Equal(t, character.SkillMatchStatusNoARZRecord, got.Skills[0].MatchStatus)
	assert.Equal(t, "Inquisitor", got.Skills[1].Name)
	assert.Equal(t, "Inquisitor", got.Skills[1].Mastery)
	assert.Equal(t, character.SkillCategoryMasteryBar, got.Skills[1].Category)
	assert.Equal(t, uint32(50), got.Skills[1].Level)
	assert.Equal(t, character.SkillMatchStatusNoARZRecord, got.Skills[1].MatchStatus)
	assert.Equal(t, "Flame Touched", got.Skills[2].Name)
	assert.Equal(t, character.SkillCategoryMastery, got.Skills[2].Category)
	assert.Equal(t, uint32(3), got.Skills[2].Level)
	assert.Equal(t, uint32(3), got.Skills[2].GDEditLevel)
	assert.Equal(t, demolitionistSkill, got.Skills[2].RecordID)
	assert.Equal(t, "Inquisitor Seal", got.Skills[3].Name)
	assert.Equal(t, uint32(2), got.Skills[3].Level)
	assert.Equal(t, character.SkillCategoryDefault, got.Skills[4].Category)
	assert.Equal(t, character.SkillMatchStatusNoARZRecord, got.Skills[4].MatchStatus)
	assert.Equal(t, character.SkillNameResolutionMissing, got.Skills[4].NameResolution)
	assert.Equal(t, character.SkillCategoryDefault, got.Skills[5].Category)
	assert.Equal(t, "tagDefaultHealthPotion", got.Skills[5].DisplayTag)
	assert.Equal(t, character.SkillNameResolutionTagOnly, got.Skills[5].NameResolution)
	assert.Equal(t, character.SkillCategoryPotion, got.Skills[6].Category)
	assert.Equal(t, "tagHealthPotionHealOverTime", got.Skills[6].DisplayTag)
	assert.Equal(t, character.SkillNameResolutionTagOnly, got.Skills[6].NameResolution)
	assert.Equal(t, character.SkillMatchStatusNoJoinKey, got.Skills[6].MatchStatus)
	assert.True(t, got.Skills[6].Enabled)
	assert.True(t, got.Skills[6].EnabledObserved)
	assert.True(t, got.Skills[6].SaveBacked)
	assert.Equal(t, character.SkillCategoryPotion, got.Skills[7].Category)
	assert.Equal(t, uint32(0), got.Skills[7].Level)
	assert.False(t, got.Skills[7].Enabled)
	assert.True(t, got.Skills[7].SaveBacked)
	assert.Empty(t, got.Skills[7].DisplayTag)
	assert.Equal(t, character.SkillNameResolutionMissing, got.Skills[7].NameResolution)
	assert.Equal(t, "Uninvested gear-bonused node", got.Skills[8].Name)
	assert.Equal(t, uint32(0), got.Skills[8].GDEditLevel)
	assert.Equal(t, uint32(2), got.Skills[8].GearBonus)
	assert.False(t, got.Skills[8].SaveBacked)
	assert.False(t, got.Skills[8].EnabledObserved)
	assert.True(t, got.Skills[8].GDEditOnly)
	assert.Equal(t, character.SkillMatchStatusUnusedHTMLNode, got.Skills[8].MatchStatus)
	assert.Empty(t, got.Diagnostics)
}

func TestMergeMasteryBarFallbackAndMissingDatabaseStatus(t *testing.T) {
	t.Parallel()

	got, err := Merge(character.Character{Skills: []character.Skill{
		{RecordID: "records/skills/playerclass02/_classtraining_class02.dbr", Level: 36, SaveBacked: true},
		{RecordID: "records/skills/playerclass07/_classtraining_class07.dbr", Level: 50, SaveBacked: true},
		{RecordID: "records/skills/default/defaultpetattack.dbr", Level: 1, Enabled: false, SaveBacked: true},
	}}, gdedit.Data{})
	require.NoError(t, err)
	require.Len(t, got.Skills, 3)
	assert.Equal(t, "Demolitionist", got.Skills[0].Name)
	assert.Equal(t, "Inquisitor", got.Skills[1].Name)
	assert.Equal(t, uint32(36), got.Skills[0].Level)
	assert.Equal(t, uint32(50), got.Skills[1].Level)
	assert.Equal(t, character.SkillMatchStatusNoDatabase, got.Skills[2].MatchStatus)
	assert.Equal(t, character.SkillNameResolutionNoDatabase, got.Skills[2].NameResolution)
	assert.False(t, got.Skills[2].Enabled)
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
	assert.Equal(t, character.SkillMatchStatusAmbiguous, got.Skills[0].MatchStatus)
	assert.Equal(t, character.SkillMatchStatusAmbiguous, got.Skills[1].MatchStatus)
	assert.True(t, got.Skills[2].GDEditOnly)
	assert.Empty(t, got.Skills[2].RecordID)
	assert.Equal(t, character.SkillMatchStatusAmbiguous, got.Skills[2].MatchStatus)
	require.Len(t, got.Diagnostics, 1)
	assert.Equal(t, character.DiagnosticCodeSkillUnmatchedInvested, got.Diagnostics[0].Code)
}

func TestMergeUsesStableRecordIDBeforeAmbiguousTextures(t *testing.T) {
	t.Parallel()

	const texture = "ui/skills/icons/shared.tex"
	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/skills/playerclass01/first.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {texture},
			},
		},
		{
			Name: "records/skills/playerclass01/second.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {texture},
			},
		},
	}))
	require.NoError(t, err)

	got, err := MergeWithDatabase(character.Character{Skills: []character.Skill{
		{RecordID: "records/skills/playerclass01/first.dbr", Level: 1, SaveBacked: true},
		{RecordID: "records/skills/playerclass01/second.dbr", Level: 2, SaveBacked: true},
	}}, gdedit.Data{Skills: []gdedit.SkillObservation{{
		RecordID: "records/skills/playerclass01/second.dbr",
		Mastery:  "Soldier",
		Name:     "Exact Match",
		Level:    2,
		JoinKey:  texture,
	}}}, database)
	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Empty(t, got.Skills[0].Name)
	assert.Equal(t, character.SkillMatchStatusNoHTMLNode, got.Skills[0].MatchStatus)
	assert.Equal(t, "Exact Match", got.Skills[1].Name)
	assert.Equal(t, character.SkillMatchStatusMatchedRecordID, got.Skills[1].MatchStatus)
	assert.Equal(t, uint32(2), got.Skills[1].Level)
	assert.Empty(t, got.Diagnostics)
}

func TestMergeDoesNotFallbackAcrossConflictingExplicitRecordIDs(t *testing.T) {
	t.Parallel()

	const texture = "ui/skills/icons/shared.tex"
	database, err := arz.New(arzlib.Encode([]arzlib.EncRec{
		{
			Name: "records/skills/playerclass01/save-skill.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {texture},
			},
		},
		{
			Name: "records/skills/playerclass01/html-skill.dbr",
			Strings: map[string][]string{
				"skillUpBitmapName": {texture},
			},
		},
	}))
	require.NoError(t, err)

	got, err := MergeWithDatabase(character.Character{Skills: []character.Skill{{
		RecordID: "records/skills/playerclass01/save-skill.dbr",
		Level:    4,
	}}}, gdedit.Data{Skills: []gdedit.SkillObservation{{
		RecordID: "records/skills/playerclass01/html-skill.dbr",
		Mastery:  "Soldier",
		Name:     "Conflicting Node",
		Level:    4,
		JoinKey:  texture,
	}}}, database)
	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Equal(t, "records/skills/playerclass01/save-skill.dbr", got.Skills[0].RecordID)
	assert.Empty(t, got.Skills[0].Name)
	assert.Equal(t, character.SkillMatchStatusRecordIDConflict, got.Skills[0].MatchStatus)
	assert.True(t, got.Skills[1].GDEditOnly)
	assert.Equal(t, "records/skills/playerclass01/html-skill.dbr", got.Skills[1].RecordID)
	assert.Equal(t, "Conflicting Node", got.Skills[1].Name)
	assert.Equal(t, character.SkillMatchStatusRecordIDConflict, got.Skills[1].MatchStatus)
	assert.Equal(t, uint32(4), got.Skills[1].GDEditLevel)
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
