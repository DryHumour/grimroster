package arz

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type DevotionConstellation struct {
	DisplayTag string
	Bitmap     string
	Stars      map[uint32][]string
}

func (d *Database) DevotionConstellations() ([]DevotionConstellation, error) {
	constellationPrefix := "records/ui/skills/devotion/constellations/constellation"
	constellations := make(map[string]*DevotionConstellation)
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
		constellation := constellations[displayTag]
		if constellation == nil {
			constellation = &DevotionConstellation{
				DisplayTag: displayTag,
				Stars:      make(map[uint32][]string),
			}
			constellations[displayTag] = constellation
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
