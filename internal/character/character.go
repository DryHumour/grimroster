package character

import (
	"encoding/json/jsontext"
	"strings"
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type Character struct {
	Name        string       `json:"name"`
	ClassID     string       `json:"class_id"`
	ClassName   string       `json:"class_name,omitempty"`
	ClassTag    string       `json:"class_tag,omitempty"`
	Masteries   []string     `json:"masteries,omitempty"`
	Level       uint32       `json:"level"`
	Gender      Gender       `json:"gender"`
	Hardcore    bool         `json:"hardcore"`
	Expansions  uint8        `json:"expansions"`
	Expansion   string       `json:"expansion,omitempty"`
	Difficulty  string       `json:"difficulty,omitempty"`
	Progress    Progression  `json:"progression"`
	Inventory   Inventory    `json:"inventory"`
	Skills      []Skill      `json:"skills,omitempty"`
	Devotions   []Devotion   `json:"devotions,omitempty"`
	Calculated  *Calculated  `json:"calculated,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type Progression struct {
	Experience      uint32     `json:"experience"`
	AttributePoints uint32     `json:"attribute_points"`
	SkillPoints     uint32     `json:"skill_points"`
	DevotionPoints  uint32     `json:"devotion_points"`
	TotalDevotion   uint32     `json:"total_devotion_points"`
	Attributes      Attributes `json:"attributes"`
}

type Attributes struct {
	Physique float32 `json:"physique"`
	Cunning  float32 `json:"cunning"`
	Spirit   float32 `json:"spirit"`
	Health   float32 `json:"health"`
	Energy   float32 `json:"energy"`
}

type Calculated struct {
	Physique         float64            `json:"physique"`
	Cunning          float64            `json:"cunning"`
	Spirit           float64            `json:"spirit"`
	Health           float64            `json:"health"`
	Energy           float64            `json:"energy"`
	EnergyReserved   float64            `json:"energy_reserved"`
	EnergyUsable     float64            `json:"energy_usable"`
	OffensiveAbility float64            `json:"offensive_ability"`
	DefensiveAbility float64            `json:"defensive_ability"`
	Armor            float64            `json:"armour"`
	ArmorBySlot      map[string]float64 `json:"armour_by_slot"`
	Resistances      []Resistance       `json:"resistances"`
}

type Resistance struct {
	Field string  `json:"field"`
	Label string  `json:"label"`
	Shown float64 `json:"shown"`
	Cap   float64 `json:"cap"`
	Over  float64 `json:"over"`
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

const (
	DiagnosticCodeCharacterNameMismatch        = "character.name_mismatch"
	DiagnosticCodeCharacterLevelMismatch       = "character.level_mismatch"
	DiagnosticCodeGrimBlockDecodingError       = "grim.block_decoding_error"
	DiagnosticCodeGrimBlockMissing             = "grim.block_missing"
	DiagnosticCodeEquipmentUnmatched           = "equipment.unmatched"
	DiagnosticCodeEquipmentSlotMismatch        = "equipment.slot_mismatch"
	DiagnosticCodeEquipmentDisplayNameMismatch = "equipment.display_name_mismatch"
	DiagnosticCodeSkillLevelMismatch           = "skill.level_mismatch"
	DiagnosticCodeSkillUnmatchedInvested       = "skill.unmatched_invested"
	DiagnosticCodeDevotionStateMismatch        = "devotion.star_state_mismatch"
	DiagnosticCodeDevotionBindingMismatch      = "devotion.binding_mismatch"
)

type Diagnostic struct {
	Severity    Severity       `json:"severity"`
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Field       string         `json:"field"`
	GrimValue   jsontext.Value `json:"grim_value"`
	GDEditValue jsontext.Value `json:"gd_edit_value"`
}

type Inventory struct {
	Equipment []EquippedItem `json:"equipment"`
	Bags      []Bag          `json:"bags"`
}

type Skill struct {
	RecordID           string              `json:"record_id,omitempty"`
	Name               string              `json:"name,omitempty"`
	DisplayTag         string              `json:"display_tag,omitempty"`
	NameResolution     SkillNameResolution `json:"name_resolution,omitempty"`
	Mastery            string              `json:"mastery,omitempty"`
	Category           SkillCategory       `json:"category"`
	Level              uint32              `json:"level"`
	LevelObserved      bool                `json:"level_observed"`
	IsDevotion         bool                `json:"is_devotion"`
	DevotionLevel      uint32              `json:"devotion_level"`
	DevotionExperience uint32              `json:"devotion_experience"`
	Sublevel           uint32              `json:"sublevel"`
	Enabled            bool                `json:"enabled"`
	EnabledObserved    bool                `json:"enabled_observed"`
	Aura               bool                `json:"aura"`
	Locked             bool                `json:"locked"`
	AutocastRecordID   string              `json:"autocast_record_id,omitempty"`
	ControllerRecordID string              `json:"controller_record_id,omitempty"`
	GDEditLevel        uint32              `json:"gd_edit_level,omitzero"`
	GearBonus          uint32              `json:"gear_bonus,omitzero"`
	MaxLevel           uint32              `json:"max_level,omitzero"`
	UltimateLevel      uint32              `json:"ultimate_level,omitzero"`
	Description        string              `json:"description,omitempty"`
	Current            []string            `json:"current,omitempty"`
	Next               []string            `json:"next,omitempty"`
	SaveBacked         bool                `json:"save_backed"`
	GDEditOnly         bool                `json:"gd_edit_only,omitzero"`
	MatchStatus        SkillMatchStatus    `json:"match_status,omitempty"`
}

type SkillCategory string

func ClassifySkillCategory(recordID string, isDevotion bool) SkillCategory {
	id := strings.ToLower(recordID)
	switch {
	case strings.Contains(id, "/potion"):
		return SkillCategoryPotion
	case strings.Contains(id, "/default/"):
		return SkillCategoryDefault
	case isDevotion || strings.Contains(id, "/devotion/"):
		return SkillCategoryDevotion
	case strings.Contains(id, "/_classtraining_"):
		return SkillCategoryMasteryBar
	case strings.Contains(id, "/playerclass"):
		return SkillCategoryMastery
	default:
		return SkillCategoryOther
	}
}

const (
	SkillCategoryMastery    SkillCategory = "mastery_skill"
	SkillCategoryMasteryBar SkillCategory = "mastery_bar"
	SkillCategoryDevotion   SkillCategory = "devotion"
	SkillCategoryDefault    SkillCategory = "default_action"
	SkillCategoryPotion     SkillCategory = "potion_related"
	SkillCategoryOther      SkillCategory = "other"
)

type SkillMatchStatus string

const (
	SkillMatchStatusMatchedRecordID   SkillMatchStatus = "matched_record_id"
	SkillMatchStatusMatchedTexture    SkillMatchStatus = "matched_texture"
	SkillMatchStatusNoDatabase        SkillMatchStatus = "no_database"
	SkillMatchStatusNoARZRecord       SkillMatchStatus = "no_arz_record"
	SkillMatchStatusNoJoinKey         SkillMatchStatus = "no_join_key"
	SkillMatchStatusNoHTMLNode        SkillMatchStatus = "no_html_node"
	SkillMatchStatusAmbiguous         SkillMatchStatus = "ambiguous_candidates"
	SkillMatchStatusRecordIDConflict  SkillMatchStatus = "record_id_conflict"
	SkillMatchStatusUnmatchedInvested SkillMatchStatus = "unmatched_invested"
	SkillMatchStatusUnusedHTMLNode    SkillMatchStatus = "unused_html_node"
)

type SkillNameResolution string

const (
	SkillNameResolutionTagOnly    SkillNameResolution = "tag_only"
	SkillNameResolutionMissing    SkillNameResolution = "missing"
	SkillNameResolutionNoDatabase SkillNameResolution = "no_database"
)

type Devotion struct {
	Name       string         `json:"name"`
	StarsTaken uint32         `json:"stars_taken"`
	StarsTotal uint32         `json:"stars_total"`
	Complete   bool           `json:"complete"`
	Stars      []DevotionStar `json:"stars"`
}

type DevotionStar struct {
	Name               string `json:"name"`
	Button             uint32 `json:"button"`
	Skill              bool   `json:"skill"`
	Taken              bool   `json:"taken"`
	RecordID           string `json:"record_id,omitempty"`
	Level              uint32 `json:"level,omitzero"`
	DevotionLevel      uint32 `json:"devotion_level,omitzero"`
	DevotionExperience uint32 `json:"devotion_experience,omitzero"`
	BoundSkill         string `json:"bound_skill,omitempty"`
	BoundSkillRecordID string `json:"bound_skill_record_id,omitempty"`
	Trigger            string `json:"trigger,omitempty"`
}

type EquipmentSlot string

const (
	SlotHead       EquipmentSlot = "head"
	SlotNecklace   EquipmentSlot = "necklace"
	SlotTorso      EquipmentSlot = "torso"
	SlotLegs       EquipmentSlot = "legs"
	SlotFeet       EquipmentSlot = "feet"
	SlotHands      EquipmentSlot = "hands"
	SlotRightRing  EquipmentSlot = "right_ring"
	SlotLeftRing   EquipmentSlot = "left_ring"
	SlotWaist      EquipmentSlot = "waist"
	SlotShoulders  EquipmentSlot = "shoulders"
	SlotMedal      EquipmentSlot = "medal"
	SlotRelic      EquipmentSlot = "relic"
	SlotRightHand1 EquipmentSlot = "right_hand_1"
	SlotLeftHand1  EquipmentSlot = "left_hand_1"
	SlotRightHand2 EquipmentSlot = "right_hand_2"
	SlotLeftHand2  EquipmentSlot = "left_hand_2"
)

type EquippedItem struct {
	Slot EquipmentSlot `json:"slot"`
	Item Item          `json:"item"`
}

type Bag struct {
	Index          uint32    `json:"index"`
	HasUnseenItems bool      `json:"has_unseen_items"`
	Items          []BagItem `json:"items"`
}

type BagItem struct {
	X    uint32 `json:"x"`
	Y    uint32 `json:"y"`
	Item Item   `json:"item"`
}

type Item struct {
	RecordID        string         `json:"record_id"`
	Seed            uint32         `json:"seed,omitzero"`
	DisplayName     string         `json:"display_name,omitempty"`
	NameTag         string         `json:"name_tag,omitempty"`
	PrefixID        string         `json:"prefix_id,omitempty"`
	SuffixID        string         `json:"suffix_id,omitempty"`
	CraftingBonusID string         `json:"crafting_bonus_id,omitempty"`
	IllusionID      string         `json:"illusion_id,omitempty"`
	ComponentID     string         `json:"component_id,omitempty"`
	RelicBonusID    string         `json:"relic_bonus_id,omitempty"`
	AugmentID       string         `json:"augment_id,omitempty"`
	AscendedAffixID string         `json:"ascended_affix_id,omitempty"`
	Amount          uint32         `json:"amount"`
	GDEditDetails   jsontext.Value `json:"gd_edit_details,omitzero"`
}
