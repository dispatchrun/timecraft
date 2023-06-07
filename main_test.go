package main_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	main "github.com/stealthrocket/timecraft"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

func TestTimecraft(t *testing.T) {
	t.Setenv("TIMECRAFT_TEST_CACHE", t.TempDir())
	t.Run("export", export.run)
	t.Run("get", get.run)
	t.Run("help", help.run)
	t.Run("logs", logs.run)
	t.Run("replay", replay.run)
	t.Run("root", root.run)
	t.Run("run", run.run)
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
					// Add a "one/two" subdirectory because the path is unlikely
					// to be used in the code, and it will detect a regression
					// if using a non-existing directory causes the program to
					// fail.
					Location: filepath.Join(t.TempDir(), "one", "two"),
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

func timecraft(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	ctx := context.Background()
	deadline, ok := t.Deadline()
	if ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	outbuf := acquireBuffer()
	defer releaseBuffer(outbuf)

	errbuf := acquireBuffer()
	defer releaseBuffer(errbuf)

	tmpdir := t.TempDir()

	stdoutFile, err := os.Create(filepath.Join(tmpdir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(filepath.Join(tmpdir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	defer stderrFile.Close()

	defaultStdout := os.Stdout
	defaultStderr := os.Stderr
	defer func() {
		os.Stdout = defaultStdout
		os.Stderr = defaultStderr
	}()
	os.Stdout = stdoutFile
	os.Stderr = stderrFile

	exitCode = main.Root(ctx, args...)

	if err = readFromFile(outbuf, stdoutFile.Name()); err != nil {
		t.Fatal(err)
	}
	if err = readFromFile(errbuf, stderrFile.Name()); err != nil {
		t.Fatal(err)
	}

	stdout = outbuf.String()
	stdout = strings.TrimPrefix(stdout, "\n")

	stderr = errbuf.String()
	stderr = strings.TrimPrefix(stderr, "\n")
	return
}

func readFromFile(buf *bytes.Buffer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	s, err := f.Stat()
	if err != nil {
		return err
	}
	buf.Grow(int(s.Size()))
	_, err = buf.ReadFrom(f)
	return err
}

var buffers sync.Pool

func acquireBuffer() *bytes.Buffer {
	b, _ := buffers.Get().(*bytes.Buffer)
	if b == nil {
		b = new(bytes.Buffer)
	} else {
		b.Reset()
	}
	return b
}

func releaseBuffer(b *bytes.Buffer) {
	buffers.Put(b)
}
