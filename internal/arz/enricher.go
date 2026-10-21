package arz

import "github.com/DryHumour/grimroster/internal/character"

type Enricher interface {
	Enrich(character.Character) (character.Character, error)
}
