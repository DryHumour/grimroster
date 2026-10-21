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

On Windows, `internal/arz` can discover a Steam installation from the registry
and load its base and installed expansion databases. Other platforms can open
ARZ files or a game directory explicitly.

The project does not parse GDC save files directly.
