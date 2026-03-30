package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"openpaws/internal/model"
)

type FixtureLoader struct{}

func (FixtureLoader) Load(dir string, platforms []model.Platform) ([]model.Account, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixture directory: %w", err)
	}

	allowed := make(map[model.Platform]struct{}, len(platforms))
	for _, platform := range platforms {
		allowed[platform] = struct{}{}
	}

	var accounts []model.Account
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read fixture file %s: %w", path, err)
		}

		// Each fixture file contains a flat array of accounts so analysts can
		// curate platform datasets without learning a bespoke storage format.
		var fileAccounts []model.Account
		if err := json.Unmarshal(data, &fileAccounts); err != nil {
			return nil, fmt.Errorf("decode fixture file %s: %w", path, err)
		}

		for _, account := range fileAccounts {
			if len(allowed) > 0 {
				if _, ok := allowed[account.Platform]; !ok {
					continue
				}
			}
			accounts = append(accounts, account)
		}
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no accounts found in %s for requested platforms %v", dir, slices.Compact(platforms))
	}

	return accounts, nil
}
