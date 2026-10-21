package gdedit

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/DryHumour/grimroster/internal/character"
)

type Data struct {
	Name       string
	Level      uint32
	Masteries  []string
	Difficulty string
	Expansion  string
	Calculated character.Calculated
	Equipment  []Equipment
	Skills     []SkillObservation
	Devotions  []Devotion
}

type Equipment struct {
	Slot string
	Name string
	Seed uint32
	Raw  jsontext.Value
}

type SkillObservation struct {
	RecordID      string
	Mastery       string
	Name          string
	Level         uint32
	GearBonus     uint32
	MaxLevel      uint32
	UltimateLevel uint32
	Description   string
	Current       []string
	Next          []string
	JoinKey       string
}

type Devotion struct {
	Name       string
	JoinKey    string
	StarsTaken uint32
	StarsTotal uint32
	Complete   bool
	Stars      []DevotionStar
}

type DevotionStar struct {
	Name    string
	Button  uint32
	Skill   bool
	Taken   bool
	Bound   string
	Trigger string
}

type document struct {
	Sheet *sheet `json:"sheet"`
	Tree  *tree  `json:"tree"`
	Devo  *devo  `json:"devo"`
}

type sheet struct {
	Character   *sheetCharacter        `json:"character"`
	Stats       *gdEditStats           `json:"stats"`
	Resistances []character.Resistance `json:"resistances"`
	Equipment   []jsontext.Value       `json:"equipment"`
}

type gdEditStats struct {
	Physique         float64            `json:"physique"`
	Cunning          float64            `json:"cunning"`
	Spirit           float64            `json:"spirit"`
	Health           float64            `json:"health"`
	Energy           float64            `json:"energy"`
	EnergyReserved   float64            `json:"energyReserved"`
	EnergyUsable     float64            `json:"energyUsable"`
	OffensiveAbility float64            `json:"offensiveAbility"`
	DefensiveAbility float64            `json:"defensiveAbility"`
	Armor            float64            `json:"armour"`
	ArmorBySlot      map[string]float64 `json:"armourBySlot"`
}

type sheetCharacter struct {
	Name       string   `json:"name"`
	Level      uint32   `json:"level"`
	Masteries  []string `json:"masteries"`
	Difficulty string   `json:"difficulty"`
	Expansion  string   `json:"expansion"`
}

type equipmentItem struct {
	Slot string `json:"slot"`
	Name string `json:"name"`
	Seed uint32 `json:"seed"`
}

type tree struct {
	Trees []masteryTree `json:"trees"`
}

type masteryTree struct {
	Mastery string      `json:"mastery"`
	Nodes   []skillNode `json:"nodes"`
}

type skillNode struct {
	RecordID      string   `json:"record_id"`
	RecordIDCamel string   `json:"recordId"`
	Name          string   `json:"name"`
	Description   string   `json:"desc"`
	Level         uint32   `json:"level"`
	GearBonus     uint32   `json:"gear"`
	MaxLevel      uint32   `json:"max"`
	UltimateLevel uint32   `json:"ultimate"`
	Current       []string `json:"cur"`
	Next          []string `json:"next"`
	Texture       string   `json:"tex"`
}

type devo struct {
	Constellations []devotionConstellation `json:"constellations"`
}

type devotionConstellation struct {
	Name       string         `json:"name"`
	Bitmap     string         `json:"bitmap"`
	StarsTaken uint32         `json:"starsTaken"`
	StarsTotal uint32         `json:"starsTotal"`
	Complete   bool           `json:"complete"`
	Stars      []devotionStar `json:"stars"`
}

type devotionStar struct {
	Name   string      `json:"name"`
	Button uint32      `json:"button"`
	Skill  bool        `json:"skill"`
	Taken  bool        `json:"taken"`
	Bound  *boundSkill `json:"bound"`
}

type boundSkill struct {
	Name    string `json:"name"`
	Trigger string `json:"trigger"`
}

