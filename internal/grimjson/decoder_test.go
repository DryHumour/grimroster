package grimjson

import (
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
