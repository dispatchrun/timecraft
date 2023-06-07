package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/stealthrocket/timecraft/internal/debug/nettrace"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

const traceUsage = `
Usage:	timecraft trace [options] <process id>

Options:
   -c, --config             Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -d, --duration duration  Duration of the trace (default to the process uptime)
   -h, --help               Show this usage information
   -m, --messages           Show protocol messages instead of low level connection data
   -o, --output format      Output format, one of: text, json, yaml
   -t, --start-time time    Time at which the trace starts (default to 1 minute)
   -v, --verbose            For text output, display more details about the trace
`

func trace(ctx context.Context, args []string) error {
	var (
		output    = outputFormat("text")
		startTime = human.Time{}
		duration  = human.Duration(1 * time.Minute)
		messages  = false
		verbose   = false
	)

	flagSet := newFlagSet("timecraft trace", traceUsage)
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &duration, "d", "duration")
	customVar(flagSet, &startTime, "t", "start-time")
	boolVar(flagSet, &messages, "m", "messages")
	boolVar(flagSet, &verbose, "v", "verbose")

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New(`expected exactly one process id as argument`)
	}

	processID, err := parseProcessID(args[0])
	if err != nil {
		return err
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
	// timeRange := timemachine.TimeRange{
	// 	Start: time.Time(startTime),
	// 	End:   time.Time(startTime).Add(time.Duration(duration)),
	// }

	logSegment, err := registry.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest)
	defer logReader.Close()

	format := "%v"
	if verbose {
		format = "%+v"
	}

	events := &nettrace.EventReader{
		Records: timemachine.NewLogRecordReader(logReader),
	}

	if messages {
		var writer stream.WriteCloser[nettrace.Message]
		switch output {
		case "json":
			writer = jsonprint.NewWriter[nettrace.Message](os.Stdout)
		case "yaml":
			writer = yamlprint.NewWriter[nettrace.Message](os.Stdout)
		default:
			writer = textprint.NewWriter[nettrace.Message](os.Stdout,
				textprint.Format[nettrace.Message](format),
				textprint.Separator[nettrace.Message]("\n"),
			)
			defer fmt.Println()
		}
		defer writer.Close()
		_, err = stream.Copy[nettrace.Message](writer, &nettrace.MessageReader{
			Events: events,
			Protos: []nettrace.ConnProtocol{
				nettrace.HTTP1(),
			},
		})
	} else {
		var writer stream.WriteCloser[nettrace.Event]
		switch output {
		case "json":
			writer = jsonprint.NewWriter[nettrace.Event](os.Stdout)
		case "yaml":
			writer = yamlprint.NewWriter[nettrace.Event](os.Stdout)
		default:
			writer = textprint.NewWriter[nettrace.Event](os.Stdout,
				textprint.Format[nettrace.Event](format),
				textprint.Separator[nettrace.Event](""),
			)
		}
		defer writer.Close()
		_, err = stream.Copy[nettrace.Event](writer, events)
	}
	return err
}
