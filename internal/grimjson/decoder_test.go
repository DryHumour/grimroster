package grimjson

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/DryHumour/grimroster/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeMapsCharacterAndInventory(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"Header":{"Level":12,"IsMale":true,"IsHardcore":true,"Expansion":7,"Name":"Grim Name","Tag":"tagClass"},
		"Block2":{"Level":12,"Experience":900,"AttributePoints":2,"SkillPoints":3,"DevotionPoints":4,"TotalDevotionPoints":20,"Physique":100,"Cunning":110,"Spirit":120,"Health":1300,"Energy":400},
		"Block8":{"Skills":[]},
		"Block3":{
			"Head":{"Seed":123,"Amount":1,"Name":"records/items/helm.dbr","Prefix":"records/affixes/prefix.dbr","Component":"records/components/component.dbr"},
			"LeftHand2":{"Name":""},
			"Sacks":[{"HasUnseenItems":true,"Items":[{"Name":"records/items/potion.dbr","Amount":5,"X":2,"Y":3}]},{"HasUnseenItems":false,"Items":null}]
		}
	}`)

	got, err := Decode(data)
	require.NoError(t, err)
	assert.Equal(t, "Grim Name", got.Name)
	assert.Equal(t, "tagClass", got.ClassTag)
	assert.Equal(t, uint32(12), got.Level)
	assert.Equal(t, "male", string(got.Gender))
	assert.True(t, got.Hardcore)
	assert.Equal(t, uint8(7), got.Expansions)
	assert.Equal(t, uint32(900), got.Progress.Experience)
	assert.Equal(t, float32(120), got.Progress.Attributes.Spirit)
	require.Len(t, got.Inventory.Equipment, 1)
	assert.Equal(t, "records/items/helm.dbr", got.Inventory.Equipment[0].Item.RecordID)
	assert.Equal(t, uint32(123), got.Inventory.Equipment[0].Item.Seed)
	assert.Equal(t, "records/components/component.dbr", got.Inventory.Equipment[0].Item.ComponentID)
	require.Len(t, got.Inventory.Bags, 2)
	assert.True(t, got.Inventory.Bags[0].HasUnseenItems)
	require.Len(t, got.Inventory.Bags[0].Items, 1)
	assert.Equal(t, uint32(5), got.Inventory.Bags[0].Items[0].Item.Amount)
	assert.Empty(t, got.Inventory.Bags[1].Items)
	assert.NotNil(t, got.Inventory.Bags[1].Items)
}

func TestDecodeUsesBlockLevelWhenHeaderLevelIsZero(t *testing.T) {
	t.Parallel()

	got, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{"Level":9},"Block3":{},"Block8":{"Skills":[]}}`))
	require.NoError(t, err)
	assert.Equal(t, uint32(9), got.Level)
	assert.Equal(t, "female", string(got.Gender))
}

func TestDecodeRejectsMalformedOrIncompleteInput(t *testing.T) {
	t.Parallel()

	_, err := Decode([]byte("{"))
	assert.ErrorContains(t, err, "decode grim character JSON")

	_, err = Decode([]byte(`{"Block2":{},"Block3":{}}`))
	assert.ErrorContains(t, err, "missing Header")
}

