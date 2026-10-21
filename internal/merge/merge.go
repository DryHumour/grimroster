package merge

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/DryHumour/grimroster/internal/arz"
	"github.com/DryHumour/grimroster/internal/character"
	"github.com/DryHumour/grimroster/internal/gdedit"
	"github.com/DryHumour/grimroster/internal/grimjson"
)

func ReadFiles(grimJSONPath, gdEditHTMLPath string) (character.Character, error) {
	return ReadFilesWithDatabase(grimJSONPath, gdEditHTMLPath, nil)
}

func ReadFilesWithGameDir(grimJSONPath, gdEditHTMLPath, gameDir string) (character.Character, error) {
	database, err := arz.OpenGameDir(gameDir)
	if err != nil {
		return character.Character{}, fmt.Errorf("open Grim Dawn database from %q: %w", gameDir, err)
	}
	return ReadFilesWithDatabase(grimJSONPath, gdEditHTMLPath, database)
}

func ReadFilesWithDatabase(grimJSONPath, gdEditHTMLPath string, database *arz.Database) (character.Character, error) {
	grimData, err := os.ReadFile(grimJSONPath)
	if err != nil {
		return character.Character{}, fmt.Errorf("read grim JSON %q: %w", grimJSONPath, err)
	}
	grim, err := grimjson.Decode(grimData)
	if err != nil {
		return character.Character{}, fmt.Errorf("load grim JSON %q: %w", grimJSONPath, err)
	}

	htmlData, err := os.ReadFile(gdEditHTMLPath)
	if err != nil {
		return character.Character{}, fmt.Errorf("read gd-edit HTML %q: %w", gdEditHTMLPath, err)
	}
	sheet, err := gdedit.DecodeHTML(htmlData)
	if err != nil {
		return character.Character{}, fmt.Errorf("load gd-edit HTML %q: %w", gdEditHTMLPath, err)
	}

	return MergeWithDatabase(grim, sheet, database)
}

func Merge(grim character.Character, sheet gdedit.Data) (character.Character, error) {
	return MergeWithDatabase(grim, sheet, nil)
}

