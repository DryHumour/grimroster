//go:build windows

package arz

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func DiscoverGameDir() (string, error) {
	const keyPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 219990`

	var errs []error
	for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE|view)
		if err != nil {
			errs = append(errs, fmt.Errorf("open registry key in view %d: %w", view, err))
			continue
		}

		gameDir, _, readErr := key.GetStringValue("InstallLocation")
		closeErr := key.Close()
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read install location in registry view %d: %w", view, readErr))
		}
		if closeErr != nil {
			errs = append(errs, fmt.Errorf("close registry key in view %d: %w", view, closeErr))
		}
		if readErr != nil || closeErr != nil {
			continue
		}

		if gameDir = strings.TrimSpace(gameDir); gameDir != "" {
			return gameDir, nil
		}
		errs = append(errs, errors.New("registry install location is empty"))
	}

	return "", fmt.Errorf("find Grim Dawn Steam install location: %w", errors.Join(errs...))
}
