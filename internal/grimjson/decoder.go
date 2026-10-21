package grimjson

import (
	"cmp"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/DryHumour/grimroster/internal/character"
)

type file struct {
	Header *header `json:"Header"`
	Block2 *block2 `json:"Block2"`
	Block3 *block3 `json:"Block3"`
	Block8 *block8 `json:"Block8"`
}

type header struct {
	Level      uint32 `json:"Level"`
	IsMale     bool   `json:"IsMale"`
	IsHardcore bool   `json:"IsHardcore"`
	Expansion  uint8  `json:"Expansion"`
	Name       string `json:"Name"`
	Tag        string `json:"Tag"`
}

type block2 struct {
	Level               uint32  `json:"Level"`
	Experience          uint32  `json:"Experience"`
	AttributePoints     uint32  `json:"AttributePoints"`
	SkillPoints         uint32  `json:"SkillPoints"`
	DevotionPoints      uint32  `json:"DevotionPoints"`
	TotalDevotionPoints uint32  `json:"TotalDevotionPoints"`
	Physique            float32 `json:"Physique"`
	Cunning             float32 `json:"Cunning"`
	Spirit              float32 `json:"Spirit"`
	Health              float32 `json:"Health"`
	Energy              float32 `json:"Energy"`
}

type block3 struct {
	Sacks      []sack    `json:"Sacks"`
	Head       *grimItem `json:"Head"`
	Necklace   *grimItem `json:"Necklace"`
	Torso      *grimItem `json:"Torso"`
	Legs       *grimItem `json:"Legs"`
	Feet       *grimItem `json:"Feet"`
	Hands      *grimItem `json:"Hands"`
	RightRing  *grimItem `json:"RightRing"`
	LeftRing   *grimItem `json:"LeftRing"`
	Waist      *grimItem `json:"Waist"`
	Shoulders  *grimItem `json:"Shoulders"`
	Medal      *grimItem `json:"Medal"`
	Relic      *grimItem `json:"Relic"`
	RightHand1 *grimItem `json:"RightHand1"`
	LeftHand1  *grimItem `json:"LeftHand1"`
	RightHand2 *grimItem `json:"RightHand2"`
	LeftHand2  *grimItem `json:"LeftHand2"`
}

type block8 struct {
	Skills []*grimSkill `json:"Skills"`
}

type grimSkill struct {
	Level              uint32 `json:"Level"`
	DevotionLevel      uint32 `json:"DevotionLevel"`
	DevotionExperience uint32 `json:"DevotionExperience"`
	Sublevel           uint32 `json:"Sublevel"`
	IsEnabled          bool   `json:"IsEnabled"`
	IsAura             bool   `json:"IsAura"`
	IsLocked           bool   `json:"IsLocked"`
	Name               string `json:"Name"`
	Autocast           string `json:"Autocast"`
	Controller         string `json:"Controller"`
}

type sack struct {
	HasUnseenItems bool        `json:"HasUnseenItems"`
	Items          []*grimItem `json:"Items"`
}

type grimItem struct {
	Seed          uint32 `json:"Seed"`
	Amount        uint32 `json:"Amount"`
	Name          string `json:"Name"`
	Prefix        string `json:"Prefix"`
	Suffix        string `json:"Suffix"`
	CraftingBonus string `json:"CraftingBonus"`
	Illusion      string `json:"Illusion"`
	Component     string `json:"Component"`
	RelicBonus    string `json:"RelicBonus"`
	Augment       string `json:"Augment"`
	Ascended      string `json:"Ascended"`
	X             uint32 `json:"X"`
	Y             uint32 `json:"Y"`
}