func MergeWithDatabase(grim character.Character, sheet gdedit.Data, database *arz.Database) (character.Character, error) {
	result := grim
	result.Masteries = slices.Clone(grim.Masteries)
	result.Skills = slices.Clone(grim.Skills)
	for i := range result.Skills {
		result.Skills[i].Current = slices.Clone(grim.Skills[i].Current)
		result.Skills[i].Next = slices.Clone(grim.Skills[i].Next)
	}
	result.Devotions = slices.Clone(grim.Devotions)
	for i := range result.Devotions {
		result.Devotions[i].Stars = slices.Clone(grim.Devotions[i].Stars)
	}
	result.Diagnostics = slices.Clone(grim.Diagnostics)
	result.Inventory.Equipment = slices.Clone(grim.Inventory.Equipment)
	result.Inventory.Bags = slices.Clone(grim.Inventory.Bags)
	for i := range result.Inventory.Bags {
		result.Inventory.Bags[i].Items = slices.Clone(grim.Inventory.Bags[i].Items)
	}

	if grim.Name != "" && sheet.Name != "" && grim.Name != sheet.Name {
		if err := addDiagnostic(
			&result,
			character.SeverityError,
			character.DiagnosticCodeCharacterNameMismatch,
			"Character name differs between grim and gd-edit.",
			"name",
			grim.Name,
			sheet.Name,
		); err != nil {
			return character.Character{}, err
		}
	} else if result.Name == "" {
		result.Name = sheet.Name
	}
	if grim.Level != 0 && sheet.Level != 0 && grim.Level != sheet.Level {
		if err := addDiagnostic(
			&result,
			character.SeverityError,
			character.DiagnosticCodeCharacterLevelMismatch,
			"Character level differs between grim and gd-edit.",
			"level",
			grim.Level,
			sheet.Level,
		); err != nil {
			return character.Character{}, err
		}
	} else if result.Level == 0 {
		result.Level = sheet.Level
	}

	if result.ClassName == "" && len(sheet.Masteries) > 0 {
		result.ClassName = strings.Join(sheet.Masteries, " + ")
	}
	if len(sheet.Masteries) > 0 {
		result.Masteries = slices.Clone(sheet.Masteries)
	}
	if sheet.Expansion != "" {
		result.Expansion = sheet.Expansion
	}
	if sheet.Difficulty != "" {
		result.Difficulty = sheet.Difficulty
	}

	calculated := sheet.Calculated
	calculated.ArmorBySlot = maps.Clone(sheet.Calculated.ArmorBySlot)
	calculated.Resistances = slices.Clone(sheet.Calculated.Resistances)
	result.Calculated = &calculated

	if err := mergeSkills(&result, sheet.Skills, database); err != nil {
		return character.Character{}, err
	}
	if err := mergeDevotions(&result, sheet.Devotions, database); err != nil {
		return character.Character{}, err
	}

	used := make([]bool, len(result.Inventory.Equipment))
	for _, observation := range sheet.Equipment {
		index := matchEquipment(result.Inventory.Equipment, used, observation)
		if index < 0 {
			if err := addRawDiagnostic(
				&result,
				character.SeverityWarning,
				character.DiagnosticCodeEquipmentUnmatched,
				"gd-edit equipment item did not match a grim equipment item.",
				equipmentField(observation),
				jsontext.Value("null"),
				observation.Raw,
			); err != nil {
				return character.Character{}, err
			}
			continue
		}
		used[index] = true
		item := &result.Inventory.Equipment[index].Item
		if observedSlot := equipmentSlot(observation.Slot); observedSlot != "" && result.Inventory.Equipment[index].Slot != observedSlot {
			if err := addDiagnostic(
				&result,
				character.SeverityWarning,
				character.DiagnosticCodeEquipmentSlotMismatch,
				"Equipment slot differs between grim and gd-edit.",
				equipmentField(observation)+".slot",
				result.Inventory.Equipment[index].Slot,
				observation.Slot,
			); err != nil {
				return character.Character{}, err
			}
		}
		if item.DisplayName != "" && item.DisplayName != observation.Name {
			if err := addDiagnostic(
				&result,
				character.SeverityWarning,
				character.DiagnosticCodeEquipmentDisplayNameMismatch,
				"Equipment display name differs between grim and gd-edit.",
				equipmentField(observation)+".display_name",
				item.DisplayName,
				observation.Name,
			); err != nil {
				return character.Character{}, err
			}
		} else if item.DisplayName == "" {
			item.DisplayName = observation.Name
		}
		item.GDEditDetails = observation.Raw.Clone()
	}

	if database != nil {
		enriched, err := database.Enrich(result)
		if err != nil {
			return character.Character{}, fmt.Errorf("enrich character items from ARZ database: %w", err)
		}
		result = enriched
	}

	return result, nil
}

