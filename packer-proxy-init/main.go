package main

import (
	"crypto/sha256"
	"flag"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/hashicorp/hcl/v2/hclparse"
	pluginsdk "github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/hashicorp/packer/hcl2template"
	"github.com/hashicorp/packer/packer"
	plugingetter "github.com/hashicorp/packer/packer/plugin-getter"
	"github.com/hashicorp/packer/version"
)

var githubSource string
var releasesSource string
var force bool

func init() {
	releasesSource = os.Getenv("PKR_INIT_RELEASES_SOURCE")
	if releasesSource == "" {
		releasesSource = "https://releases.hashicorp.com/"
	}
	flag.StringVar(&githubSource, "github-source", os.Getenv("PKR_INIT_GITHUB_SOURCE"), "GitHub proxy, e.g. https://artifacts.my.org/artifactory/GITHUB")
	flag.StringVar(&releasesSource, "releases-source", releasesSource, "Hashicorp Releases proxy, e.g. https://artifacts.my.org/artifactory/HASHICORP")
	flag.BoolVar(&force, "force", false, "Forces reinstallation of plugins, even if already installed.")
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	flag.Parse()
	if githubSource == "" {
		log.Fatal("github-source is required")
	}
	if _, err := url.Parse(githubSource); err != nil {
		log.Fatal("error parsing releases source: ", err)
	}
	if _, err := url.Parse(releasesSource); err != nil {
		log.Fatal("error parsing releases source: ", err)
	}
	if !strings.HasSuffix(releasesSource, "/") {
		releasesSource += "/"
	}
	pluginDir, err := packer.PluginFolder()
	if err != nil {
		log.Fatal(err)
	}
	args := flag.Args()
	switch len(args) {
	case 0:
		args = append(args, ".")
	case 1:
		// pass
	default:
		log.Fatal("too many arguments")
	}
	if val, set := os.LookupEnv("PACKER_PLUGIN_PATH"); set {
		// homeDir, _ := os.UserHomeDir()
		// srcDir := filepath.Join(homeDir, ".config", "packer", "plugins")
		srcDir := "/root/.config/packer/plugins"
		log.Infof("Copying pre-installed plugins into PACKER_PLUGIN_PATH (%s)", val)

		if err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(srcDir, path)
			dst := filepath.Join(pluginDir, relPath)
			if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, data, 0755)
		}); err != nil {
			log.Fatal(err)
		}
	}
	parser := &hcl2template.Parser{
		CorePackerVersion:       version.SemVer,
		CorePackerVersionString: version.FormattedVersion(),
		Parser:                  hclparse.NewParser(),
	}
	cfg, diags := parser.Parse(args[0], nil, nil)
	if diags.HasErrors() {
		log.Fatal(diags.Error())
	}
	reqs, diags := cfg.PluginRequirements()
	if diags.HasErrors() {
		log.Fatal(diags.Error())
	}

	opts := plugingetter.ListInstallationsOptions{
		PluginDirectory: pluginDir,
		BinaryInstallationOptions: plugingetter.BinaryInstallationOptions{
			OS:              runtime.GOOS,
			ARCH:            runtime.GOARCH,
			APIVersionMajor: pluginsdk.APIVersionMajor,
			APIVersionMinor: pluginsdk.APIVersionMinor,
			Checksummers: []plugingetter.Checksummer{
				{Type: "sha256", Hash: sha256.New()},
			},
			ReleasesOnly: true,
		},
	}

	if runtime.GOOS == "windows" && opts.Ext == "" {
		opts.BinaryInstallationOptions.Ext = ".exe"
	}

	log.Debugf("init: %#v", opts)

	// the ordering of the getters is important here, place the getter on top which you want to try first
	getters := []plugingetter.Getter{
		&ReleasesGetter{
			Name:    "releases",
			BaseURL: releasesSource,
		},
		&GithubProxyGetter{
			Name:    "github-proxy",
			BaseURL: githubSource,
		},
	}

	for _, pluginRequirement := range reqs {

		installs, err := pluginRequirement.ListInstallations(opts)
		if err != nil {
			log.Fatal(err)
		}
		if len(installs) > 0 {
			if !force {
				continue
			}
		}

		newInstall, err := pluginRequirement.InstallLatest(plugingetter.InstallOptions{
			PluginDirectory:           opts.PluginDirectory,
			BinaryInstallationOptions: opts.BinaryInstallationOptions,
			Getters:                   getters,
			Force:                     force,
		})
		if err != nil {
			log.Fatalf("Error installing plugin %q: %s", pluginRequirement.Identifier, err)
		}
		if newInstall != nil {
			log.Infof("Installed plugin %s %s in %q", pluginRequirement.Identifier, newInstall.Version, newInstall.BinaryPath)
		}
	}
}
