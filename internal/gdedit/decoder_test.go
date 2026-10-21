package gdedit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeHTMLExtractsAndMapsWindowData(t *testing.T) {
	t.Parallel()

	html := []byte(`<html><script>
		window.DATA = {
			"sheet": {
				"character": {"name":"Sheet Name","level":30,"masteries":["Soldier","Arcanist"],"difficulty":"Elite","expansion":"Ashes of Malmouth"},
				"stats": {"physique":300,"cunning":200,"spirit":250,"health":5000,"energy":1500,"energyReserved":100,"energyUsable":1400,"offensiveAbility":1200,"defensiveAbility":1300,"armour":450,"armourBySlot":{"Head":450}},
				"resistances": [{"field":"defensiveFire","label":"Fire","shown":80,"cap":80,"over":12}],
				"equipment": [{"slot":"Head","name":"Test Helm","seed":42,"itemLevel":20},null,{"slot":"Hands","name":""}]
			},
			"tree": {"trees":[{"mastery":"Soldier","nodes":[{"name":"Cadence","level":3,"gear":1,"max":16,"ultimate":22,"desc":"Melee attack.","cur":["Current effect"],"next":["Next effect"],"tex":"ui/skills/icons/cadence.tex"}]}]},
			"devo": {"constellations":[{"name":"Fiend","bitmap":"ui/skills/devotion/fiend.tex","starsTaken":1,"starsTotal":5,"complete":false,"stars":[{"name":"Flame Torrent","button":5,"skill":true,"taken":true,"bound":{"name":"Cadence","trigger":"selfonattack_25%"}},{"name":"Fiend","button":1,"skill":false,"taken":false,"bound":null}]}]}
			}
		};
	</script></html>`)

	got, err := DecodeHTML(html)
	require.NoError(t, err)
	assert.Equal(t, "Sheet Name", got.Name)
	assert.Equal(t, uint32(30), got.Level)
	assert.Equal(t, []string{"Soldier", "Arcanist"}, got.Masteries)
	assert.Equal(t, "Elite", got.Difficulty)
	assert.Equal(t, "Ashes of Malmouth", got.Expansion)
	assert.Equal(t, float64(450), got.Calculated.Armor)
	assert.Equal(t, float64(1400), got.Calculated.EnergyUsable)
	assert.Equal(t, float64(450), got.Calculated.ArmorBySlot["Head"])
	require.Len(t, got.Calculated.Resistances, 1)
	assert.Equal(t, "Fire", got.Calculated.Resistances[0].Label)
	require.Len(t, got.Equipment, 1)
	assert.Equal(t, uint32(42), got.Equipment[0].Seed)
	assert.JSONEq(t, `{"slot":"Head","name":"Test Helm","seed":42,"itemLevel":20}`, string(got.Equipment[0].Raw))
	require.Len(t, got.Skills, 1)
	assert.Equal(t, "Soldier", got.Skills[0].Mastery)
	assert.Equal(t, "Cadence", got.Skills[0].Name)
	assert.Equal(t, uint32(3), got.Skills[0].Level)
	assert.Equal(t, "ui/skills/icons/cadence.tex", got.Skills[0].JoinKey)
	require.Len(t, got.Devotions, 1)
	assert.Equal(t, "Fiend", got.Devotions[0].Name)
	require.Len(t, got.Devotions[0].Stars, 2)
	assert.Equal(t, "Cadence", got.Devotions[0].Stars[0].Bound)
	assert.Equal(t, "selfonattack_25%", got.Devotions[0].Stars[0].Trigger)
}

func TestDecodeHTMLReportsMissingOrMalformedData(t *testing.T) {
	t.Parallel()

	_, err := DecodeHTML([]byte("<html></html>"))
	assert.ErrorContains(t, err, "assignment not found")

	_, err = DecodeHTML([]byte(`<script>window.DATA = {not json};</script>`))
	assert.ErrorContains(t, err, "window.DATA JSON")
}

func TestDecodeHTMLStripsANSIFormattingCodes(t *testing.T) {
	t.Parallel()

	html := []byte(`<script>window.DATA={
		"sheet":{
			"character":{"name":"\u001b[31mSheet Name\u001b[m"},
			"stats":{},
			"equipment":[{"slot":"Head","name":"\u001b[33mHelm\u001b[m","primary":["\u001b[33m200\u001b[m Armor"],"groups":[{"lines":["\u001b[31mBurning\u001b[m"]}]}]
		},
		"tree":{"trees":[{"mastery":"\u001b[32mSoldier\u001b[m","nodes":[{"name":"\u001b[33mCadence\u001b[m","cur":["\u001b[33m+2\u001b[m Damage"]}]}]},
		"devo":{"constellations":[{"name":"\u001b[35mFiend\u001b[m","stars":[{"name":"\u001b[31mFlame Torrent\u001b[m","bound":{"name":"\u001b[36mCadence\u001b[m","trigger":"on Attack"}}]}]}
	};</script>`)

	got, err := DecodeHTML(html)
	require.NoError(t, err)
	assert.Equal(t, "Sheet Name", got.Name)
	assert.Equal(t, "Cadence", got.Skills[0].Name)
	assert.Equal(t, "Soldier", got.Skills[0].Mastery)
	assert.Equal(t, []string{"+2 Damage"}, got.Skills[0].Current)
	assert.Equal(t, "Fiend", got.Devotions[0].Name)
	assert.Equal(t, "Flame Torrent", got.Devotions[0].Stars[0].Name)
	assert.Equal(t, "Cadence", got.Devotions[0].Stars[0].Bound)
	assert.JSONEq(t, `{"slot":"Head","name":"Helm","primary":["200 Armor"],"groups":[{"lines":["Burning"]}]}`, string(got.Equipment[0].Raw))
}

func TestDecodeJSONRejectsMissingSheetData(t *testing.T) {
	t.Parallel()

	_, err := DecodeJSON([]byte(`{}`))
	assert.ErrorContains(t, err, "missing sheet")

	_, err = DecodeJSON([]byte(`{"sheet":{"character":{}}}`))
	assert.ErrorContains(t, err, "missing sheet.stats")
}