func mergeSkills(result *character.Character, observations []gdedit.SkillObservation, database *arz.Database) error {
	matched := make([]bool, len(observations))
	ambiguousObservations := make([]bool, len(observations))
	for i := range result.Skills {
		skill := &result.Skills[i]
		if skill.Category == "" {
			skill.Category = character.ClassifySkillCategory(skill.RecordID, skill.IsDevotion)
		}
		if database == nil {
			skill.MatchStatus = character.SkillMatchStatusNoDatabase
			continue
		}
		if !database.HasRecord(skill.RecordID) {
			skill.MatchStatus = character.SkillMatchStatusNoARZRecord
			continue
		}
		textures, err := skillTextures(database, skill.RecordID)
		if err != nil {
			return fmt.Errorf("resolve skill texture for %q: %w", skill.RecordID, err)
		}
		if len(textures) == 0 {
			skill.MatchStatus = character.SkillMatchStatusNoJoinKey
			continue
		}
		skill.MatchStatus = character.SkillMatchStatusNoHTMLNode
	}

	// Stable record IDs take precedence when newer gd-edit exports include them.
	grimByID := make(map[string][]int)
	sheetByID := make(map[string][]int)
	for i := range result.Skills {
		if recordID := strings.ToLower(result.Skills[i].RecordID); recordID != "" {
			grimByID[recordID] = append(grimByID[recordID], i)
		}
	}
	for i, observation := range observations {
		if recordID := strings.ToLower(observation.RecordID); recordID != "" {
			sheetByID[recordID] = append(sheetByID[recordID], i)
		}
	}
	for sheetIndex, observation := range observations {
		recordID := strings.ToLower(observation.RecordID)
		if recordID == "" {
			continue
		}
		sheetIndices := sheetByID[recordID]
		if sheetIndex != sheetIndices[0] {
			continue
		}
		grimIndices := grimByID[recordID]
		if len(sheetIndices) != 1 || len(grimIndices) != 1 {
			for _, sheetIndex := range sheetIndices {
				ambiguousObservations[sheetIndex] = true
			}
			for _, grimIndex := range grimIndices {
				result.Skills[grimIndex].MatchStatus = character.SkillMatchStatusAmbiguous
			}
			continue
		}
		grimIndex := grimIndices[0]
		if matched[sheetIndex] {
			continue
		}
		if err := applySkillObservation(result, grimIndex, observations[sheetIndex], character.SkillMatchStatusMatchedRecordID); err != nil {
			return err
		}
		matched[sheetIndex] = true
	}

	sheetSkillsByTexture := make(map[string][]int)
	for index, observation := range observations {
		if matched[index] || observation.JoinKey == "" {
			continue
		}
		texture := normalizeTexture(observation.JoinKey)
		if texture != "" {
			sheetSkillsByTexture[texture] = append(sheetSkillsByTexture[texture], index)
		}
	}

	if database != nil {
		grimSkillsByTexture := make(map[string][]int)
		for i := range result.Skills {
			if result.Skills[i].MatchStatus == character.SkillMatchStatusMatchedRecordID {
				continue
			}
			textures, err := skillTextures(database, result.Skills[i].RecordID)
			if err != nil {
				return fmt.Errorf("resolve skill texture for %q: %w", result.Skills[i].RecordID, err)
			}
			for _, texture := range textures {
				key := normalizeTexture(texture)
				if key != "" && !slices.Contains(grimSkillsByTexture[key], i) {
					grimSkillsByTexture[key] = append(grimSkillsByTexture[key], i)
				}
			}
		}

		grimCandidates := make([]map[int]struct{}, len(result.Skills))
		sheetCandidates := make([]map[int]struct{}, len(observations))
		for texture, grimIndices := range grimSkillsByTexture {
			for _, grimIndex := range grimIndices {
				for _, sheetIndex := range sheetSkillsByTexture[texture] {
					if grimCandidates[grimIndex] == nil {
						grimCandidates[grimIndex] = make(map[int]struct{})
					}
					if sheetCandidates[sheetIndex] == nil {
						sheetCandidates[sheetIndex] = make(map[int]struct{})
					}
					grimCandidates[grimIndex][sheetIndex] = struct{}{}
					sheetCandidates[sheetIndex][grimIndex] = struct{}{}
				}
			}
		}
		for grimIndex, candidates := range grimCandidates {
			if len(candidates) > 1 {
				result.Skills[grimIndex].MatchStatus = character.SkillMatchStatusAmbiguous
			}
		}
		for sheetIndex, candidates := range sheetCandidates {
			if matched[sheetIndex] || len(candidates) == 0 {
				continue
			}
			if len(candidates) != 1 {
				for grimIndex := range candidates {
					result.Skills[grimIndex].MatchStatus = character.SkillMatchStatusAmbiguous
				}
				continue
			}
			var grimIndex int
			for candidate := range candidates {
				grimIndex = candidate
			}
			if len(grimCandidates[grimIndex]) != 1 {
				result.Skills[grimIndex].MatchStatus = character.SkillMatchStatusAmbiguous
				continue
			}
			if err := applySkillObservation(result, grimIndex, observations[sheetIndex], character.SkillMatchStatusMatchedTexture); err != nil {
				return err
			}
			matched[sheetIndex] = true
		}
		for sheetIndex, candidates := range sheetCandidates {
			if !matched[sheetIndex] && len(candidates) > 0 {
				ambiguousObservations[sheetIndex] = true
			}
		}
	}

	for index, observation := range observations {
		if matched[index] {
			continue
		}
		status := character.SkillMatchStatusUnusedHTMLNode
		switch {
		case database == nil:
			status = character.SkillMatchStatusNoDatabase
		case ambiguousObservations[index]:
			status = character.SkillMatchStatusAmbiguous
		case observation.JoinKey == "":
			status = character.SkillMatchStatusNoJoinKey
		}
		if database != nil && observation.Mastery != "" && observation.Level > 0 {
			if err := addDiagnostic(
				result,
				character.SeverityWarning,
				character.DiagnosticCodeSkillUnmatchedInvested,
				"Invested gd-edit mastery skill could not be correlated to a grim skill.",
				fmt.Sprintf("skills[%q:%q].level", observation.Mastery, observation.Name),
				nil,
				observation.Level,
			); err != nil {
				return err
			}
		}
		result.Skills = append(result.Skills, character.Skill{
			RecordID:      observation.RecordID,
			Name:          observation.Name,
			Mastery:       observation.Mastery,
			Category:      character.ClassifySkillCategory(observation.RecordID, false),
			GDEditLevel:   observation.Level,
			GearBonus:     observation.GearBonus,
			MaxLevel:      observation.MaxLevel,
			UltimateLevel: observation.UltimateLevel,
			Description:   observation.Description,
			Current:       slices.Clone(observation.Current),
			Next:          slices.Clone(observation.Next),
			GDEditOnly:    true,
			MatchStatus:   status,
		})
		if result.Skills[len(result.Skills)-1].Category == character.SkillCategoryOther {
			result.Skills[len(result.Skills)-1].Category = character.SkillCategoryMastery
		}
	}

	resolveMasteryBars(result)
	if database == nil {
		for i := range result.Skills {
			if result.Skills[i].Name == "" && result.Skills[i].RecordID != "" {
				result.Skills[i].NameResolution = character.SkillNameResolutionNoDatabase
			}
		}
	}
	return nil
}

