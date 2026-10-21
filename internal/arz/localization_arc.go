package arz

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"codeberg.org/alex-ilchukov/arc0"
	"codeberg.org/alex-ilchukov/arc0/decode"
	"codeberg.org/alex-ilchukov/lz4"
)

func decodeLocalizationARC(data []byte) (map[string]string, error) {
	reader := bytes.NewReader(data)
	archive, err := decode.Archive(reader)
	if err != nil {
		return nil, fmt.Errorf("decode ARC archive: %w", err)
	}
	result := make(map[string]string)
	for _, file := range archive.Files() {
		if !strings.EqualFold(filepath.Ext(file.Name()), ".txt") {
			continue
		}
		content, err := readARCFile(reader, file)
		if err != nil {
			return nil, fmt.Errorf("read ARC localization file %q: %w", file.Name(), err)
		}
		for key, value := range parseLocalizationText(content) {
			result[key] = value
		}
	}
	return result, nil
}

func readARCFile(reader io.ReadSeeker, file arc0.File) ([]byte, error) {
	var result []byte
	for _, chunk := range file.Chunks() {
		_, err := reader.Seek(int64(chunk.Offset()), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("seek to chunk: %w", err)
		}
		compressed := make([]byte, chunk.CompressedSize())
		if _, err := io.ReadFull(reader, compressed); err != nil {
			return nil, fmt.Errorf("read compressed chunk: %w", err)
		}
		if chunk.CompressedSize() >= chunk.DecompressedSize() {
			result = append(result, compressed...)
			continue
		}
		decompressed := make([]byte, chunk.DecompressedSize())
		if err := lz4.Decompress(decompressed, compressed); err != nil {
			return nil, fmt.Errorf("decompress chunk: %w", err)
		}
		result = append(result, decompressed...)
	}
	return result, nil
}

func parseLocalizationText(data []byte) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if found && key != "" {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

func (d *Database) LocalizedText(tag string) (string, bool) {
	value, found := d.localization[tag]
	return value, found
}
