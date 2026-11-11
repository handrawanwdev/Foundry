package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	tokenusage "github.com/handrawanwdev/foundry/internal/modules/usage"
)

// runUsage prints the token usage the agent CLI reported for the runs of one
// or more projects, summed by run, operation, project, model or day.
func runUsage(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	by := fs.String("by", "operation", strings.Join(tokenusage.Groupings, ", "))
	asJSON := fs.Bool("json", false, "cetak JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dirs := fs.Args()
	if len(dirs) == 0 {
		wd, _ := os.Getwd()
		dirs = []string{wd}
	}
	var recs []tokenusage.Record
	bad := 0
	for _, d := range dirs {
		r, b, err := tokenusage.Read(d)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", d, err)
			return 1
		}
		recs, bad = append(recs, r...), bad+b
	}
	sums, err := tokenusage.Summarize(recs, *by)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *asJSON {
		e := json.NewEncoder(stdout)
		e.SetIndent("", "  ")
		e.Encode(map[string]any{"by": *by, "runs": len(recs), "unreadable_lines": bad, "groups": sums})
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, *by+"\truns\treported\tfailed\tretries\tinput\toutput\tcache_write\tcache_read\ttotal_reported\tcache_read_ratio\tprompt_bytes\t")
	for _, s := range sums {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%.2f\t%d\t\n", s.Key, s.Runs, s.Reported, s.Failed, s.Retries,
			s.InputTokens, s.OutputTokens, s.CacheCreationTokens, s.CacheReadTokens, s.TotalReported, s.CacheReadRatio, s.PromptBytes)
	}
	tw.Flush()
	fmt.Fprintf(stdout, "\nToken = yang dilaporkan CLI; run tanpa laporan (reported < runs) tidak dihitung. prompt_bytes = ukuran terukur, bukan token.\n")
	if bad > 0 {
		fmt.Fprintf(stdout, "%d baris log tidak terbaca dilewati.\n", bad)
	}
	return 0
}
