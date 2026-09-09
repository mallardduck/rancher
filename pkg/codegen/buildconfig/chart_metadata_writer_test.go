package main_test

import (
	"bytes"
	"testing"

	main "github.com/rancher/rancher/pkg/codegen/buildconfig"
	"github.com/stretchr/testify/require"
)

func TestChartMetadataWriterRun(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{
		// Future: add build.yaml config values that should update Chart.yaml
	}

	chartInput := `apiVersion: v2
name: rancher
description: Install Rancher Server to manage Kubernetes clusters across providers.
# Note: version and appVersion are updated at chart build time by scripts/chart/build
# from values computed by scripts/version. Default values here ensure valid YAML.
version: 0.0.0-dev
appVersion: v0.0.0-dev
kubeVersion: < 1.37.0-0
home: https://rancher.com
icon: https://raw.githubusercontent.com/rancher/ui/master/public/assets/images/logos/welcome-cow.svg
keywords:
  - rancher
sources:
  - https://github.com/rancher/rancher
maintainers:
  - name: Rancher Labs
    email: charts@rancher.com
`

	// Currently no replacements happen, so expected equals input
	expected := chartInput

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartMetadataWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	require.Equal(t, expected, got)
}

func TestChartMetadataWriterPreservesFormatting(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{}

	// Test that all formatting is preserved: comments, blank lines, indentation
	chartInput := `# Chart metadata
apiVersion: v2
name: rancher

# Description and versioning
description: Install Rancher Server to manage Kubernetes clusters across providers.
version: 0.0.0-dev
appVersion: v0.0.0-dev

# Kubernetes compatibility
kubeVersion: < 1.37.0-0

# Project information
home: https://rancher.com
keywords:
  - rancher
  - kubernetes
`

	expected := chartInput

	chart := bytes.NewBufferString(chartInput)
	out := new(bytes.Buffer)

	w := &main.ChartMetadataWriter{
		Config: cfg,
		Chart:  chart,
		Output: out,
	}
	require.NoError(t, w.Run())

	got := out.String()
	require.Equal(t, expected, got)
}

func TestChartMetadataWriterErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		w := &main.ChartMetadataWriter{
			Config: nil,
			Chart:  bytes.NewBufferString("test"),
			Output: new(bytes.Buffer),
		}
		err := w.Run()
		require.Error(t, err)
		require.Contains(t, err.Error(), "nil config")
	})

	t.Run("nil chart input", func(t *testing.T) {
		w := &main.ChartMetadataWriter{
			Config: map[string]string{},
			Chart:  nil,
			Output: new(bytes.Buffer),
		}
		err := w.Run()
		require.Error(t, err)
		require.Contains(t, err.Error(), "nil chart input")
	})

	t.Run("nil output", func(t *testing.T) {
		w := &main.ChartMetadataWriter{
			Config: map[string]string{},
			Chart:  bytes.NewBufferString("test"),
			Output: nil,
		}
		err := w.Run()
		require.Error(t, err)
		require.Contains(t, err.Error(), "nil output")
	})

	t.Run("invalid yaml", func(t *testing.T) {
		invalidYAML := `apiVersion: v2
name: [unclosed
`
		w := &main.ChartMetadataWriter{
			Config: map[string]string{},
			Chart:  bytes.NewBufferString(invalidYAML),
			Output: new(bytes.Buffer),
		}
		err := w.Run()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse chart YAML")
	})
}
