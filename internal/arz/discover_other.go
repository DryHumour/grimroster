//go:build !windows

package arz

import "errors"

func DiscoverGameDir() (string, error) {
	return "", errors.New("Grim Dawn registry discovery is only available on Windows")
}
