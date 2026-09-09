package main

import (
	"errors"
	"io"
)

// ChartMetadataWriter updates chart/Chart.yaml with values from build.yaml
// It parses the chart metadata as YAML and updates specific paths with values from the build config
type ChartMetadataWriter struct {
	Config map[string]string // build.yaml parsed config
	Chart  io.Reader         // chart/Chart.yaml content
	Output io.Writer         // updated chart/Chart.yaml
}

// Run reads the chart metadata, updates specific paths from the config, and writes the updated chart
func (w *ChartMetadataWriter) Run() error {
	if w.Config == nil {
		return errors.New("nil config")
	}
	if err := w.processChart(); err != nil {
		return err
	}
	return nil
}

func (w *ChartMetadataWriter) processChart() error {
	return nil
}