func applySkillObservation(result *character.Character, skillIndex int, observation gdedit.SkillObservation, status character.SkillMatchStatus) error {
	skill := &result.Skills[skillIndex]
	skill.Name = observation.Name
	skill.Mastery = observation.Mastery
	skill.GDEditLevel = observation.Level
	skill.GearBonus = observation.GearBonus
	skill.MaxLevel = observation.MaxLevel
	skill.UltimateLevel = observation.UltimateLevel
	skill.Description = observation.Description
	skill.Current = slices.Clone(observation.Current)
	skill.Next = slices.Clone(observation.Next)
	skill.MatchStatus = status
	if skill.Level != observation.Level {
		if err := addDiagnostic(
			result,
			character.SeverityWarning,
			character.DiagnosticCodeSkillLevelMismatch,
			"Skill level differs between grim and gd-edit.",
			"skills["+skill.RecordID+"].level",
			skill.Level,
			observation.Level,
		); err != nil {
			return err
		}
	}
	return nil
}

func resolveMasteryBars(result *character.Character) {
	// A matched skill's playerclass directory ties its gd-edit mastery label to its save bar.
	masteryByClass := make(map[string]string)
	for _, skill := range result.Skills {
		if skill.Category != character.SkillCategoryMastery || skill.Mastery == "" {
			continue
		}
		classID := skillClassDirectory(skill.RecordID)
		if classID != "" && masteryByClass[classID] == "" {
			masteryByClass[classID] = skill.Mastery
		}
	}
	for i := range result.Skills {
		skill := &result.Skills[i]
		if skill.Category != character.SkillCategoryMasteryBar {
			continue
		}
		mastery := masteryByClass[skillClassDirectory(skill.RecordID)]
		if mastery == "" {
			mastery = fallbackMasteryName(skill.RecordID)
		}
		if mastery == "" {
			continue
		}
		skill.Mastery = mastery
		if skill.Name == "" {
			skill.Name = mastery
		}
	}
}

