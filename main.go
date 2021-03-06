package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/victoriametrics/vmctl/influx"
	"github.com/victoriametrics/vmctl/prometheus"
	"github.com/victoriametrics/vmctl/vm"
)

var (
	buildTag      = "unknown"
	buildRevision = "unknown"
	buildTime     = "unknown"
)

func main() {
	start := time.Now()
	app := &cli.App{
		Name:    "vmctl",
		Usage:   "Victoria metrics command-line tool",
		Version: fmt.Sprintf("%s, rev. %s, built at %s", buildTag, buildRevision, buildTime),
		Commands: []*cli.Command{
			{
				Name:  "influx",
				Usage: "Migrate timeseries from InfluxDB",
				Flags: mergeFlags(globalFlags, influxFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("InfluxDB import mode")

					iCfg := influx.Config{
						Addr:      c.String(influxAddr),
						Username:  c.String(influxUser),
						Password:  c.String(influxPassword),
						Database:  c.String(influxDB),
						Retention: c.String(influxRetention),
						Filter: influx.Filter{
							Series:    c.String(influxFilterSeries),
							TimeStart: c.String(influxFilterTimeStart),
							TimeEnd:   c.String(influxFilterTimeEnd),
						},
						ChunkSize: c.Int(influxChunkSize),
					}
					influxClient, err := influx.NewClient(iCfg)
					if err != nil {
						return fmt.Errorf("failed to create influx client: %s", err)
					}

					vmCfg := initConfigVM(c)
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					processor := newInfluxProcessor(influxClient, importer,
						c.Int(influxConcurrency), c.String(influxMeasurementFieldSeparator))
					return processor.run(c.Bool(globalSilent))
				},
			},
			{
				Name:  "prometheus",
				Usage: "Migrate timeseries from Prometheus",
				Flags: mergeFlags(globalFlags, promFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("Prometheus import mode")

					vmCfg := initConfigVM(c)
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					promCfg := prometheus.Config{
						Snapshot: c.String(promSnapshot),
						Filter: prometheus.Filter{
							TimeMin:    c.String(promFilterTimeStart),
							TimeMax:    c.String(promFilterTimeEnd),
							Label:      c.String(promFilterLabel),
							LabelValue: c.String(promFilterLabelValue),
						},
					}
					cl, err := prometheus.NewClient(promCfg)
					if err != nil {
						return fmt.Errorf("failed to create prometheus client: %s", err)
					}
					pp := prometheusProcessor{
						cl: cl,
						im: importer,
						cc: c.Int(promConcurrency),
					}
					return pp.run(c.Bool(globalSilent))
				},
			},
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Execution cancelled")
		os.Exit(0)
	}()

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Total time: %v\n", time.Since(start))
}

func initConfigVM(c *cli.Context) vm.Config {
	return vm.Config{
		Addr:               c.String(vmAddr),
		User:               c.String(vmUser),
		Password:           c.String(vmPassword),
		Concurrency:        uint8(c.Int(vmConcurrency)),
		Compress:           c.Bool(vmCompress),
		AccountID:          c.String(vmAccountID),
		BatchSize:          c.Int(vmBatchSize),
		SignificantFigures: c.Int(vmSignificantFigures),
	}
}
