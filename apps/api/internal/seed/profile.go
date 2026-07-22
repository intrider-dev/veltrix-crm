package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	seedVersion       = 1
	generatorContract = "synthetic-sales-crm-v1"
)

// Profile describes a deterministic synthetic dataset. Counts intentionally
// exclude supporting rows such as the workspace, pipeline, and search index.
type Profile struct {
	Name       string
	Workspace  string
	Slug       string
	Contacts   int64
	Companies  int64
	Deals      int64
	Activities int64
}

var profiles = map[string]Profile{
	"demo": {
		Name:       "demo",
		Workspace:  "Demo workspace",
		Slug:       "demo-workspace",
		Contacts:   24,
		Companies:  8,
		Deals:      12,
		Activities: 80,
	},
	"small": {
		Name:       "small",
		Workspace:  "Small synthetic dataset",
		Slug:       "seed-small",
		Contacts:   1_000,
		Companies:  250,
		Deals:      500,
		Activities: 5_000,
	},
	"benchmark": {
		Name:       "benchmark",
		Workspace:  "Benchmark synthetic dataset",
		Slug:       "seed-benchmark",
		Contacts:   100_000,
		Companies:  25_000,
		Deals:      50_000,
		Activities: 500_000,
	},
}

func ParseProfile(name string) (Profile, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	profile, ok := profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown seed profile %q; expected demo, small, or benchmark", name)
	}
	return profile, nil
}

func (profile Profile) validate() error {
	if profile.Name == "" || profile.Workspace == "" || profile.Slug == "" {
		return errors.New("seed profile identity is incomplete")
	}
	if profile.Contacts < 1 || profile.Companies < 1 || profile.Deals < 1 || profile.Activities < 1 {
		return errors.New("seed profile counts must be positive")
	}
	registered, ok := profiles[profile.Name]
	if !ok || registered != profile {
		return errors.New("seed profile does not match a registered deterministic contract")
	}
	return nil
}

type Counts struct {
	Contacts   int64 `json:"contacts"`
	Companies  int64 `json:"companies"`
	Deals      int64 `json:"deals"`
	Activities int64 `json:"activities"`
}

func (profile Profile) counts() Counts {
	return Counts{
		Contacts:   profile.Contacts,
		Companies:  profile.Companies,
		Deals:      profile.Deals,
		Activities: profile.Activities,
	}
}

func (profile Profile) datasetHash() string {
	manifest := struct {
		Contract string  `json:"contract"`
		Version  int     `json:"version"`
		Profile  Profile `json:"profile"`
	}{
		Contract: generatorContract,
		Version:  seedVersion,
		Profile:  profile,
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
