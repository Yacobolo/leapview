package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/flidai/leapview/rtest/internal/benchmark"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var specPath string
	var outputPath string
	flag.StringVar(&specPath, "spec", "", "benchmark JSON specification")
	flag.StringVar(&outputPath, "output", "", "directory for raw logs and summary.json")
	flag.Parse()
	if specPath == "" || outputPath == "" || flag.NArg() != 0 {
		return fmt.Errorf("usage: rtest-benchmark --spec <file.json> --output <directory>")
	}

	file, err := os.Open(specPath)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var spec benchmark.Spec
	if err := decoder.Decode(&spec); err != nil {
		return fmt.Errorf("decode benchmark spec: %w", err)
	}
	if !filepath.IsAbs(spec.ProjectDir) {
		spec.ProjectDir = filepath.Join(filepath.Dir(specPath), spec.ProjectDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	summary, err := benchmark.RunWithProgress(ctx, spec, outputPath, os.Stdout)
	for _, candidate := range summary.Candidates {
		if candidate.Status == "completed" {
			fmt.Printf("%s: median=%.3fs p95=%.3fs\n", candidate.Name, candidate.WallSeconds.Median, candidate.WallSeconds.P95)
		} else {
			fmt.Printf("%s: %s (%s)\n", candidate.Name, candidate.Status, candidate.Reason)
		}
	}
	return err
}