func TestDecodeReportsNonNullBlockDecodingErrors(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"Header":{"Name":"A"},
		"Block1":{"DecodingError":null},
		"Block2":{"DecodingError":"progression could not be decoded"},
		"Block3":{},
		"Block8":{"Skills":[]},
		"Block17":{"DecodingError":{"reason":"invalid data"}}
	}`)

	got, err := Decode(data)
	require.NoError(t, err)
	require.Len(t, got.Diagnostics, 2)
	assert.Equal(t, character.SeverityError, got.Diagnostics[0].Severity)
	assert.Equal(t, character.DiagnosticCodeGrimBlockDecodingError, got.Diagnostics[0].Code)
	assert.Equal(t, "grim reported a decoding error in Block2.", got.Diagnostics[0].Message)
	assert.Equal(t, "Block2.DecodingError", got.Diagnostics[0].Field)
	assert.JSONEq(t, `"progression could not be decoded"`, string(got.Diagnostics[0].GrimValue))
	assert.JSONEq(t, `null`, string(got.Diagnostics[0].GDEditValue))
	assert.Equal(t, "Block17.DecodingError", got.Diagnostics[1].Field)
	assert.JSONEq(t, `{"reason":"invalid data"}`, string(got.Diagnostics[1].GrimValue))
	assert.JSONEq(t, `null`, string(got.Diagnostics[1].GDEditValue))
}

func TestDecodeImportsGrimSkillsAndBindingRecords(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"Header":{"Name":"A"},
		"Block2":{},
		"Block3":{},
		"Block8":{"Skills":[
			{"Name":"records/skills/playerclass02/firestrike1.dbr","Level":5,"DevotionLevel":0,"DevotionExperience":0,"Sublevel":1,"IsEnabled":true,"IsAura":false,"IsLocked":false,"Autocast":"records/skills/devotion/tier1_20e_skill.dbr","Controller":"records/controllers/cast.dbr"},
			{"Name":"records/skills/devotion/tier1_20e_skill.dbr","Level":1,"DevotionLevel":14,"DevotionExperience":12345,"IsEnabled":true},
			null
		]}
	}`)

	got, err := Decode(data)
	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Equal(t, "records/skills/playerclass02/firestrike1.dbr", got.Skills[0].RecordID)
	assert.Equal(t, character.SkillCategoryMastery, got.Skills[0].Category)
	assert.True(t, got.Skills[0].SaveBacked)
	assert.True(t, got.Skills[0].LevelObserved)
	assert.True(t, got.Skills[0].EnabledObserved)
	assert.Equal(t, uint32(5), got.Skills[0].Level)
	assert.Equal(t, "records/skills/devotion/tier1_20e_skill.dbr", got.Skills[0].AutocastRecordID)
	assert.Equal(t, "records/controllers/cast.dbr", got.Skills[0].ControllerRecordID)
	assert.Equal(t, character.SkillCategoryDevotion, got.Skills[1].Category)
	assert.Equal(t, uint32(14), got.Skills[1].DevotionLevel)
	assert.Equal(t, uint32(12345), got.Skills[1].DevotionExperience)
}

func TestDecodeDistinguishesMissingBlock8FromEmptySkills(t *testing.T) {
	t.Parallel()

	missing, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{}}`))
	require.NoError(t, err)
	require.Len(t, missing.Diagnostics, 1)
	assert.Equal(t, character.SeverityError, missing.Diagnostics[0].Severity)
	assert.Equal(t, character.DiagnosticCodeGrimBlockMissing, missing.Diagnostics[0].Code)
	assert.Equal(t, "Block8", missing.Diagnostics[0].Field)
	assert.Equal(t, "grim skill data is unavailable because Block8 is missing.", missing.Diagnostics[0].Message)
	assert.JSONEq(t, `null`, string(missing.Diagnostics[0].GrimValue))
	assert.JSONEq(t, `null`, string(missing.Diagnostics[0].GDEditValue))

	empty, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{},"Block8":{"Skills":[]}}`))
	require.NoError(t, err)
	assert.Empty(t, empty.Skills)
	assert.Empty(t, empty.Diagnostics)
}

