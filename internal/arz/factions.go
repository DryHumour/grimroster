package arz

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	arzlib "codeberg.org/alex-ilchukov/arz/v2"
)

type FactionDefinition struct {
	ID       string
	Name     string
	NameTag  string
	RecordID string
}

type FactionStandingThreshold struct {
	Minimum float32
	Tier    string
	Name    string
	Tag     string
}

type FactionCatalog struct {
	Definitions             map[string]FactionDefinition
	StandingThresholds      []FactionStandingThreshold
	StandingThresholdsKnown bool
}

func (d *Database) FactionCatalog() (FactionCatalog, error) {
	const recordID = "records/game/gamefactions.dbr"
	result := FactionCatalog{
		Definitions: make(map[string]FactionDefinition),
	}
	refs, err := d.factionControllerReferences(recordID)
	if err != nil {
		return FactionCatalog{}, fmt.Errorf("read faction controller references: %w", err)
	}
	for _, controllerID := range refs {
		id, found, err := d.StringField(controllerID, "myFaction")
		if err != nil {
			return FactionCatalog{}, fmt.Errorf("read faction identity from %q: %w", controllerID, err)
		}
		if !found || id == "" {
			continue
		}
		nameTag := factionNameTag(id)
		name := id
		if localized, found := d.LocalizedText(nameTag); found &&
			localized != "" && !strings.HasPrefix(localized, "User") {
			name = localized
		}
		result.Definitions[id] = FactionDefinition{
			ID:       id,
			Name:     name,
			NameTag:  nameTag,
			RecordID: controllerID,
		}
	}

	thresholds := make([]FactionStandingThreshold, 0, 8)
	allThresholdsKnown := true
	for index := 1; index <= 8; index++ {
		minimum, minimumFound, err := d.Int32Field(recordID, fmt.Sprintf("factionValue%d", index))
		if err != nil {
			return FactionCatalog{}, fmt.Errorf("read faction tier threshold %d: %w", index, err)
		}
		tag, tagFound, err := d.StringField(recordID, fmt.Sprintf("factionTag%d", index))
		if err != nil {
			return FactionCatalog{}, fmt.Errorf("read faction tier tag %d: %w", index, err)
		}
		tier := factionTierID(tag)
		if !minimumFound || !tagFound || tier == "" {
			allThresholdsKnown = false
			continue
		}
		name := tier
		if localized, found := d.LocalizedText(tag); found &&
			localized != "" && !strings.HasPrefix(localized, "tag") {
			name = localized
		}
		thresholds = append(thresholds, FactionStandingThreshold{
			Minimum: float32(minimum),
			Tier:    tier,
			Name:    name,
			Tag:     tag,
		})
	}
	if allThresholdsKnown && len(thresholds) == 8 {
		slices.SortFunc(thresholds, func(a, b FactionStandingThreshold) int {
			return cmp.Compare(a.Minimum, b.Minimum)
		})
		result.StandingThresholds = thresholds
		result.StandingThresholdsKnown = true
	}
	return result, nil
}

func (d *Database) factionControllerReferences(recordID string) ([]string, error) {
	ref, found := d.records[recordID]
	if !found {
		return nil, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fields, err := ref.db.Fields(ref.record)
	if err != nil {
		return nil, fmt.Errorf("read fields from %q: %w", recordID, err)
	}
	refs := make([]string, 0)
	for field := range fields.Seq {
		fieldName := string(ref.db.String(field.Name()))
		if !strings.HasPrefix(fieldName, "faction") ||
			field.Type() != arzlib.FieldTypeString ||
			field.ValuesAmount() != 1 {
			continue
		}
		value := ref.db.String(field.Value())
		if value == nil {
			return nil, fmt.Errorf("faction field %q in %q has an invalid string", fieldName, recordID)
		}
		if strings.HasPrefix(string(value), "records/controllers/factions/") {
			refs = append(refs, string(value))
		}
	}
	slices.Sort(refs)
	return slices.Compact(refs), nil
}

func factionNameTag(id string) string {
	const userPrefix = "User"
	if !strings.HasPrefix(id, userPrefix) {
		return ""
	}
	if _, err := strconv.ParseUint(strings.TrimPrefix(id, userPrefix), 10, 32); err != nil {
		return ""
	}
	return "tagFaction" + id
}

func factionTierID(tag string) string {
	const tagPrefix = "tagFactionState"
	if !strings.HasPrefix(tag, tagPrefix) {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(tag, tagPrefix))
}