var ansiEscapeSequence = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func DecodeJSON(data []byte) (Data, error) {
	var input document
	if err := jsonv2.Unmarshal(data, &input); err != nil {
		return Data{}, fmt.Errorf("decode gd-edit JSON: %w", err)
	}
	if input.Sheet == nil {
		return Data{}, errors.New("decode gd-edit JSON: missing sheet")
	}
	if input.Sheet.Character == nil {
		return Data{}, errors.New("decode gd-edit JSON: missing sheet.character")
	}
	if input.Sheet.Stats == nil {
		return Data{}, errors.New("decode gd-edit JSON: missing sheet.stats")
	}

	result := Data{
		Name:       stripANSI(input.Sheet.Character.Name),
		Level:      input.Sheet.Character.Level,
		Masteries:  stripANSIStrings(input.Sheet.Character.Masteries),
		Difficulty: stripANSI(input.Sheet.Character.Difficulty),
		Expansion:  stripANSI(input.Sheet.Character.Expansion),
		Calculated: character.Calculated{
			Physique:         input.Sheet.Stats.Physique,
			Cunning:          input.Sheet.Stats.Cunning,
			Spirit:           input.Sheet.Stats.Spirit,
			Health:           input.Sheet.Stats.Health,
			Energy:           input.Sheet.Stats.Energy,
			EnergyReserved:   input.Sheet.Stats.EnergyReserved,
			EnergyUsable:     input.Sheet.Stats.EnergyUsable,
			OffensiveAbility: input.Sheet.Stats.OffensiveAbility,
			DefensiveAbility: input.Sheet.Stats.DefensiveAbility,
			Armor:            input.Sheet.Stats.Armor,
			ArmorBySlot:      input.Sheet.Stats.ArmorBySlot,
		},
		Equipment: make([]Equipment, 0, len(input.Sheet.Equipment)),
		Skills:    make([]SkillObservation, 0),
		Devotions: make([]Devotion, 0),
	}
	result.Calculated.Resistances = slices.Clone(input.Sheet.Resistances)
	for i := range result.Calculated.Resistances {
		result.Calculated.Resistances[i].Field = stripANSI(result.Calculated.Resistances[i].Field)
		result.Calculated.Resistances[i].Label = stripANSI(result.Calculated.Resistances[i].Label)
	}

	if input.Tree != nil {
		for _, mastery := range input.Tree.Trees {
			for _, node := range mastery.Nodes {
				if node.Name == "" {
					continue
				}
				result.Skills = append(result.Skills, SkillObservation{
					RecordID:      firstNonEmpty(node.RecordID, node.RecordIDCamel),
					Mastery:       stripANSI(mastery.Mastery),
					Name:          stripANSI(node.Name),
					Level:         node.Level,
					GearBonus:     node.GearBonus,
					MaxLevel:      node.MaxLevel,
					UltimateLevel: node.UltimateLevel,
					Description:   stripANSI(node.Description),
					Current:       stripANSIStrings(node.Current),
					Next:          stripANSIStrings(node.Next),
					JoinKey:       node.Texture,
				})
			}
		}
	}

	if input.Devo != nil {
		for _, constellation := range input.Devo.Constellations {
			devotion := Devotion{
				Name:       stripANSI(constellation.Name),
				JoinKey:    constellation.Bitmap,
				StarsTaken: constellation.StarsTaken,
				StarsTotal: constellation.StarsTotal,
				Complete:   constellation.Complete,
				Stars:      make([]DevotionStar, 0, len(constellation.Stars)),
			}
			for _, star := range constellation.Stars {
				var bound, trigger string
				if star.Bound != nil {
					bound = star.Bound.Name
					trigger = star.Bound.Trigger
				}
				devotion.Stars = append(devotion.Stars, DevotionStar{
					Name:    stripANSI(star.Name),
					Button:  star.Button,
					Skill:   star.Skill,
					Taken:   star.Taken,
					Bound:   stripANSI(bound),
					Trigger: stripANSI(trigger),
				})
			}
			result.Devotions = append(result.Devotions, devotion)
		}
	}

	for index, raw := range input.Sheet.Equipment {
		if raw.Kind() == jsontext.KindNull {
			continue
		}
		var item equipmentItem
		if err := jsonv2.Unmarshal(raw, &item); err != nil {
			return Data{}, fmt.Errorf("decode gd-edit sheet.equipment[%d]: %w", index, err)
		}
		if item.Name == "" {
			continue
		}
		result.Equipment = append(result.Equipment, Equipment{
			Slot: stripANSI(item.Slot),
			Name: stripANSI(item.Name),
			Seed: item.Seed,
			Raw:  raw,
		})
		cleaned, err := stripANSIJSON(raw)
		if err != nil {
			return Data{}, fmt.Errorf("strip ANSI codes from gd-edit sheet.equipment[%d]: %w", index, err)
		}
		result.Equipment[len(result.Equipment)-1].Raw = cleaned
	}

	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func DecodeHTML(data []byte) (Data, error) {
	const marker = "window.DATA"
	for offset := 0; offset < len(data); {
		index := bytes.Index(data[offset:], []byte(marker))
		if index < 0 {
			return Data{}, errors.New("extract gd-edit window.DATA: assignment not found")
		}
		index += offset + len(marker)
		remaining := strings.TrimLeft(string(data[index:]), " \t\r\n")
		offset = index
		if len(remaining) == 0 || remaining[0] != '=' {
			continue
		}

		decoder := jsontext.NewDecoder(strings.NewReader(strings.TrimLeft(remaining[1:], " \t\r\n")))
		payload, err := decoder.ReadValue()
		if err != nil {
			return Data{}, fmt.Errorf("extract gd-edit window.DATA JSON: %w", err)
		}
		result, err := DecodeJSON(payload)
		if err != nil {
			return Data{}, fmt.Errorf("decode gd-edit window.DATA: %w", err)
		}
		return result, nil
	}

	return Data{}, errors.New("extract gd-edit window.DATA: assignment not found")
}

func stripANSI(input string) string {
	return ansiEscapeSequence.ReplaceAllString(input, "")
}

func stripANSIStrings(input []string) []string {
	result := make([]string, len(input))
	for i, value := range input {
		result[i] = stripANSI(value)
	}
	return result
}

func stripANSIJSON(input jsontext.Value) (jsontext.Value, error) {
	var value any
	if err := jsonv2.Unmarshal(input, &value); err != nil {
		return nil, fmt.Errorf("decode raw JSON value: %w", err)
	}
	value = stripANSIValue(value)
	result, err := jsonv2.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode stripped JSON value: %w", err)
	}
	return jsontext.Value(result), nil
}

func stripANSIValue(value any) any {
	switch typed := value.(type) {
	case string:
		return stripANSI(typed)
	case []any:
		for i := range typed {
			typed[i] = stripANSIValue(typed[i])
		}
	case map[string]any:
		for key, field := range typed {
			typed[key] = stripANSIValue(field)
		}
	}
	return value
}
