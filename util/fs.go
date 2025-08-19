package util

import (
	"path/filepath"
	"strings"

	"open-vpn-test-connect/models"
)

// DiscoverConfigs finds *.ovpn files and returns VPNConfig list.
func DiscoverConfigs(dir string) ([]models.VPNConfig, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.ovpn"))
	if err != nil {
		return nil, err
	}

	cfgs := make([]models.VPNConfig, 0, len(files))
	for _, f := range files {
		cfgs = append(cfgs, models.VPNConfig{
			Name:     strings.TrimSuffix(filepath.Base(f), ".ovpn"),
			Filename: filepath.Base(f),
			FullPath: f,
		})
	}
	return cfgs, nil
}
