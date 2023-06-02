package main

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/internal/debug/stdio"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

const logsUsage = `
Usage:	timecraft logs [options] <process id>

Example:

   $ timecraft logs f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

Options:
   -c, --config             Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -h, --help               Show this usage information
   -n, --limit              Limit the number of log lines to print (default to no limit)
   -t, --start-time time    Time at which the logr gets started (default to 1 minute)
`

func logs(ctx context.Context, args []string) error {
	var (
		limit     human.Count
		startTime = human.Time{}
	)

	flagSet := newFlagSet("timecraft logs", logsUsage)
	customVar(flagSet, &limit, "n", "limit")
	customVar(flagSet, &startTime, "t", "start-time")
	if limit == 0 {
		limit = math.MaxInt32
	}

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New(`expected exactly one process id as argument`)
	}

	processID, err := uuid.Parse(args[0])
	if err != nil {
		return errors.New(`malformed process id passed as argument (not a UUID)`)
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	registry, err := config.openRegistry()
	if err != nil {
		return err
	}

	manifest, err := registry.LookupLogManifest(ctx, processID)
	if err != nil {
		return err
	}
	if startTime.IsZero() {
		startTime = human.Time(manifest.StartTime)
	}

	logSegment, err := registry.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest.StartTime)
	defer logReader.Close()

	_, err = io.Copy(os.Stdout, &stdio.Limit{
		R: &stdio.Reader{
			Records:   timemachine.NewLogRecordReader(logReader),
			StartTime: time.Time(startTime),
			Stdout:    1,
			Stderr:    2,
		},
		N: int(limit),
	})
	return err
}
