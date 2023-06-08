package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/stealthrocket/timecraft/internal/debug/tracing"
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
   -x, --exchange           Show protocol messages exchanged instead of low level connection events
   -o, --output format      Output format, one of: text, json, yaml
   -t, --start-time time    Time at which the trace starts (default to 1 minute)
   -v, --verbose            For text output, display more details about the trace
`

func trace(ctx context.Context, args []string) error {
	var (
		output    = outputFormat("text")
		startTime = human.Time{}
		duration  = human.Duration(1 * time.Minute)
		exchange  = false
		verbose   = false
	)

	flagSet := newFlagSet("timecraft trace", traceUsage)
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &duration, "d", "duration")
	customVar(flagSet, &startTime, "t", "start-time")
	boolVar(flagSet, &exchange, "x", "exchange")
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

	events := &tracing.EventReader{
		Records: timemachine.NewLogRecordReader(logReader),
	}

	if exchange {
		var writer stream.WriteCloser[tracing.Exchange]
		switch output {
		case "json":
			writer = jsonprint.NewWriter[tracing.Exchange](os.Stdout)
		case "yaml":
			writer = yamlprint.NewWriter[tracing.Exchange](os.Stdout)
		default:
			writer = textprint.NewWriter[tracing.Exchange](os.Stdout,
				textprint.Format[tracing.Exchange](format),
				textprint.Separator[tracing.Exchange]("\n"),
			)
			defer fmt.Println()
		}
		defer writer.Close()
		_, err = stream.Copy[tracing.Exchange](writer, &tracing.ExchangeReader{
			Messages: &tracing.MessageReader{
				Events: events,
				Protos: []tracing.ConnProtocol{
					tracing.HTTP1(),
				},
			},
		})
	} else {
		var writer stream.WriteCloser[tracing.Event]
		switch output {
		case "json":
			writer = jsonprint.NewWriter[tracing.Event](os.Stdout)
		case "yaml":
			writer = yamlprint.NewWriter[tracing.Event](os.Stdout)
		default:
			writer = textprint.NewWriter[tracing.Event](os.Stdout,
				textprint.Format[tracing.Event](format),
				textprint.Separator[tracing.Event](""),
			)
		}
		defer writer.Close()
		_, err = stream.Copy[tracing.Event](writer, events)
	}
	return err
}
