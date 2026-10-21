package arz

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/DryHumour/grimroster/internal/character"
)

type DevotionConstellation struct {
	DisplayTag           string
	Bitmap               string
	AffinityRequirements character.AffinityAmounts
	AffinityRewards      character.AffinityAmounts
	Stars                map[uint32][]string
}

func (d *Database) DevotionConstellations() ([]DevotionConstellation, error) {
	constellationPrefix := "records/ui/skills/devotion/constellations/constellation"
	constellations := make(map[string]*DevotionConstellation)
	requirementConflicts := make(map[string]map[character.Affinity]bool)
	rewardConflicts := make(map[string]map[character.Affinity]bool)
	for _, recordID := range d.RecordIDsWithPrefix(constellationPrefix) {
		suffix := strings.TrimSuffix(strings.TrimPrefix(recordID, constellationPrefix), ".dbr")
		if _, err := strconv.Atoi(suffix); err != nil || !strings.HasSuffix(recordID, ".dbr") {
			continue
		}

		displayTag, found, err := d.StringField(recordID, "constellationDisplayTag")
		if err != nil {
			return nil, fmt.Errorf("read constellation display tag from %q: %w", recordID, err)
		}
		if !found || displayTag == "" {
			continue
		}
		requirements, err := d.affinityAmounts(recordID, "affinityRequired", "affinityRequiredName")
		if err != nil {
			return nil, fmt.Errorf("read affinity requirements from %q: %w", recordID, err)
		}
		rewards, err := d.affinityAmounts(recordID, "affinityGiven", "affinityGivenName")
		if err != nil {
			return nil, fmt.Errorf("read affinity rewards from %q: %w", recordID, err)
		}
		constellation := constellations[displayTag]
		if constellation == nil {
			constellation = &DevotionConstellation{
				DisplayTag:           displayTag,
				AffinityRequirements: requirements,
				AffinityRewards:      rewards,
				Stars:                make(map[uint32][]string),
			}
			constellations[displayTag] = constellation
		} else {
			if requirementConflicts[displayTag] == nil {
				requirementConflicts[displayTag] = make(map[character.Affinity]bool)
			}
			if rewardConflicts[displayTag] == nil {
				rewardConflicts[displayTag] = make(map[character.Affinity]bool)
			}
			constellation.AffinityRequirements = mergeAffinityAmounts(
				constellation.AffinityRequirements,
				requirements,
				requirementConflicts[displayTag],
			)
			constellation.AffinityRewards = mergeAffinityAmounts(
				constellation.AffinityRewards,
				rewards,
				rewardConflicts[displayTag],
			)
		}

		background, found, err := d.StringField(recordID, "constellationBackground")
		if err != nil {
			return nil, fmt.Errorf("read constellation background from %q: %w", recordID, err)
		}
		if found && background != "" {
			bitmap, found, err := d.StringField(background, "bitmapName")
			if err != nil {
				return nil, fmt.Errorf("read constellation bitmap from %q: %w", background, err)
			}
			if found && bitmap != "" && constellation.Bitmap == "" {
				constellation.Bitmap = bitmap
			}
		}

		buttonFields, err := d.StringFieldsWithPrefix(recordID, "devotionButton")
		if err != nil {
			return nil, fmt.Errorf("read devotion buttons from %q: %w", recordID, err)
		}
		buttonNames := make([]string, 0, len(buttonFields))
		for name := range buttonFields {
			buttonNames = append(buttonNames, name)
		}
		slices.Sort(buttonNames)
		for _, fieldName := range buttonNames {
			button, err := strconv.ParseUint(strings.TrimPrefix(fieldName, "devotionButton"), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parse devotion button field %q in %q: %w", fieldName, recordID, err)
			}
			buttonRecord := buttonFields[fieldName]
			skillRecord, found, err := d.StringField(buttonRecord, "skillName")
			if err != nil {
				return nil, fmt.Errorf("read skill name from devotion button %q: %w", buttonRecord, err)
			}
			if found && skillRecord != "" && !slices.Contains(constellation.Stars[uint32(button)], skillRecord) {
				constellation.Stars[uint32(button)] = append(constellation.Stars[uint32(button)], skillRecord)
			}
		}
	}

	result := make([]DevotionConstellation, 0, len(constellations))
	for _, constellation := range constellations {
		result = append(result, *constellation)
	}
	slices.SortFunc(result, func(a, b DevotionConstellation) int {
		return strings.Compare(a.DisplayTag, b.DisplayTag)
	})
	return result, nil
}

func (d *Database) affinityAmounts(recordID, pointsPrefix, namesPrefix string) (character.AffinityAmounts, error) {
	result := character.AffinityAmounts{}
	values := make([]struct {
		name   string
		points uint32
	}, 0, 3)
	complete := true
	hasFields := false
	for index := 1; index <= 3; index++ {
		pointsField := fmt.Sprintf("%s%d", pointsPrefix, index)
		namesField := fmt.Sprintf("%s%d", namesPrefix, index)
		points, pointsFound, err := d.Int32Field(recordID, pointsField)
		if err != nil {
			return character.AffinityAmounts{}, fmt.Errorf("read %s%d: %w", pointsPrefix, index, err)
		}
		name, nameFound, err := d.StringField(recordID, namesField)
		if err != nil {
			return character.AffinityAmounts{}, fmt.Errorf("read %s%d: %w", namesPrefix, index, err)
		}
		if !pointsFound {
			complete = false
			continue
		}
		hasFields = true
		if points < 0 {
			return character.AffinityAmounts{}, fmt.Errorf("field %s%d is negative", pointsPrefix, index)
		}
		if points == 0 && (!nameFound || name == "") {
			continue
		}
		if !nameFound || name == "" {
			complete = false
			continue
		}
		var affinityCheck character.AffinityAmounts
		if !affinityCheck.Set(character.Affinity(name), uint32(points)) {
			complete = false
		}
		values = append(values, struct {
			name   string
			points uint32
		}{name: name, points: uint32(points)})
	}
	if complete && hasFields {
		result = character.KnownZeroAffinityAmounts()
	}
	for _, value := range values {
		result.Set(character.Affinity(value.name), value.points)
	}
	return result, nil
}

func mergeAffinityAmounts(
	current, candidate character.AffinityAmounts,
	conflicts map[character.Affinity]bool,
) character.AffinityAmounts {
	merge := func(affinity character.Affinity, current, candidate character.AffinityValue) character.AffinityValue {
		if conflicts[affinity] {
			return character.AffinityValue{}
		}
		switch {
		case !current.Known && candidate.Known:
			return candidate
		case current.Known && candidate.Known && current.Points != candidate.Points:
			conflicts[affinity] = true
			return character.AffinityValue{}
		}
		return current
	}
	return character.AffinityAmounts{
		Ascendant:  merge(character.AffinityAscendant, current.Ascendant, candidate.Ascendant),
		Chaos:      merge(character.AffinityChaos, current.Chaos, candidate.Chaos),
		Eldritch:   merge(character.AffinityEldritch, current.Eldritch, candidate.Eldritch),
		Order:      merge(character.AffinityOrder, current.Order, candidate.Order),
		Primordial: merge(character.AffinityPrimordial, current.Primordial, candidate.Primordial),
	}
}
