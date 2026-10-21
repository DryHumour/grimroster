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

On Windows, `internal/arz` can discover a Steam installation from the registry
and load its base and installed expansion databases. Other platforms can open
ARZ files or a game directory explicitly.

The project does not parse GDC save files directly.