func skillClassDirectory(recordID string) string {
	const skillsPrefix = "records/skills/"
	normalized := strings.ToLower(recordID)
	if !strings.HasPrefix(normalized, skillsPrefix) {
		return ""
	}
	remaining := strings.TrimPrefix(normalized, skillsPrefix)
	classDirectory, _, found := strings.Cut(remaining, "/")
	if !found || !strings.HasPrefix(classDirectory, "playerclass") {
		return ""
	}
	return classDirectory
}

func fallbackMasteryName(recordID string) string {
	switch skillClassDirectory(recordID) {
	case "playerclass01":
		return "Soldier"
	case "playerclass02":
		return "Demolitionist"
	case "playerclass03":
		return "Occultist"
	case "playerclass04":
		return "Nightblade"
	case "playerclass05":
		return "Arcanist"
	case "playerclass06":
		return "Shaman"
	case "playerclass07":
		return "Inquisitor"
	case "playerclass08":
		return "Necromancer"
	case "playerclass09":
		return "Oathkeeper"
	default:
		return ""
	}
}

func mergeDevotions(result *character.Character, observations []gdedit.Devotion, database *arz.Database) error {
	constellationsByBitmap := make(map[string][]arz.DevotionConstellation)
	if database != nil {
		constellations, err := database.DevotionConstellations()
		if err != nil {
			return fmt.Errorf("read devotion constellation records: %w", err)
		}
		for _, constellation := range constellations {
			bitmap := normalizeTexture(constellation.Bitmap)
			if bitmap != "" {
				constellationsByBitmap[bitmap] = append(constellationsByBitmap[bitmap], constellation)
			}
		}
	}

	skillsByRecordID := make(map[string]*character.Skill, len(result.Skills))
	boundSkills := make(map[string][]string)
	for i := range result.Skills {
		skill := &result.Skills[i]
		if skill.RecordID != "" {
			skillsByRecordID[strings.ToLower(skill.RecordID)] = skill
		}
		if skill.AutocastRecordID != "" && skill.RecordID != "" {
			key := strings.ToLower(skill.AutocastRecordID)
			if !slices.Contains(boundSkills[key], skill.RecordID) {
				boundSkills[key] = append(boundSkills[key], skill.RecordID)
			}
		}
	}

	for _, observation := range observations {
		devotion := character.Devotion{
			Name:       observation.Name,
			StarsTaken: observation.StarsTaken,
			StarsTotal: observation.StarsTotal,
			Complete:   observation.Complete,
			Stars:      make([]character.DevotionStar, 0, len(observation.Stars)),
		}
		constellations := constellationsByBitmap[normalizeTexture(observation.JoinKey)]
		for _, observedStar := range observation.Stars {
			star := character.DevotionStar{
				Name:       observedStar.Name,
				Button:     observedStar.Button,
				Skill:      observedStar.Skill,
				Taken:      observedStar.Taken,
				BoundSkill: observedStar.Bound,
				Trigger:    observedStar.Trigger,
			}
			recordID, found := matchDevotionRecord(constellations, observedStar.Button, skillsByRecordID)
			if found {
				star.RecordID = recordID
				skill := skillsByRecordID[strings.ToLower(recordID)]
				grimTaken := false
				if skill != nil {
					star.Level = skill.Level
					star.DevotionLevel = skill.DevotionLevel
					star.DevotionExperience = skill.DevotionExperience
					grimTaken = skill.Enabled && skill.Level > 0 && skill.DevotionLevel > 0
					skill.IsDevotion = true
					if skill.Name == "" {
						skill.Name = star.Name
					}
				}
				if observedStar.Taken != grimTaken {
					if err := addDiagnostic(
						result,
						character.SeverityWarning,
						character.DiagnosticCodeDevotionStateMismatch,
						"Devotion star state differs between grim and gd-edit.",
						fmt.Sprintf("devotions[%q].stars[%d].taken", observation.Name, observedStar.Button),
						grimTaken,
						observedStar.Taken,
					); err != nil {
						return err
					}
				}
				star.Taken = grimTaken

				bound := boundSkills[strings.ToLower(recordID)]
				switch len(bound) {
				case 1:
					star.BoundSkillRecordID = bound[0]
					if boundSkill := skillsByRecordID[strings.ToLower(bound[0])]; boundSkill != nil {
						if star.BoundSkill == "" {
							star.BoundSkill = boundSkill.Name
						} else if boundSkill.Name != "" && !strings.EqualFold(star.BoundSkill, boundSkill.Name) {
							if err := addDiagnostic(
								result,
								character.SeverityWarning,
								character.DiagnosticCodeDevotionBindingMismatch,
								"Bound skill differs between grim and gd-edit.",
								fmt.Sprintf("devotions[%q].stars[%d].bound_skill", observation.Name, observedStar.Button),
								boundSkill.Name,
								star.BoundSkill,
							); err != nil {
								return err
							}
						}
					}
				case 0:
					if star.BoundSkill != "" {
						if err := addDiagnostic(
							result,
							character.SeverityWarning,
							character.DiagnosticCodeDevotionBindingMismatch,
							"gd-edit reports a bound skill that grim does not.",
							fmt.Sprintf("devotions[%q].stars[%d].bound_skill", observation.Name, observedStar.Button),
							"",
							star.BoundSkill,
						); err != nil {
							return err
						}
					}
				default:
					if err := addDiagnostic(
						result,
						character.SeverityWarning,
						character.DiagnosticCodeDevotionBindingMismatch,
						"Grim reports multiple skills bound to one devotion star.",
						fmt.Sprintf("devotions[%q].stars[%d].bound_skill", observation.Name, observedStar.Button),
						bound,
						star.BoundSkill,
					); err != nil {
						return err
					}
				}
			}
			devotion.Stars = append(devotion.Stars, star)
		}

		devotion.StarsTaken = 0
		for _, star := range devotion.Stars {
			if star.Taken {
				devotion.StarsTaken++
			}
		}
		if devotion.StarsTotal == 0 {
			devotion.StarsTotal = uint32(len(devotion.Stars))
		}
		devotion.Complete = devotion.StarsTotal > 0 && devotion.StarsTaken == devotion.StarsTotal
		result.Devotions = append(result.Devotions, devotion)
	}
	return nil
}

