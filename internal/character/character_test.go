package character

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCharacterJSONRoundTrip(t *testing.T) {
	t.Parallel()

	want := Character{
		Name:       "Test Character",
		ClassID:    "Class01",
		ClassName:  "Soldier",
		ClassTag:   "tagSkillClassName01",
		Masteries:  []string{"Soldier", "Arcanist"},
		Level:      100,
		Gender:     GenderFemale,
		Hardcore:   true,
		Expansions: 7,
		Expansion:  "Ashes of Malmouth",
		Difficulty: "Ultimate",
		Progress: Progression{
			Experience:      123456,
			AttributePoints: 3,
			SkillPoints:     4,
			DevotionPoints:  2,
			TotalDevotion:   55,
			Attributes: Attributes{
				Physique: 800,
				Cunning:  500,
				Spirit:   650,
				Health:   12000,
				Energy:   3000,
			},
		},
		Inventory: Inventory{
			Equipment: []EquippedItem{
				{
					Slot: SlotHead,
					Item: Item{
						RecordID:      "records/items/gearhead01.dbr",
						Seed:          123,
						DisplayName:   "Ironclad Helm",
						NameTag:       "tagGearHead01",
						PrefixID:      "records/items/affixes/prefix01.dbr",
						GDEditDetails: jsontext.Value(`{"slot":"Head","name":"Ironclad Helm"}`),
					},
				},
			},
			Bags: []Bag{
				{
					Index:          0,
					HasUnseenItems: true,
					Items: []BagItem{
						{
							X: 1,
							Y: 2,
							Item: Item{
								RecordID: "records/items/consumables/potion01.dbr",
								Amount:   3,
							},
						},
					},
				},
			},
		},
		Skills: []Skill{{
			RecordID:           "records/skills/playerclass02/firestrike1.dbr",
			Name:               "Fire Strike",
			Mastery:            "Demolitionist",
			Level:              5,
			AutocastRecordID:   "records/skills/devotion/tier1_20e_skill.dbr",
			ControllerRecordID: "records/controllers/cast_@selfonattack_25%.dbr",
			GDEditLevel:        5,
			GearBonus:          2,
			MaxLevel:           12,
			Description:        "An attack skill.",
			Current:            []string{"current effect"},
			Next:               []string{"next effect"},
		}},
		Devotions: []Devotion{{
			Name:       "Fiend",
			StarsTaken: 5,
			StarsTotal: 5,
			Complete:   true,
			Stars: []DevotionStar{{
				Name:               "Flame Torrent",
				Button:             5,
				Skill:              true,
				Taken:              true,
				RecordID:           "records/skills/devotion/tier1_20e_skill.dbr",
				Level:              1,
				DevotionLevel:      14,
				DevotionExperience: 2672063,
				BoundSkill:         "Fire Strike",
				BoundSkillRecordID: "records/skills/playerclass02/firestrike1.dbr",
				Trigger:            "selfonattack_25%",
			}},
		}},
		Calculated: &Calculated{
			Health:           12000,
			DefensiveAbility: 1500,
			Armor:            500,
			ArmorBySlot:      map[string]float64{"Head": 500},
			Resistances:      []Resistance{{Field: "defensiveFire", Label: "Fire", Shown: 80, Cap: 80, Over: 20}},
		},
		Diagnostics: []Diagnostic{{
			Severity:    SeverityWarning,
			Code:        DiagnosticCodeCharacterLevelMismatch,
			Message:     "Character level differs between grim and gd-edit.",
			Field:       "level",
			GrimValue:   jsontext.Value(`99`),
			GDEditValue: jsontext.Value(`100`),
		}},
	}

	data, err := jsonv2.Marshal(want)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	var got Character
	err = jsonv2.Unmarshal(data, &got)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCharacterJSONFieldNames(t *testing.T) {
	t.Parallel()

	data, err := jsonv2.Marshal(Character{
		Name:      "Example",
		ClassID:   "Class01",
		ClassName: "Soldier",
		Gender:    GenderMale,
		Inventory: Inventory{
			Equipment: []EquippedItem{{
				Slot: SlotRightHand2,
				Item: Item{
					RecordID:    "records/items/weapon01.dbr",
					DisplayName: "Example Sword",
					NameTag:     "tagWeapon01",
				},
			}},
			Bags: []Bag{},
		},
	})
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.JSONEq(t, `{
		"name": "Example",
		"class_id": "Class01",
		"class_name": "Soldier",
		"level": 0,
		"gender": "male",
		"hardcore": false,
		"expansions": 0,
		"progression": {
			"experience": 0,
			"attribute_points": 0,
			"skill_points": 0,
			"devotion_points": 0,
			"total_devotion_points": 0,
			"attributes": {
				"physique": 0,
				"cunning": 0,
				"spirit": 0,
				"health": 0,
				"energy": 0
			}
		},
		"inventory": {
			"equipment": [{
				"slot": "right_hand_2",
				"item": {
					"record_id": "records/items/weapon01.dbr",
					"display_name": "Example Sword",
					"name_tag": "tagWeapon01",
					"amount": 0
				}
			}],
			"bags": []
		},
		"current_affinities": {
			"ascendant": {"points": 0, "known": false},
			"chaos": {"points": 0, "known": false},
			"eldritch": {"points": 0, "known": false},
			"order": {"points": 0, "known": false},
			"primordial": {"points": 0, "known": false}
		}
	}`, string(data))
}
