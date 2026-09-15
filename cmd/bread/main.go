package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	bread "github.com/HuakunShen/bread"
	"github.com/HuakunShen/bread/internal/read"
	"github.com/HuakunShen/bread/internal/skill"
	"github.com/HuakunShen/bread/internal/update"
	appversion "github.com/HuakunShen/bread/internal/version"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "upgrade":
			return runUpgrade(ctx, args[1:], stdout, stderr)
		case "skill":
			return runSkill(args[1:], stdout, stderr)
		}
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

func runSkill(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("bread skill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	add := flags.Bool("add", false, "install the bundled agent skill")
	project := flags.Bool("project", false, "install into the current project instead of the user profile")
	target := flags.String("target", string(skill.TargetAll), "agent skills directory: all, agents, claude, or codex")
	dir := flags.String("dir", "", "install into this skills directory instead of the default locations")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "bread skill: unexpected positional arguments")
		return 2
	}
	if !*add {
		if *project || *dir != "" || *target != string(skill.TargetAll) {
			_, _ = fmt.Fprintln(stderr, "bread skill: --project, --target, and --dir require --add")
			return 2
		}
		guide := bread.NavigationSkill
		if !strings.HasSuffix(guide, "\n") {
			guide += "\n"
		}
		_, _ = fmt.Fprint(stdout, guide)
		return 0
	}
	if !skill.ValidTarget(*target) {
		_, _ = fmt.Fprintf(stderr, "bread skill: invalid target %q (want all, agents, claude, or codex)\n", *target)
		return 2
	}
	if *dir != "" && (*project || *target != string(skill.TargetAll)) {
		_, _ = fmt.Fprintln(stderr, "bread skill: --dir cannot be combined with --project or --target")
		return 2
	}

	options := skill.Options{
		Content: bread.NavigationSkill,
		Target:  skill.Target(*target),
		Dir:     *dir,
	}
	if *dir == "" {
		if *project {
			workingDir, err := os.Getwd()
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "bread skill: %s\n", err)
				return 1
			}
			options.Scope = skill.ScopeProject
			options.WorkingDir = workingDir
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "bread skill: resolve home directory: %s\n", err)
				return 1
			}
			options.Scope = skill.ScopeGlobal
			options.HomeDir = home
		}
	}

	paths, err := skill.Install(options)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bread skill: %s\n", err)
		return 1
	}
	location := "in the user profile"
	switch {
	case *project:
		location = "in the current project"
	case *dir != "":
		location = "in " + *dir
	}
	_, _ = fmt.Fprintf(stdout, "Installed %s skill %s:\n", skill.Name, location)
	for _, path := range paths {
		_, _ = fmt.Fprintf(stdout, "  %s\n", path)
	}
	_, _ = fmt.Fprintln(stdout, "Reload your agent so it picks up the new skill.")
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
