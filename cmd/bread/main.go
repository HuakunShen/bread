package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/HuakunShen/bread/internal/read"
	"github.com/HuakunShen/bread/internal/update"
	appversion "github.com/HuakunShen/bread/internal/version"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "upgrade" {
		return runUpgrade(ctx, args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("bread", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	maxLines := flags.Int("max-lines", 2000, "maximum lines per file; 0 means unlimited")
	maxBytes := flags.Int64("max-bytes", 200000, "maximum aggregate content bytes; 0 means unlimited")
	noLineNumbers := flags.Bool("no-line-numbers", false, "omit line-number prefixes in text output")
	requestPath := flags.String("request", "", "read JSON requests from FILE, or - for stdin")
	showVersion := flags.Bool("version", false, "print the version")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintf(stdout, "bread %s\n", appversion.Value)
		return 0
	}
	if *format != "text" && *format != "json" {
		_, _ = fmt.Fprintf(stderr, "bread: invalid format %q (want text or json)\n", *format)
		return 2
	}
	if err := read.ValidateLimits(*maxLines, *maxBytes); err != nil {
		_, _ = fmt.Fprintf(stderr, "bread: %s\n", err)
		return 2
	}

	request, err := loadRequest(*requestPath, flags.Args(), stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bread: %s\n", err)
		return 2
	}
	request.MaxLines = *maxLines
	request.MaxBytes = *maxBytes
	results := read.Read(ctx, request)

	if *format == "json" {
		err = read.RenderJSON(stdout, results)
	} else {
		err = read.RenderText(stdout, results, !*noLineNumbers)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bread: write output: %s\n", err)
		return 2
	}
	for _, result := range results {
		if result.Error != "" {
			return 1
		}
	}
	return 0
}

func runUpgrade(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("bread upgrade", flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkOnly := flags.Bool("check", false, "check for an update without changing the executable")
	repository := os.Getenv("BREAD_RELEASE_REPOSITORY")
	if repository == "" {
		repository = appversion.Repository
	}
	repositoryFlag := flags.String("repository", repository, "GitHub repository in OWNER/NAME form")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "bread upgrade: unexpected positional arguments")
		return 2
	}
	client := update.NewClient(*repositoryFlag, appversion.Value)
	if *checkOnly {
		info, err := client.Check(ctx)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "bread upgrade: %s\n", err)
			return 1
		}
		if info.Available {
			_, _ = fmt.Fprintf(stdout, "update available: %s -> %s\n", info.Current, info.Latest)
		} else {
			_, _ = fmt.Fprintf(stdout, "bread is up to date (%s)\n", info.Latest)
		}
		return 0
	}
	if err := client.Upgrade(ctx, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "bread upgrade: %s\n", err)
		return 1
	}
	return 0
}

func loadRequest(requestPath string, specs []string, stdin io.Reader) (read.Request, error) {
	if requestPath != "" {
		if len(specs) != 0 {
			return read.Request{}, errors.New("--request cannot be combined with positional file requests")
		}
		var input io.Reader = stdin
		var file *os.File
		if requestPath != "-" {
			opened, err := os.Open(requestPath)
			if err != nil {
				return read.Request{}, fmt.Errorf("open request %q: %w", requestPath, err)
			}
			file = opened
			defer file.Close()
			input = file
		}

		var request read.Request
		decoder := json.NewDecoder(input)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return read.Request{}, fmt.Errorf("decode request: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return read.Request{}, errors.New("request contains more than one JSON value")
			}
			return read.Request{}, fmt.Errorf("decode request tail: %w", err)
		}
		if len(request.Files) == 0 {
			return read.Request{}, errors.New("request must contain at least one file")
		}
		return request, nil
	}
	if len(specs) == 0 {
		return read.Request{}, errors.New("provide file requests or --request FILE")
	}

	request := read.Request{Files: make([]read.FileRequest, 0, len(specs))}
	for _, spec := range specs {
		file, err := read.ParseSpec(spec)
		if err != nil {
			return read.Request{}, err
		}
		request.Files = append(request.Files, file)
	}
	return request, nil
}
