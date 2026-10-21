package arz

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	arzlib "codeberg.org/alex-ilchukov/arz/v2"
	"github.com/DryHumour/grimroster/internal/character"
)

type record struct {
	db     *arzlib.Db
	record arzlib.Record
}

type Database struct {
	records map[string]record
	mu      sync.Mutex
}

func New(data []byte) (*Database, error) {
	db, err := arzlib.Decode(bytes.Clone(data))
	if err != nil {
		return nil, fmt.Errorf("decode ARZ database: %w", err)
	}

	result := &Database{records: make(map[string]record)}
	result.add(db)
	return result, nil
}

func Open(path string) (*Database, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ARZ database %q: %w", path, err)
	}

	db, err := New(data)
	if err != nil {
		return nil, fmt.Errorf("open ARZ database %q: %w", path, err)
	}

	return db, nil
}

func OpenFiles(paths ...string) (*Database, error) {
	if len(paths) == 0 {
		return nil, errors.New("no ARZ database paths provided")
	}

	result := &Database{records: make(map[string]record)}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read ARZ database %q: %w", path, err)
		}

		db, err := arzlib.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("decode ARZ database %q: %w", path, err)
		}

		result.add(db)
	}

	return result, nil
}

func OpenGameDir(gameDir string) (*Database, error) {
	basePath := filepath.Join(gameDir, "database", "database.arz")
	if _, err := os.Stat(basePath); err != nil {
		return nil, fmt.Errorf("locate base ARZ database %q: %w", basePath, err)
	}

	paths := []string{basePath}
	for _, relativePath := range []string{
		filepath.Join("gdx1", "database", "GDX1.arz"),
		filepath.Join("gdx2", "database", "GDX2.arz"),
		filepath.Join("gdx3", "database", "GDX3.arz"),
	} {
		path := filepath.Join(gameDir, relativePath)
		_, err := os.Stat(path)
		switch {
		case err == nil:
			paths = append(paths, path)
		case errors.Is(err, os.ErrNotExist):
		default:
			return nil, fmt.Errorf("locate expansion ARZ database %q: %w", path, err)
		}
	}

	return OpenFiles(paths...)
}

func OpenInstalled() (*Database, error) {
	gameDir, err := DiscoverGameDir()
	if err != nil {
		return nil, fmt.Errorf("discover Grim Dawn installation: %w", err)
	}

	db, err := OpenGameDir(gameDir)
	if err != nil {
		return nil, fmt.Errorf("open installed Grim Dawn database: %w", err)
	}

	return db, nil
}