func Decode(data []byte) (character.Character, error) {
	var input file
	if err := json.Unmarshal(data, &input); err != nil {
		return character.Character{}, fmt.Errorf("decode grim character JSON: %w", err)
	}
	if input.Header == nil {
		return character.Character{}, errors.New("decode grim character JSON: missing Header")
	}
	if input.Block2 == nil {
		return character.Character{}, errors.New("decode grim character JSON: missing Block2")
	}
	if input.Block3 == nil {
		return character.Character{}, errors.New("decode grim character JSON: missing Block3")
	}

	level := input.Header.Level
	if level == 0 {
		level = input.Block2.Level
	}

	result := character.Character{
		Name:       input.Header.Name,
		ClassTag:   input.Header.Tag,
		Level:      level,
		Gender:     character.GenderFemale,
		Hardcore:   input.Header.IsHardcore,
		Expansions: input.Header.Expansion,
		Progress: character.Progression{
			Experience:      input.Block2.Experience,
			AttributePoints: input.Block2.AttributePoints,
			SkillPoints:     input.Block2.SkillPoints,
			DevotionPoints:  input.Block2.DevotionPoints,
			TotalDevotion:   input.Block2.TotalDevotionPoints,
			Attributes: character.Attributes{
				Physique: input.Block2.Physique,
				Cunning:  input.Block2.Cunning,
				Spirit:   input.Block2.Spirit,
				Health:   input.Block2.Health,
				Energy:   input.Block2.Energy,
			},
		},
		Inventory: character.Inventory{
			Equipment: make([]character.EquippedItem, 0, 16),
			Bags:      make([]character.Bag, 0, len(input.Block3.Sacks)),
		},
		Skills: make([]character.Skill, 0),
	}
	if input.Block8 != nil {
		result.Skills = make([]character.Skill, 0, len(input.Block8.Skills))
		for _, skill := range input.Block8.Skills {
			if skill == nil || skill.Name == "" {
				continue
			}
			result.Skills = append(result.Skills, character.Skill{
				RecordID:           skill.Name,
				Category:           character.ClassifySkillCategory(skill.Name, skill.DevotionLevel > 0),
				Level:              skill.Level,
				LevelObserved:      true,
				IsDevotion:         skill.DevotionLevel > 0,
				DevotionLevel:      skill.DevotionLevel,
				DevotionExperience: skill.DevotionExperience,
				Sublevel:           skill.Sublevel,
				Enabled:            skill.IsEnabled,
				EnabledObserved:    true,
				Aura:               skill.IsAura,
				Locked:             skill.IsLocked,
				AutocastRecordID:   skill.Autocast,
				ControllerRecordID: skill.Controller,
				SaveBacked:         true,
			})
		}
	}
	if input.Header.IsMale {
		result.Gender = character.GenderMale
	}

	equipment := []struct {
		slot character.EquipmentSlot
		item *grimItem
	}{
		{character.SlotHead, input.Block3.Head},
		{character.SlotNecklace, input.Block3.Necklace},
		{character.SlotTorso, input.Block3.Torso},
		{character.SlotLegs, input.Block3.Legs},
		{character.SlotFeet, input.Block3.Feet},
		{character.SlotHands, input.Block3.Hands},
		{character.SlotRightRing, input.Block3.RightRing},
		{character.SlotLeftRing, input.Block3.LeftRing},
		{character.SlotWaist, input.Block3.Waist},
		{character.SlotShoulders, input.Block3.Shoulders},
		{character.SlotMedal, input.Block3.Medal},
		{character.SlotRelic, input.Block3.Relic},
		{character.SlotRightHand1, input.Block3.RightHand1},
		{character.SlotLeftHand1, input.Block3.LeftHand1},
		{character.SlotRightHand2, input.Block3.RightHand2},
		{character.SlotLeftHand2, input.Block3.LeftHand2},
	}
	for _, entry := range equipment {
		if entry.item == nil || entry.item.Name == "" {
			continue
		}
		result.Inventory.Equipment = append(result.Inventory.Equipment, character.EquippedItem{
			Slot: entry.slot,
			Item: convertItem(entry.item),
		})
	}

	for index, inputSack := range input.Block3.Sacks {
		bag := character.Bag{
			Index:          uint32(index),
			HasUnseenItems: inputSack.HasUnseenItems,
			Items:          make([]character.BagItem, 0, len(inputSack.Items)),
		}
		for _, item := range inputSack.Items {
			if item == nil || item.Name == "" {
				continue
			}
			bag.Items = append(bag.Items, character.BagItem{
				X:    item.X,
				Y:    item.Y,
				Item: convertItem(item),
			})
		}
		result.Inventory.Bags = append(result.Inventory.Bags, bag)
	}

	blockDiagnostics, err := decodeBlockDiagnostics(data)
	if err != nil {
		return character.Character{}, fmt.Errorf("inspect grim block decoding errors: %w", err)
	}
	result.Diagnostics = blockDiagnostics

	return result, nil
}

func decodeBlockDiagnostics(data []byte) ([]character.Diagnostic, error) {
	var values map[string]jsontext.Value
	if err := jsonv2.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("decode grim JSON blocks: %w", err)
	}

	blockNames := make([]string, 0)
	for name, value := range values {
		if !strings.HasPrefix(name, "Block") || value.Kind() != jsontext.KindBeginObject {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(name, "Block")); err != nil {
			continue
		}
		blockNames = append(blockNames, name)
	}
	slices.SortFunc(blockNames, func(a, b string) int {
		aNumber, _ := strconv.Atoi(strings.TrimPrefix(a, "Block"))
		bNumber, _ := strconv.Atoi(strings.TrimPrefix(b, "Block"))
		return cmp.Compare(aNumber, bNumber)
	})

	diagnostics := make([]character.Diagnostic, 0)
	if block8, ok := values["Block8"]; !ok || block8.Kind() == jsontext.KindNull {
		diagnostics = append(diagnostics, character.Diagnostic{
			Severity:    character.SeverityError,
			Code:        character.DiagnosticCodeGrimBlockMissing,
			Message:     "grim skill data is unavailable because Block8 is missing.",
			Field:       "Block8",
			GrimValue:   jsontext.Value("null"),
			GDEditValue: jsontext.Value("null"),
		})
	}
	for _, name := range blockNames {
		var block map[string]jsontext.Value
		if err := jsonv2.Unmarshal(values[name], &block); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		decodingError, ok := block["DecodingError"]
		if !ok || decodingError.Kind() == jsontext.KindNull {
			continue
		}
		diagnostics = append(diagnostics, character.Diagnostic{
			Severity:    character.SeverityError,
			Code:        character.DiagnosticCodeGrimBlockDecodingError,
			Message:     fmt.Sprintf("grim reported a decoding error in %s.", name),
			Field:       name + ".DecodingError",
			GrimValue:   decodingError.Clone(),
			GDEditValue: jsontext.Value("null"),
		})
	}

	return diagnostics, nil
}

func convertItem(input *grimItem) character.Item {
	return character.Item{
		RecordID:        input.Name,
		Seed:            input.Seed,
		PrefixID:        input.Prefix,
		SuffixID:        input.Suffix,
		CraftingBonusID: input.CraftingBonus,
		IllusionID:      input.Illusion,
		ComponentID:     input.Component,
		RelicBonusID:    input.RelicBonus,
		AugmentID:       input.Augment,
		AscendedAffixID: input.Ascended,
		Amount:          input.Amount,
	}
}