func TestDecodeFactionReputationsPreservesIdentityAndKnownZero(t *testing.T) {
	t.Parallel()

	makeFaction := func(changed, unlocked bool, reputation, positiveBoost, negativeBoost float32) *factionValue {
		return &factionValue{
			IsChanged:     &changed,
			IsUnlocked:    &unlocked,
			Value:         &reputation,
			PositiveBoost: &positiveBoost,
			NegativeBoost: &negativeBoost,
		}
	}
	factions := make([]*factionValue, 15)
	factions[0] = makeFaction(false, false, 0, 0, 0)
	factions[2] = makeFaction(true, true, 1500, 3, 0)
	factions[3] = makeFaction(true, true, -8000, 0, 0)
	factions[4] = makeFaction(true, true, -1500, 0, 0)
	factions[5] = makeFaction(false, false, -1, 0, 0)
	factions[8] = makeFaction(false, false, 0, 0, 0)
	factions[11] = makeFaction(true, true, -1500, 0, 0)
	factions[14] = makeFaction(true, true, 25000, 0, 0)

	block13, err := json.Marshal(factionBlock{Version: 5, Faction: uint32Pointer(0), Factions: factions})
	require.NoError(t, err)
	data := []byte(fmt.Sprintf(
		`{"Header":{"Name":"Factions"},"Block2":{},"Block3":{},"Block13":%s}`,
		block13,
	))

	got, err := Decode(data)
	require.NoError(t, err)
	assert.Equal(t, character.FactionDataSourceGrim, got.FactionDataSource)
	require.NotNil(t, got.MyFaction)
	assert.Equal(t, "Player", got.MyFaction.ID)
	assert.Equal(t, character.FactionIdentitySourceGrimSchema, got.MyFaction.Source)
	require.Len(t, got.FactionReputations, len(factions))

	player := got.FactionReputations[0]
	assert.Equal(t, "Player", player.Faction.ID)
	assert.False(t, player.Faction.DefinitionKnown)
	assert.True(t, player.ReputationKnown)
	assert.Zero(t, player.Reputation)
	assert.False(t, player.IsUnlocked)
	assert.True(t, player.IsUnlockedKnown)

	assert.Equal(t, "Survivors", got.FactionReputations[1].Faction.ID)
	assert.Equal(t, "Devil's Crossing", got.FactionReputations[1].Faction.Name)
	assert.False(t, got.FactionReputations[1].ReputationKnown)
	assert.Equal(t, "Aetherials", got.FactionReputations[2].Faction.ID)
	assert.Equal(t, float32(1500), got.FactionReputations[2].Reputation)
	assert.Equal(t, float32(3), got.FactionReputations[2].PositiveBoost)
	assert.True(t, got.FactionReputations[2].PositiveBoostKnown)
	assert.Equal(t, "Cthonians", got.FactionReputations[3].Faction.ID)
	assert.Equal(t, float32(-8000), got.FactionReputations[3].Reputation)
	assert.Equal(t, "Outlaws", got.FactionReputations[4].Faction.ID)
	assert.Equal(t, "Cronley's Gang", got.FactionReputations[4].Faction.Name)
	assert.Equal(t, "Beasts", got.FactionReputations[5].Faction.ID)
	assert.False(t, got.FactionReputations[5].ReputationKnown)
	assert.Zero(t, got.FactionReputations[8].Reputation)
	assert.True(t, got.FactionReputations[8].ReputationKnown)
	assert.Equal(t, "User5", got.FactionReputations[11].Faction.ID)
	assert.Equal(t, uint32(11), got.FactionReputations[11].Faction.SourceIndex)
	assert.Equal(t, float32(-1500), got.FactionReputations[11].Reputation)
	assert.Equal(t, "User8", got.FactionReputations[14].Faction.ID)
	assert.Equal(t, float32(25000), got.FactionReputations[14].Reputation)
}

func TestDecodeFactionReputationsKeepsMissingOrUndecodableDataUnknown(t *testing.T) {
	t.Parallel()

	missing, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{}}`))
	require.NoError(t, err)
	assert.Equal(t, character.FactionDataSourceUnknown, missing.FactionDataSource)
	assert.Empty(t, missing.FactionReputations)
	assert.Nil(t, missing.MyFaction)

	identityOnly, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{},"Block13":{"Version":5,"Faction":0}}`))
	require.NoError(t, err)
	assert.Equal(t, character.FactionDataSourceUnknown, identityOnly.FactionDataSource)
	assert.Empty(t, identityOnly.FactionReputations)
	require.NotNil(t, identityOnly.MyFaction)
	assert.Equal(t, "Player", identityOnly.MyFaction.ID)

	undecodable, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{},"Block8":{"Skills":[]},"Block13":{"Version":5,"DecodingError":"bad faction block","Factions":[{"Value":0}]}}`))
	require.NoError(t, err)
	assert.Equal(t, character.FactionDataSourceUnknown, undecodable.FactionDataSource)
	assert.Empty(t, undecodable.FactionReputations)
	require.Len(t, undecodable.Diagnostics, 1)
	assert.Equal(t, character.DiagnosticCodeGrimBlockDecodingError, undecodable.Diagnostics[0].Code)

	malformed, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{},"Block8":{"Skills":[]},"Block13":"not a block"}`))
	require.NoError(t, err)
	assert.Equal(t, character.FactionDataSourceUnknown, malformed.FactionDataSource)
	assert.Empty(t, malformed.FactionReputations)
	require.Len(t, malformed.Diagnostics, 1)
	assert.Equal(t, character.DiagnosticCodeFactionBlockUnavailable, malformed.Diagnostics[0].Code)

	unknownVersion, err := Decode([]byte(`{"Header":{"Name":"A"},"Block2":{},"Block3":{},"Block13":{"Version":6,"Faction":0,"Factions":[{"Value":0}]}}`))
	require.NoError(t, err)
	assert.Equal(t, character.FactionDataSourceGrim, unknownVersion.FactionDataSource)
	require.Len(t, unknownVersion.FactionReputations, 1)
	assert.False(t, unknownVersion.FactionReputations[0].Faction.Known)
	assert.True(t, unknownVersion.FactionReputations[0].ReputationKnown)
}

func uint32Pointer(value uint32) *uint32 {
	return &value
}