func (d *Database) StringField(recordID, fieldName string) (string, bool, error) {
	ref, ok := d.records[recordID]
	if !ok {
		return "", false, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fields, err := ref.db.Fields(ref.record)
	if err != nil {
		return "", false, fmt.Errorf("read fields from ARZ record %q: %w", recordID, err)
	}

	for field := range fields.Seq {
		if string(ref.db.String(field.Name())) != fieldName {
			continue
		}
		if field.Type() != arzlib.FieldTypeString {
			return "", false, fmt.Errorf("ARZ field %q in record %q is not a string", fieldName, recordID)
		}
		if field.ValuesAmount() != 1 {
			return "", false, fmt.Errorf("ARZ field %q in record %q has %d values; expected one", fieldName, recordID, field.ValuesAmount())
		}

		value := ref.db.String(field.Value())
		if value == nil {
			return "", false, fmt.Errorf("ARZ field %q in record %q refers to an invalid string", fieldName, recordID)
		}

		return string(value), true, nil
	}

	return "", false, nil
}

func (d *Database) Int32Field(recordID, fieldName string) (int32, bool, error) {
	ref, ok := d.records[recordID]
	if !ok {
		return 0, false, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fields, err := ref.db.Fields(ref.record)
	if err != nil {
		return 0, false, fmt.Errorf("read fields from ARZ record %q: %w", recordID, err)
	}

	for field := range fields.Seq {
		if string(ref.db.String(field.Name())) != fieldName {
			continue
		}
		if field.Type() != arzlib.FieldTypeInt32 {
			return 0, false, fmt.Errorf("ARZ field %q in record %q is not an int32", fieldName, recordID)
		}
		if field.ValuesAmount() != 1 {
			return 0, false, fmt.Errorf("ARZ field %q in record %q has %d values; expected one", fieldName, recordID, field.ValuesAmount())
		}
		return int32(field.Value()), true, nil
	}

	return 0, false, nil
}

func (d *Database) StringValuesField(recordID, fieldName string) ([]string, bool, error) {
	ref, ok := d.records[recordID]
	if !ok {
		return nil, false, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fields, err := ref.db.Fields(ref.record)
	if err != nil {
		return nil, false, fmt.Errorf("read fields from ARZ record %q: %w", recordID, err)
	}

	for field, values := range fields.Seq {
		if string(ref.db.String(field.Name())) != fieldName {
			continue
		}
		if field.Type() != arzlib.FieldTypeString {
			return nil, false, fmt.Errorf("ARZ field %q in record %q is not a string", fieldName, recordID)
		}

		result := make([]string, 0, field.ValuesAmount())
		if field.ValuesAmount() == 1 {
			value := ref.db.String(field.Value())
			if value == nil {
				return nil, false, fmt.Errorf("ARZ field %q in record %q refers to an invalid string", fieldName, recordID)
			}
			return []string{string(value)}, true, nil
		}
		for value := range values {
			text := ref.db.String(value)
			if text == nil {
				return nil, false, fmt.Errorf("ARZ field %q in record %q refers to an invalid string", fieldName, recordID)
			}
			result = append(result, string(text))
		}
		return result, true, nil
	}

	return nil, false, nil
}

func (d *Database) StringFieldsWithPrefix(recordID, fieldPrefix string) (map[string]string, error) {
	ref, ok := d.records[recordID]
	if !ok {
		return nil, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fields, err := ref.db.Fields(ref.record)
	if err != nil {
		return nil, fmt.Errorf("read fields from ARZ record %q: %w", recordID, err)
	}

	result := make(map[string]string)
	for field := range fields.Seq {
		fieldName := string(ref.db.String(field.Name()))
		if !strings.HasPrefix(fieldName, fieldPrefix) {
			continue
		}
		if field.Type() != arzlib.FieldTypeString {
			return nil, fmt.Errorf("ARZ field %q in record %q is not a string", fieldName, recordID)
		}
		if field.ValuesAmount() != 1 {
			return nil, fmt.Errorf("ARZ field %q in record %q has %d values; expected one", fieldName, recordID, field.ValuesAmount())
		}
		value := ref.db.String(field.Value())
		if value == nil {
			return nil, fmt.Errorf("ARZ field %q in record %q refers to an invalid string", fieldName, recordID)
		}
		result[fieldName] = string(value)
	}

	return result, nil
}

func (d *Database) RecordIDsWithPrefix(prefix string) []string {
	result := make([]string, 0)
	for recordID := range d.records {
		if strings.HasPrefix(recordID, prefix) {
			result = append(result, recordID)
		}
	}
	slices.Sort(result)
	return result
}

func (d *Database) HasRecord(recordID string) bool {
	_, ok := d.records[recordID]
	return ok
}

func (d *Database) SkillDisplayTag(recordID string) (string, bool, error) {
	tag, found, err := d.StringField(recordID, "skillDisplayName")
	if err != nil {
		return "", false, fmt.Errorf("read skill display tag from %q: %w", recordID, err)
	}
	return tag, found && tag != "", nil
}

func (d *Database) Enrich(input character.Character) (character.Character, error) {
	result := input
	result.Skills = slices.Clone(input.Skills)
	result.Inventory.Equipment = slices.Clone(input.Inventory.Equipment)
	result.Inventory.Bags = slices.Clone(input.Inventory.Bags)
	for i := range result.Inventory.Bags {
		result.Inventory.Bags[i].Items = slices.Clone(input.Inventory.Bags[i].Items)
	}

	for i := range result.Inventory.Equipment {
		if err := d.enrichItem(&result.Inventory.Equipment[i].Item); err != nil {
			return character.Character{}, err
		}
	}
	for i := range result.Skills {
		skill := &result.Skills[i]
		if skill.RecordID == "" || skill.Name != "" || skill.DisplayTag != "" {
			continue
		}
		tag, found, err := d.SkillDisplayTag(skill.RecordID)
		if err != nil {
			return character.Character{}, fmt.Errorf("enrich skill %q: %w", skill.RecordID, err)
		}
		if found {
			skill.DisplayTag = tag
			skill.NameResolution = character.SkillNameResolutionTagOnly
		} else {
			skill.NameResolution = character.SkillNameResolutionMissing
		}
	}
	for i := range result.Inventory.Bags {
		for j := range result.Inventory.Bags[i].Items {
			if err := d.enrichItem(&result.Inventory.Bags[i].Items[j].Item); err != nil {
				return character.Character{}, err
			}
		}
	}

	return result, nil
}

func (d *Database) add(db *arzlib.Db) {
	for r := range db.Records {
		name := string(db.String(r.Name()))
		if name != "" {
			d.records[name] = record{db: db, record: r}
		}
	}
}

func (d *Database) enrichItem(item *character.Item) error {
	if item.RecordID == "" || item.NameTag != "" {
		return nil
	}

	nameTag, found, err := d.StringField(item.RecordID, "itemNameTag")
	if err != nil {
		return fmt.Errorf("resolve item name tag for %q: %w", item.RecordID, err)
	}
	if found {
		item.NameTag = nameTag
	}

	return nil
}