func matchDevotionRecord(constellations []arz.DevotionConstellation, button uint32, skills map[string]*character.Skill) (string, bool) {
	recordIDs := make([]string, 0)
	for _, constellation := range constellations {
		for _, recordID := range constellation.Stars[button] {
			if !slices.Contains(recordIDs, recordID) {
				recordIDs = append(recordIDs, recordID)
			}
		}
	}
	if len(recordIDs) == 1 {
		return recordIDs[0], true
	}

	active := make([]string, 0)
	for _, recordID := range recordIDs {
		skill := skills[strings.ToLower(recordID)]
		if skill != nil && skill.Enabled && skill.Level > 0 && skill.DevotionLevel > 0 {
			active = append(active, recordID)
		}
	}
	if len(active) == 1 {
		return active[0], true
	}
	return "", false
}

func skillTextures(database *arz.Database, recordID string) ([]string, error) {
	pending := []string{recordID}
	visited := make(map[string]bool)
	textures := make([]string, 0)
	for depth := 0; len(pending) > 0 && depth < 8; depth++ {
		current := pending[0]
		pending = pending[1:]
		key := strings.ToLower(current)
		if current == "" || visited[key] {
			continue
		}
		visited[key] = true

		values, found, err := database.StringValuesField(current, "skillUpBitmapName")
		if err != nil {
			return nil, fmt.Errorf("read skillUpBitmapName from %q: %w", current, err)
		}
		if found {
			for _, texture := range values {
				if texture != "" && !slices.Contains(textures, texture) {
					textures = append(textures, texture)
				}
			}
			if len(textures) > 0 {
				continue
			}
		}
		for _, fieldName := range []string{"buffSkillName", "petSkillName"} {
			next, found, err := database.StringField(current, fieldName)
			if err != nil {
				return nil, fmt.Errorf("read %s from %q: %w", fieldName, current, err)
			}
			if found && next != "" {
				pending = append(pending, next)
			}
		}
	}
	return textures, nil
}

