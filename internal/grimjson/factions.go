package grimjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DryHumour/grimroster/internal/character"
)

type factionBlock struct {
	Version       uint32          `json:"Version"`
	Faction       *uint32         `json:"Faction"`
	Factions      []*factionValue `json:"Factions"`
	DecodingError *string         `json:"DecodingError"`
}

type factionValue struct {
	IsChanged     *bool    `json:"IsChanged"`
	IsUnlocked    *bool    `json:"IsUnlocked"`
	Value         *float32 `json:"Value"`
	PositiveBoost *float32 `json:"PositiveBoost"`
	NegativeBoost *float32 `json:"NegativeBoost"`
}

func decodeFactionBlock(raw json.RawMessage) ([]character.FactionReputation, *character.FactionIdentity, bool, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil, false, nil
	}

	var block factionBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, nil, false, fmt.Errorf("decode Block13 factions: %w", err)
	}
	if block.DecodingError != nil && *block.DecodingError != "" {
		return nil, nil, false, nil
	}
	var myFaction *character.FactionIdentity
	if block.Faction != nil {
		identity := factionIdentity(block.Version, *block.Faction)
		myFaction = &identity
	}
	if block.Factions == nil {
		return nil, myFaction, false, nil
	}

	reputations := make([]character.FactionReputation, 0, len(block.Factions))
	for index, value := range block.Factions {
		faction := character.FactionReputation{
			Faction:  factionIdentity(block.Version, uint32(index)),
			Standing: character.FactionStanding{Source: character.FactionDataSourceUnknown},
		}
		if value != nil {
			if value.Value != nil {
				faction.Reputation = *value.Value
				faction.ReputationKnown = !(*value.Value == -1 && !boolValue(value.IsUnlocked) && !boolValue(value.IsChanged))
			}
			if value.PositiveBoost != nil {
				faction.PositiveBoost = *value.PositiveBoost
				faction.PositiveBoostKnown = true
			}
			if value.NegativeBoost != nil {
				faction.NegativeBoost = *value.NegativeBoost
				faction.NegativeBoostKnown = true
			}
			if value.IsUnlocked != nil {
				faction.IsUnlocked = *value.IsUnlocked
				faction.IsUnlockedKnown = true
			}
			if value.IsChanged != nil {
				faction.IsChanged = *value.IsChanged
				faction.IsChangedKnown = true
			}
		}
		reputations = append(reputations, faction)
	}

	return reputations, myFaction, true, nil
}

func factionIdentity(version, sourceIndex uint32) character.FactionIdentity {
	identity := character.FactionIdentity{
		SourceIndex: sourceIndex,
		Source:      character.FactionIdentitySourceUnknown,
	}
	id, name, found := factionIDForGDCIndex(version, sourceIndex)
	if !found {
		return identity
	}

	identity.ID = id
	identity.Name = name
	identity.NameTag = factionNameTag(id)
	identity.Known = true
	identity.Source = character.FactionIdentitySourceGrimSchema
	return identity
}

func factionIDForGDCIndex(version, sourceIndex uint32) (string, string, bool) {
	// Block13 v5 slots are translated to canonical identities before joining
	// to ARZ; sourceIndex remains provenance and is never used as the faction ID.
	if version != 5 {
		return "", "", false
	}
	switch sourceIndex {
	case 0:
		return "Player", "Player", true
	case 1:
		return "Survivors", "Devil's Crossing", true
	case 2:
		return "Aetherials", "Aetherials", true
	case 3:
		return "Cthonians", "Chthonians", true
	case 4:
		return "Outlaws", "Cronley's Gang", true
	case 5:
		return "Beasts", "Beasts", true
	default:
		if sourceIndex >= 6 {
			id := fmt.Sprintf("User%d", sourceIndex-6)
			return id, id, true
		}
		return "", "", false
	}
}

func factionNameTag(id string) string {
	const userPrefix = "User"
	if !strings.HasPrefix(id, userPrefix) || len(id) == len(userPrefix) {
		return ""
	}
	if _, err := strconv.ParseUint(strings.TrimPrefix(id, userPrefix), 10, 32); err != nil {
		return ""
	}
	return "tagFaction" + id
}

func boolValue(value *bool) bool {
	return value != nil && *value
}
