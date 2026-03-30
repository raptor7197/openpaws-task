package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"openpaws/internal/cli"
	"openpaws/internal/config"
	"openpaws/internal/llm"
	"openpaws/internal/pipeline"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return cli.ErrUsage
	}

	switch args[0] {
	case "rank":
		return runRank(args[1:])
	default:
		return fmt.Errorf("%w: unsupported command %q", cli.ErrUsage, args[0])
	}
}

func runRank(args []string) error {
	fs := flag.NewFlagSet("rank", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var (
		topic     = fs.String("topic", "", "campaign topic to evaluate against")
		inputDir  = fs.String("input", "", "directory containing fixture JSON files")
		output    = fs.String("output", "", "optional path to write JSON report")
		platforms = fs.String("platforms", "instagram,x", "comma-separated platform list")
		provider  = fs.String("provider", "openai", "llm provider: openai (default) or mock for demo")
	)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*topic) == "" || strings.TrimSpace(*inputDir) == "" {
		return errors.New("rank requires --topic and --input")
	}

	cfg := config.Default()
	llmProvider, err := llm.NewProvider(*provider, cfg)
	if err != nil {
		return err
	}

	// The command delegates the orchestration to the pipeline package so the CLI
	// stays thin and focused on argument parsing and I/O concerns.
	runner := pipeline.Runner{
		Config:      cfg,
		LLMProvider: llmProvider,
	}

	report, err := runner.Rank(context.Background(), pipeline.RankRequest{
		Topic:     *topic,
		InputDir:  *inputDir,
		Output:    *output,
		Platforms: cli.ParsePlatforms(*platforms),
	})
	if err != nil {
		return err
	}

	// The terminal summary gives operators an immediate ranked shortlist without
	// requiring them to open the JSON artifact first.
	fmt.Print(cli.FormatConsoleReport(report))
	return nil
}
