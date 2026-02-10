package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/Snotr/passGen/internal/generator"
	"github.com/Snotr/passGen/internal/parser"
	"github.com/Snotr/passGen/internal/tui"
	"github.com/Snotr/passGen/internal/writer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	input := flag.String("input", "", "comma-separated list of input file paths (.txt, .csv)")
	output := flag.String("output", "", "output file path (default: stdout)")
	outFormat := flag.String("format", "txt", "output format: txt or csv")
	pattern := flag.String("pattern", "", "generation pattern, e.g. {names}{numbers}{symbols}")
	amount := flag.Int64("amount", 0, "maximum number of passwords to generate (0 = unlimited)")
	maxTime := flag.Duration("time", 0, "maximum generation duration, e.g. 30s, 5m (0 = unlimited)")
	quiet := flag.Bool("quiet", false, "suppress progress output")
	caseExpand := flag.Bool("case", false, "expand values to UPPER, Camel, and lower variants")
	unique := flag.Bool("unique", false, "eliminate duplicate passwords")
	flex := flag.Bool("flex", false, "generate from every non-empty subset of groups (default mode only)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: passgen [flags] [files...]\n\n")
		fmt.Fprintf(os.Stderr, "A high-performance password generator using tag-based combinatorial logic.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  passgen words.txt\n")
		fmt.Fprintf(os.Stderr, "  passgen -input words.txt,names.csv -pattern \"{names}{numbers}\" -amount 1000\n")
		fmt.Fprintf(os.Stderr, "  passgen words.txt -time 30s -format csv -output results.csv\n")
		fmt.Fprintf(os.Stderr, "  passgen -case -unique words.txt\n")
		fmt.Fprintf(os.Stderr, "  passgen -flex -unique words.txt\n")
	}

	flag.Parse()

	// Collect input files
	var files []string
	if *input != "" {
		files = append(files, strings.Split(*input, ",")...)
	}
	files = append(files, flag.Args()...)

	if len(files) == 0 {
		flag.Usage()
		return fmt.Errorf("no input files specified")
	}

	// Validate files
	for i, f := range files {
		files[i] = strings.TrimSpace(f)
		ext := strings.ToLower(filepath.Ext(files[i]))
		if ext != ".txt" && ext != ".csv" {
			return fmt.Errorf("unsupported file format %q (use .txt or .csv)", files[i])
		}
		if _, err := os.Stat(files[i]); err != nil {
			return fmt.Errorf("input file: %w", err)
		}
	}

	// Validate format
	*outFormat = strings.ToLower(*outFormat)
	if *outFormat != "txt" && *outFormat != "csv" {
		return fmt.Errorf("unsupported output format %q (use txt or csv)", *outFormat)
	}

	// Parse input files
	var allGroups []parser.TagGroup
	for _, f := range files {
		var groups []parser.TagGroup
		var err error
		ext := strings.ToLower(filepath.Ext(f))
		switch ext {
		case ".txt":
			groups, err = parser.ParseTXT(f)
		case ".csv":
			groups, err = parser.ParseCSV(f)
		}
		if err != nil {
			return fmt.Errorf("parsing %s: %w", f, err)
		}
		allGroups = append(allGroups, groups...)
	}

	merged := parser.MergeGroups(allGroups)
	if len(merged) == 0 {
		return fmt.Errorf("no data found in input files")
	}

	// Set up generator
	cfg := generator.Config{
		Groups:      merged,
		Pattern:     *pattern,
		MaxCount:    *amount,
		MaxDuration: *maxTime,
		CaseExpand:  *caseExpand,
		Unique:      *unique,
		FlexTags:    *flex,
	}
	gen := generator.New(cfg)

	// Compute total before starting
	total, err := gen.EstimateTotal()
	if err != nil {
		return err
	}

	// Determine if interactive mode is available
	interactive := !*quiet && term.IsTerminal(int(os.Stdin.Fd()))

	// Determine output destination label
	outputDest := "stdout"
	if *output != "" {
		outputDest = *output
	}

	// Display pre-generation summary
	if !*quiet {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "  Generation Summary\n")
		fmt.Fprintf(os.Stderr, "  ------------------\n")

		// Groups
		fmt.Fprintf(os.Stderr, "  Groups:  ")
		for i, g := range merged {
			if i > 0 {
				fmt.Fprintf(os.Stderr, ", ")
			}
			fmt.Fprintf(os.Stderr, "#%s (%s values)", g.Tag, formatNumber(int64(len(g.Values))))
		}
		fmt.Fprintf(os.Stderr, "\n")

		// Pattern
		if *pattern != "" {
			fmt.Fprintf(os.Stderr, "  Pattern: %s\n", *pattern)
		} else if *flex {
			fmt.Fprintf(os.Stderr, "  Mode:    flex (all subsets x permutations)\n")
		} else {
			fmt.Fprintf(os.Stderr, "  Mode:    all permutations\n")
		}

		// Case expand
		if *caseExpand {
			fmt.Fprintf(os.Stderr, "  Case:    expand (UPPER + Camel + lower)\n")
		}

		// Unique
		if *unique {
			fmt.Fprintf(os.Stderr, "  Unique:  enabled\n")
		}

		// Total
		if total < 0 {
			fmt.Fprintf(os.Stderr, "  Total:   overflow (very large)\n")
		} else if *unique {
			fmt.Fprintf(os.Stderr, "  Total:   %s combinations (before dedup)\n", formatNumber(total))
		} else {
			fmt.Fprintf(os.Stderr, "  Total:   %s combinations\n", formatNumber(total))
		}

		// Limits
		if *amount > 0 {
			fmt.Fprintf(os.Stderr, "  Limit:   %s passwords\n", formatNumber(*amount))
		}
		if *maxTime > 0 {
			fmt.Fprintf(os.Stderr, "  Limit:   %v duration\n", *maxTime)
		}

		// Output
		fmt.Fprintf(os.Stderr, "  Output:  %s (%s)\n", outputDest, *outFormat)
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Prompt for confirmation in interactive mode
	if interactive {
		fmt.Fprintf(os.Stderr, "  Press [Enter] to start or [q] to quit: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "q" || line == "Q" {
			fmt.Fprintf(os.Stderr, "  Aborted.\n\n")
			return nil
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Set up output writer
	var w writer.Writer
	var outFile *os.File
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		outFile = f
		defer f.Close()
		switch *outFormat {
		case "csv":
			w = writer.NewCSVWriter(f, []string{"password"})
		default:
			w = writer.NewTXTWriter(f)
		}
	} else {
		switch *outFormat {
		case "csv":
			w = writer.NewCSVWriter(os.Stdout, []string{"password"})
		default:
			w = writer.NewTXTWriter(os.Stdout)
		}
	}

	// Context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Set up TUI controller
	var ctrl *tui.Controller
	if interactive {
		ctrl = tui.NewController(gen, gen.Progress)
		if outFile != nil {
			ctrl.SavePromptFunc = func() bool {
				// Raw mode: use \r\n for newlines; reads return immediately.
				fmt.Fprintf(os.Stderr, "\r\n  Save %s generated passwords to %s? [y/n]: ",
					formatNumber(gen.Progress.Generated()), *output)
				key, _ := ctrl.ReadKey()
				fmt.Fprintf(os.Stderr, "%c\r\n", key) // echo the keypress
				if key == 'n' || key == 'N' {
					outFile.Close()
					os.Remove(*output)
					fmt.Fprintf(os.Stderr, "  Output file removed.\r\n\r\n")
					return false
				}
				return true
			}
		}
	}

	// Run generation pipeline
	out := make(chan string, 4096)
	errCh := make(chan error, 1)

	go func() {
		errCh <- gen.Run(ctx, out)
	}()

	// Start TUI if interactive
	if ctrl != nil {
		go ctrl.Run(ctx)
	}

	// Drain output channel and write
	for pw := range out {
		if err := w.WritePassword(pw); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}

	if err := <-errCh; err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	// Signal TUI completion and wait for it to finish (restore terminal, render final line)
	if ctrl != nil {
		ctrl.Done()
		ctrl.Wait()
	}

	if !*quiet && !interactive {
		fmt.Fprintf(os.Stderr, "\nDone. Generated: %s passwords.\n", formatNumber(gen.Progress.Generated()))
	}

	if outFile != nil && ctrl != nil && ctrl.SaveDeclined {
		outFile.Close()
		os.Remove(*output)
		if !*quiet {
			fmt.Fprintf(os.Stderr, "  Output file removed.\n\n")
		}
	} else if outFile != nil && !*quiet && (ctrl == nil || !ctrl.SaveDeclined) {
		fmt.Fprintf(os.Stderr, "  Output written to %s\n\n", *output)
	}

	return nil
}

// formatNumber formats an integer with comma separators (e.g. 1234567 -> "1,234,567").
func formatNumber(n int64) string {
	if n < 0 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
