package main

import (
	"crypto/sha256"
	"flag"
	"os"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/hcl/v2/hclparse"
	pluginsdk "github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/hashicorp/packer/hcl2template"
	"github.com/hashicorp/packer/packer"
	plugingetter "github.com/hashicorp/packer/packer/plugin-getter"
	"github.com/hashicorp/packer/version"
	log "github.com/sirupsen/logrus"
)

var source string
var force bool

func init() {
	flag.StringVar(&source, "source", os.Getenv("PKR_INIT_SOURCE"), "Artifactory GitHub mirror, e.g. https://artifacts.my.org/artifactory/GITHUB")
	flag.BoolVar(&force, "force", false, "Forces reinstallation of plugins, even if already installed.")
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	flag.Parse()
	if source == "" {
		log.Fatal("source is required")
	}
	pluginDir, err := packer.PluginFolder()
	if err != nil {
		log.Fatal(err)
	}
	if _, set := os.LookupEnv("PACKER_PLUGIN_PATH"); set {
		// homeDir, _ := os.UserHomeDir()
		// srcDir := filepath.Join(homeDir, ".config", "packer", "plugins")
		srcDir := "/root/.config/packer/plugins"

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
	cfg, diags := parser.Parse("test.pkr.hcl", nil, nil)
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
		&ArtifactoryGetter{
			Name:    "artifactory",
			BaseURL: source,
		},
	}

	for _, pluginRequirement := range reqs {
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
			log.Printf("[INFO] Installed plugin %s %s in %q", pluginRequirement.Identifier, newInstall.Version, newInstall.BinaryPath)
		}
	}
}
