# grimroster
Grim Dawn Character Roster Dumper

The data flow consumes Alex's grim JSON output and the HTML file produced by
gd-edit. The grim JSON provides character, progression, and inventory data;
the JSON assigned to `window.DATA` in the HTML provides calculated stats,
resistances, richer equipment details, named mastery skills, and constellation
identities with their bound skills. Grim's Block8 skill records retain their
record IDs and levels; when an ARZ database is available, those IDs are joined
to gd-edit's readable skill and devotion data. The merged character keeps grim's
values when the sources disagree and records those disagreements in
`diagnostics`.

`internal/grimjson` and `internal/gdedit` decode the source formats, and
`internal/merge` reconciles them into the model in `internal/character`.
`merge.ReadFiles` loads both paths and returns the merged character.
The command accepts an optional third argument, the Grim Dawn game directory,
to use its ARZ databases when correlating skill records with the named
gd-edit skill trees and devotion stars. Graphic paths are used only as internal
join keys and are not included in the output:

```sh
go run . grim.json gd-edit.html /path/to/Grim\ Dawn
```

Without the game directory, the output still includes Grim skill record IDs and
levels alongside gd-edit's named skill-tree and constellation data, but cannot
reliably join those records.
Character item records remain database identifiers; the `internal/arz`
package resolves item records to localization tags from ARZ databases.

Skill entries retain the raw save-backed records, including internal records.
`category` distinguishes mastery skills, mastery bars, devotion entries, default
actions, potion-related records, and other records. A `mastery_bar` entry's
`level` is progression in that mastery, not ranks in a combat skill.
`save_backed`, `level_observed`, and `enabled_observed` indicate which values
were explicitly read from the save. `gd_edit_only` means the entry came from a
gd-edit tree node with no correlated save record; its zero-valued save fields
and `enabled` value are not observations that the save explicitly set them to
zero or false. `gd_edit_level` and `gear_bonus` remain separate observations:
a gear bonus alone is not investment.

`match_status` reports whether a skill matched by record ID or texture, or why
correlation was not established (`no_database`, `no_arz_record`, `no_join_key`,
`ambiguous_candidates`, `record_id_conflict`, `unmatched_invested`,
`unused_html_node`, or `no_html_node`). `unused_html_node` is reserved for
zero-investment tree observations; positive unmatched observations are reported
as `unmatched_invested`. Explicit but conflicting record IDs prevent texture
fallback from joining the records. The numeric class directory relationship of
a mastery bar is first derived from ordinary mastery skills successfully
matched to gd-edit. If that evidence is unavailable, the merger uses the
documented base-game `playerclass01`–`playerclass09` identity mapping; it does
not derive mastery labels from filenames.

For save-backed skills without a gd-edit name, ARZ enrichment retains the
skill's display localization tag as `display_tag`. `name_resolution` indicates
whether only a tag was found, no tag was available, or no database was
available. The project does not currently load Grim Dawn localization files,
so a display tag is not represented as a localized `name`.

Devotion affinity values use the same five-key object everywhere:
`ascendant`, `chaos`, `eldritch`, `order`, and `primordial`. Each key contains
`points` and a `known` flag; a known zero differs from unavailable data with
`known` set to false. `affinity_requirements` and `affinity_rewards` on each
devotion are separate from the character's `current_affinities`. Rewards
describe the potential return for completing that constellation and are not
added to the current total for a partial constellation.

gd-edit's embedded `devo.affinities` is authoritative for current totals and
already includes taken Crossroads stars. Its per-constellation `required` and
`affinity` fields are authoritative for requirements and completion rewards.
When either per-constellation field is unavailable, matching ARZ constellation
records provide a fallback through their bitmap and `affinityRequired*` /
`affinityGiven*` fields. Each result includes a source field
(`gd_edit`, `arz`, `mixed`, or `unknown`). Missing totals remain unknown rather
than being inferred as zero.

ARZ records with one devotion-button link carry an independent per-star award.
Those awards are associated with save skill record IDs, not localized
constellation names. This accounts for gd-edit's grouped Crossroads display:
the current total includes each taken Crossroads star even when the displayed
five-star group is incomplete. Ordinary multi-button constellations only grant
their potential reward when complete.

When the direct total and all required reward/taken-state data are known, the
merger reconciles it against completed rewards and individually earned star
awards. A `devotion.affinity_total_mismatch` diagnostic reports disagreement;
its `grim_value` records the reward calculation and `gd_edit_value` records the
direct `devo.affinities` total. Reconciliation is suppressed if per-star
contributions cannot be established (for example, missing ARZ data or a missing
save skill block), rather than treating those contributions as zero.

Faction data is read from Grim's `Block13`; gd-edit's `window.DATA` does not
currently export reputation data. `faction_reputations` contains one entry per
Block13 faction slot. `faction.id` is the stable GDC faction enum translated
to the canonical `myFaction` identity through `records/game/gamefactions.dbr`;
`faction.source_index` is retained only as provenance and is not the identity.
The version-5 GDC slot translation follows gd-edit's faction-index mapping
(fixed slots Player, Survivors, Aetherials, Cthonians, Outlaws, Beasts, then
`User0`, `User1`, and so on). Unknown Block13 versions keep reputation values
but leave identity unresolved. ARZ controller record order and localized
display names are never used to match reputations.
`faction.known` reports whether the save identity could be translated;
`faction.definition_known` reports whether the installed ARZ catalog contains
that identity. Uncatalogued future/mod faction slots retain their raw source
index and reputation but do not receive an inferred standing.

Each entry's `reputation` is Grim's raw `Value`, with `reputation_known`
distinguishing a known zero from unavailable data. An unlocked/changed-false
entry with the `-1` initialization sentinel is marked unknown. `is_unlocked`,
`is_changed`, `positive_boost`, and `negative_boost` remain separate source
fields and are not treated as reputation. `standing` is derived from the
authoritative `factionValue1..8` thresholds and `factionTag1..8` tags in
`gamefactions.dbr`, choosing the greatest threshold less than or equal to the
raw reputation; boosts are not added to that value. Its source is recorded
independently, with the tier tag localized through `Text_EN.arc` when available.
A missing Block13,
an undecodable block, an unknown faction slot, or unavailable ARZ thresholds
does not become neutral or zero. Legacy slots retain gd-edit's canonical
readable labels. User factions carry their `tagFactionUserN` display tags,
which `Text_EN.arc` resolves when available; otherwise names fall back to the
stable `myFaction` token and retain the tag. `my_faction` preserves Block13's
explicit `Faction` identity; the export does not infer faction-choice
allegiance or hostility from reputation.

There is no independent gd-edit faction-reputation total to compare with
Block13, so no cross-source reputation-disagreement diagnostic applies. Missing
or undecodable Block13 data remains unknown, with existing Grim block
diagnostics preserved.

Faction reputation is factual data, not a vendor-access guarantee. Future
augment recommendations will also need an augment catalogue and its reputation
requirements, character level/attribute requirements, equipment-slot limits,
and vendor/quest/world-access state.

On Windows, `internal/arz` can discover a Steam installation from the registry
and load its base and installed expansion databases. Other platforms can open
ARZ files or a game directory explicitly.

The project does not parse GDC save files directly.