func normalizeTexture(texture string) string {
	texture = strings.ToLower(strings.TrimSpace(texture))
	return strings.TrimPrefix(texture, "ui/")
}

func matchEquipment(equipment []character.EquippedItem, used []bool, observation gdedit.Equipment) int {
	if observation.Seed != 0 {
		for index, equipped := range equipment {
			if !used[index] && equipped.Item.Seed == observation.Seed {
				return index
			}
		}
		return -1
	}

	slot := equipmentSlot(observation.Slot)
	if slot == "" {
		return -1
	}
	match := -1
	for index, equipped := range equipment {
		if used[index] || equipped.Slot != slot {
			continue
		}
		if match >= 0 {
			return -1
		}
		match = index
	}
	return match
}

func equipmentSlot(slot string) character.EquipmentSlot {
	switch strings.ToLower(slot) {
	case "head":
		return character.SlotHead
	case "amulet", "necklace":
		return character.SlotNecklace
	case "chest", "torso":
		return character.SlotTorso
	case "legs":
		return character.SlotLegs
	case "feet":
		return character.SlotFeet
	case "hands", "arms":
		return character.SlotHands
	case "belt", "waist":
		return character.SlotWaist
	case "shoulders":
		return character.SlotShoulders
	case "medal":
		return character.SlotMedal
	case "relic":
		return character.SlotRelic
	case "weapon":
		return character.SlotRightHand1
	case "off-hand", "offhand":
		return character.SlotLeftHand1
	case "right ring":
		return character.SlotRightRing
	case "left ring":
		return character.SlotLeftRing
	default:
		return ""
	}
}

func equipmentField(observation gdedit.Equipment) string {
	return fmt.Sprintf("inventory.equipment[slot=%q,seed=%d]", observation.Slot, observation.Seed)
}

func addDiagnostic(result *character.Character, severity character.Severity, code, message, field string, grimValue, gdEditValue any) error {
	grimJSON, err := jsonv2.Marshal(grimValue)
	if err != nil {
		return fmt.Errorf("encode grim diagnostic value for %q: %w", field, err)
	}
	gdEditJSON, err := jsonv2.Marshal(gdEditValue)
	if err != nil {
		return fmt.Errorf("encode gd-edit diagnostic value for %q: %w", field, err)
	}
	result.Diagnostics = append(result.Diagnostics, character.Diagnostic{
		Severity:    severity,
		Code:        code,
		Message:     message,
		Field:       field,
		GrimValue:   jsontext.Value(grimJSON),
		GDEditValue: jsontext.Value(gdEditJSON),
	})
	return nil
}

func addRawDiagnostic(result *character.Character, severity character.Severity, code, message, field string, grimValue, gdEditValue jsontext.Value) error {
	if !grimValue.IsValid() {
		return fmt.Errorf("invalid grim diagnostic JSON for %q", field)
	}
	if !gdEditValue.IsValid() {
		return fmt.Errorf("invalid gd-edit diagnostic JSON for %q", field)
	}
	result.Diagnostics = append(result.Diagnostics, character.Diagnostic{
		Severity:    severity,
		Code:        code,
		Message:     message,
		Field:       field,
		GrimValue:   grimValue.Clone(),
		GDEditValue: gdEditValue.Clone(),
	})
	return nil
}
