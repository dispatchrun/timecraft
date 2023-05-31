package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

func TestTimecraft(t *testing.T) {
	t.Setenv("TIMECRAFT_TEST_CACHE", t.TempDir())
	t.Run("export", export.run)
	t.Run("get", get.run)
	t.Run("help", help.run)
	t.Run("root", root.run)
	t.Run("unknown", unknown.run)
	t.Run("version", version.run)
}

type configuration struct {
	Registry registry `yaml:"registry"`
	Cache    cache    `yaml:"cache"`
}

type registry struct {
	Location string `yaml:"location"`
}

type cache struct {
	Location string `yaml:"location"`
}

type tests map[string]func(*testing.T)

func (suite tests) run(t *testing.T) {
	names := maps.Keys(suite)
	slices.Sort(names)

	for _, name := range names {
		test := suite[name]
		t.Run(name, func(t *testing.T) {
			b, err := yaml.Marshal(configuration{
				Registry: registry{
					Location: t.TempDir(),
				},
				Cache: cache{
					Location: os.Getenv("TIMECRAFT_TEST_CACHE"),
				},
			})
			if err != nil {
				t.Fatal("marshaling timecraft configuration:", err)
			}

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, b, 0666); err != nil {
				t.Fatal("writing timecraft configuration:", err)
			}

			t.Setenv("TIMECRAFTCONFIG", configPath)

			test(t)
		})
	}
}

func timecraft(t *testing.T, args ...string) (stdout, stderr string, err error) {
	ctx := context.Background()
	deadline, ok := t.Deadline()
	if ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	outbuf := new(strings.Builder)
	errbuf := new(strings.Builder)

	cmd := exec.CommandContext(ctx, "./timecraft", args...)
	cmd.Stdout = outbuf
	cmd.Stderr = errbuf

	err = cmd.Run()
	return outbuf.String(), errbuf.String(), err
}
