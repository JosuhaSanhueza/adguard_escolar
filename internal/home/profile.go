package home

import (
	"bytes"
	"fmt"
	"os"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/google/renameio/v2/maybe"
	yaml "go.yaml.in/yaml/v4"
)

// profileHeader is written at the top of every exported profile file, so that
// anyone opening it understands what it is and isn't.
const profileHeader = "" +
	"# AdGuard Home configuration profile.\n" +
	"#\n" +
	"# This file contains only the settings that are safe to copy between\n" +
	"# installations (DNS behavior, blocklists, blocked services, safe\n" +
	"# search, cache, query log and statistics retention). It does NOT\n" +
	"# contain the web UI address, admin users/passwords, TLS certificates,\n" +
	"# DHCP, or persistent clients: those must always be set up by hand for\n" +
	"# each installation.\n" +
	"#\n" +
	"# Usage: finish the setup wizard on the target installation first, then\n" +
	"# run: AdGuardHome --import-profile=<this file> and restart the service.\n\n"

// profileTopLevelKeys lists the top-level YAML keys of AdGuardHome.yaml that
// are safe to export/import as part of a configuration profile. Anything not
// listed here (bind address, users, tls, dhcp, os, clients, etc.) is
// considered installation-specific and is left untouched.
var profileTopLevelKeys = []string{
	"dns",
	"filters",
	"whitelist_filters",
	"user_rules",
	"filtering",
	"querylog",
	"statistics",
	"theme",
	"language",
	"gamecontrol",
}

// profileExcludedNestedKeys lists, for a handful of the top-level keys above,
// nested keys that are still installation-specific and must never travel
// with the profile (network bind settings, filesystem paths).
var profileExcludedNestedKeys = map[string][]string{
	"dns":        {"bind_hosts", "port"},
	"querylog":   {"dir_path"},
	"statistics": {"dir_path"},
}

// yamlMap is a generic representation of a parsed YAML document, used to
// manipulate the configuration file without depending on its full Go schema.
type yamlMap = map[string]any

// exportProfile reads the configuration file at confPath and writes the
// subset of it that is safe to reuse on another installation to outPath.
func exportProfile(confPath, outPath string) (err error) {
	defer func() { err = errors.Annotate(err, "exporting profile: %w") }()

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	full := yamlMap{}
	if err = yaml.Unmarshal(data, &full); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	profile := yamlMap{}
	for _, key := range profileTopLevelKeys {
		val, ok := full[key]
		if !ok {
			continue
		}

		if nested, hasNested := val.(yamlMap); hasNested {
			val = withoutKeys(nested, profileExcludedNestedKeys[key])
		}

		profile[key] = val
	}

	buf := &bytes.Buffer{}
	buf.WriteString(profileHeader)

	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	if err = enc.Encode(profile); err != nil {
		return fmt.Errorf("encoding profile: %w", err)
	}

	if err = maybe.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing profile file: %w", err)
	}

	return nil
}

// importProfile reads a profile file previously produced by exportProfile
// and merges it into the configuration file at confPath, preserving every
// installation-specific setting already present there (bind address, users,
// tls, dhcp, os, clients, and the excluded nested keys above).
func importProfile(confPath, profilePath string) (err error) {
	defer func() { err = errors.Annotate(err, "importing profile: %w") }()

	targetData, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	target := yamlMap{}
	if err = yaml.Unmarshal(targetData, &target); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return fmt.Errorf("reading profile file: %w", err)
	}

	profile := yamlMap{}
	if err = yaml.Unmarshal(profileData, &profile); err != nil {
		return fmt.Errorf("parsing profile file: %w", err)
	}

	mergeYAMLMaps(target, profile)

	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	if err = enc.Encode(target); err != nil {
		return fmt.Errorf("encoding merged config: %w", err)
	}

	if err = maybe.WriteFile(confPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// withoutKeys returns a shallow copy of m without the given keys.
func withoutKeys(m yamlMap, keys []string) yamlMap {
	if len(keys) == 0 {
		return m
	}

	cp := make(yamlMap, len(m))
	for k, v := range m {
		cp[k] = v
	}

	for _, k := range keys {
		delete(cp, k)
	}

	return cp
}

// mergeYAMLMaps recursively overlays src onto dst: for every key in src, if
// both dst and src hold a nested map under that key they are merged
// recursively, otherwise src's value replaces dst's. Keys present only in
// dst (e.g. the installation-specific ones a profile never contains) are
// left untouched.
func mergeYAMLMaps(dst, src yamlMap) {
	for k, srcVal := range src {
		dstVal, ok := dst[k]
		if !ok {
			dst[k] = srcVal

			continue
		}

		dstNested, dstIsMap := dstVal.(yamlMap)
		srcNested, srcIsMap := srcVal.(yamlMap)
		if dstIsMap && srcIsMap {
			mergeYAMLMaps(dstNested, srcNested)
		} else {
			dst[k] = srcVal
		}
	}
}
