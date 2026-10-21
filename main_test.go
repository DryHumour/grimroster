package main

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRequiresBothInputPaths(t *testing.T) {
	t.Parallel()

	err := run([]string{"grim.json"}, &bytes.Buffer{})
	assert.ErrorContains(t, err, "usage:")
}

func TestRunWritesMergedCharacterJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	grimPath := filepath.Join(dir, "grim.json")
	htmlPath := filepath.Join(dir, "gd-edit.html")
	require.NoError(t, os.WriteFile(grimPath, []byte(`{"Header":{"Name":"Test","Level":10},"Block2":{},"Block3":{}}`), 0o600))
	require.NoError(t, os.WriteFile(htmlPath, []byte(`<script>window.DATA={"sheet":{"character":{"name":"Test","level":10},"stats":{"health":100}}};</script>`), 0o600))

	var output bytes.Buffer
	require.NoError(t, run([]string{grimPath, htmlPath}, &output))

	var got struct {
		Name       string `json:"name"`
		Calculated struct {
			Health float64 `json:"health"`
		} `json:"calculated"`
	}
	require.NoError(t, jsonv2.Unmarshal(output.Bytes(), &got))
	assert.Equal(t, "Test", got.Name)
	assert.Equal(t, float64(100), got.Calculated.Health)
	assert.True(t, bytes.HasSuffix(output.Bytes(), []byte("\n")))
}
