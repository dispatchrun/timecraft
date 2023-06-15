package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/stealthrocket/timecraft/internal/debug/tracing"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timecraft"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"golang.org/x/exp/slices"
)

const traceUsage = `
Usage:	timecraft trace <layer> <process id> [options]

Options:
   -c, --config             Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -d, --duration duration  Duration of the trace (default to the process uptime)
   -h, --help               Show this usage information
   -o, --output format      Output format, one of: text, json, yaml
   -t, --start-time time    Time at which the trace starts (default to 1 minute)
   -v, --verbose            For text output, display more details about the trace
`

type layer struct {
	typ   string
	alt   []string
	trace func(io.Writer, outputFormat, string, stream.Reader[tracing.Event]) error
}

var layers = [...]layer{
	{
		typ:   "network",
		alt:   []string{"net", "nets", "networks"},
		trace: traceNetwork,
	},
	{
		typ:   "request",
		alt:   []string{"req", "reqs", "requests"},
		trace: traceRequest,
	},
}

func findLayer(typ string) (*layer, error) {
	for i, l := range layers {
		if l.typ == typ || slices.Contains(l.alt, typ) {
			return &layers[i], nil
		}
	}
	return nil, fmt.Errorf(`no tracing layers matching '%s'`, typ)
}

func trace(ctx context.Context, args []string) error {
	var (
		output    = outputFormat("text")
		startTime = human.Time{}
		duration  = human.Duration(1 * time.Minute)
		verbose   = false
	)

	flagSet := newFlagSet("timecraft trace", traceUsage)
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &duration, "d", "duration")
	customVar(flagSet, &startTime, "t", "start-time")
	boolVar(flagSet, &verbose, "v", "verbose")

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		perrorf(`Expected trace layer and process id as arguments:

   $ timecraft trace network <process id> ...
   $ timecraft trace request <process id> ...
`)
		return exitCode(2)
	}

	layer, err := findLayer(args[0])
	if err != nil {
		perror(err)
		return exitCode(2)
	}
	processID, err := parseProcessID(args[1])
	if err != nil {
		return err
	}
	config, err := timecraft.LoadConfig()
	if err != nil {
		return err
	}
	registry, err := config.OpenRegistry()
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

	return layer.trace(os.Stdout, output, format, &tracing.EventReader{
		Records: timemachine.NewLogRecordReader(logReader),
	})
}

func traceNetwork(w io.Writer, output outputFormat, format string, events stream.Reader[tracing.Event]) error {
	var writer stream.WriteCloser[tracing.Event]
	switch output {
	case "json":
		writer = jsonprint.NewWriter[tracing.Event](w)
	case "yaml":
		writer = yamlprint.NewWriter[tracing.Event](w)
	default:
		writer = textprint.NewWriter[tracing.Event](w,
			textprint.Format[tracing.Event](format),
			textprint.Separator[tracing.Event](""),
		)
	}
	defer writer.Close()
	_, err := stream.Copy[tracing.Event](writer, events)
	return err
}

func traceRequest(w io.Writer, output outputFormat, format string, events stream.Reader[tracing.Event]) error {
	var writer stream.WriteCloser[tracing.Exchange]
	switch output {
	case "json":
		writer = jsonprint.NewWriter[tracing.Exchange](w)
	case "yaml":
		writer = yamlprint.NewWriter[tracing.Exchange](w)
	default:
		writer = textprint.NewWriter[tracing.Exchange](w,
			textprint.Format[tracing.Exchange](format),
			textprint.Separator[tracing.Exchange]("\n"),
		)
		defer fmt.Println()
	}
	defer writer.Close()
	_, err := stream.Copy[tracing.Exchange](writer, &tracing.ExchangeReader{
		Messages: &tracing.MessageReader{
			Events: events,
			Protos: []tracing.ConnProtocol{
				tracing.HTTP1(),
			},
		},
	})
	return err
}
