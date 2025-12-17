package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	plugingetter "github.com/hashicorp/packer/packer/plugin-getter"
	"github.com/hashicorp/packer/packer/plugin-getter/github"
)

type GithubProxyGetter struct {
	BaseURL string
	Name    string
}

// Ensure GithubProxyGetter implements the Getter interface
var _ plugingetter.Getter = new(GithubProxyGetter)

func (g *GithubProxyGetter) Get(what string, opts plugingetter.GetOptions) (io.ReadCloser, error) {
	// Parse the plugin source to get owner/repo
	ghPlugin, err := github.NewGithubPlugin(opts.PluginRequirement.Identifier)
	if err != nil {
		return nil, err
	}

	repoPath := ghPlugin.RealRelativePath() // e.g., "bdwyertech/packer-plugin-aws"

	switch what {
	case "releases":
		// Enforce exact version constraint
		// We expect the constraints to check against a specific version
		constraints := opts.PluginRequirement.VersionConstraints
		if len(constraints) == 0 {
			return nil, fmt.Errorf("proxy getter requires an exact version constraint")
		}

		// We will try to find the "exact" version from the constraint string.
		// This is a bit of a heuristic since Constraints are complex.
		// But for "version = 1.2.3", String() usually returns "1.2.3" or "= 1.2.3"
		constStr := constraints.String()
		// Simple parsing for now: assume user provided something like "= 1.2.3" or "1.2.3"
		// If it contains ranges like ">=", it's not exact enough for us to guess without listing.
		if strings.ContainsAny(constStr, ">,<,~") {
			return nil, fmt.Errorf("proxy getter requires an exact version, found: %s", constStr)
		}

		exactVersion := strings.TrimSpace(strings.TrimPrefix(constStr, "="))
		exactVersion = strings.TrimSpace(exactVersion)

		if exactVersion == "" {
			return nil, fmt.Errorf("could not determine exact version from constraint: %s", constStr)
		}

		// Mock a response that looks like a list of releases with just this one version
		releases := []plugingetter.Release{
			{Version: exactVersion},
		}
		buf, err := json.Marshal(releases)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(buf)), nil

	case "zip", "sha256":
		verStr := opts.VersionString() // "0.0.3" - this is better.

		var filename string
		if what == "zip" {
			filename = opts.ExpectedZipFilename()
		} else {
			// SHA256SUMS file
			// Format: packer-plugin-{name}_v{version}_SHA256SUMS usually
			filename = fmt.Sprintf("%s%s_SHA256SUMS", opts.PluginRequirement.FilenamePrefix(), opts.Version())
		}

		// URL construction
		// BaseURL: https://artifactory.my.org/artifactory/GITHUB
		// Repo: bdwyertech/packer-plugin-aws
		// Path: /releases/download/0.0.3/filename
		url := fmt.Sprintf("%s/%s/releases/download/v%s/%s", g.BaseURL, repoPath, verStr, filename)

		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get %s: %s", url, resp.Status)
		}
		if what == "sha256" {
			return github.TransformChecksumStream()(resp.Body)
		}
		return resp.Body, nil
	}

	return nil, fmt.Errorf("unknown get request: %s", what)
}

func (g *GithubProxyGetter) Init(req *plugingetter.Requirement, entry *plugingetter.ChecksumFileEntry) error {
	// reuse github's init- it parses filenames well
	ghGetter := &github.Getter{}
	return ghGetter.Init(req, entry)
}

func (g *GithubProxyGetter) Validate(opt plugingetter.GetOptions, expectedVersion string, installOpts plugingetter.BinaryInstallationOptions, entry *plugingetter.ChecksumFileEntry) error {
	ghGetter := &github.Getter{}
	return ghGetter.Validate(opt, expectedVersion, installOpts, entry)
}

func (g *GithubProxyGetter) ExpectedFileName(pr *plugingetter.Requirement, version string, entry *plugingetter.ChecksumFileEntry, zipFileName string) string {
	ghGetter := &github.Getter{}
	return ghGetter.ExpectedFileName(pr, version, entry, zipFileName)
}
