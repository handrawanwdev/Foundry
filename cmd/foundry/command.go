package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/progress"
)

const commandUsage = `foundry command: kerjakan command workflow dari coding agent.
Bekerja langsung pada .foundry/project.json; UI yang sedang terbuka ikut berubah.

  foundry command next [--claim]        Command berikutnya. --claim menandainya mulai untuk session ini.
  foundry command show ID [--history]   Instruksi lengkap, atau laporan, status, dan revisinya.
  foundry command start [ID]            Mulai command; ditolak bila dependency belum terverifikasi.
  foundry command progress ID -m TEKS [--percent N] [--file PATH]
  foundry command done ID -m TEKS --file PATH --test NAMA=pass
  foundry command fail|block|review|skip ID --reason TEKS
  foundry command status                Ringkasan workflow.
  foundry command ingest                Proses laporan di .foundry/inbox.

Flag umum: --project FOLDER  --session ID (atau FOUNDRY_SESSION)  --source NAMA (atau FOUNDRY_AGENT)  --json
Laporan: --id ID-STABIL  --file PATH  --change TEKS  --commit SHA  --evidence JENIS:REF  --test NAMA=pass|fail|skip
`

// reportFor maps a subcommand to the status it reports.
var reportFor = map[string]string{
	"progress": core.RunProgress, "done": core.RunDone, "fail": core.RunFailed,
	"block": core.RunBlocked, "review": core.RunReview, "skip": core.RunSkipped,
}

type repeated []string

func (r *repeated) String() string     { return strings.Join(*r, ",") }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }

// runCommand is the CLI of the agent contract. It returns the exit code.
func runCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, commandUsage)
		return 0
	}
	sub, id := args[0], ""
	rest := args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id, rest = rest[0], rest[1:]
	}
	fs := flag.NewFlagSet("command "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("project", "", "folder project (default: folder saat ini)")
	session := fs.String("session", os.Getenv("FOUNDRY_SESSION"), "id session agent")
	source := fs.String("source", os.Getenv("FOUNDRY_AGENT"), "nama agent")
	asJSON := fs.Bool("json", false, "keluaran JSON")
	claim := fs.Bool("claim", false, "tandai mulai untuk session ini")
	history := fs.Bool("history", false, "laporan, status, dan revisi")
	summary := fs.String("m", "", "ringkasan")
	fs.StringVar(summary, "summary", "", "ringkasan")
	reason := fs.String("reason", "", "alasan")
	reportID := fs.String("id", "", "id laporan yang stabil")
	percent := fs.Int("percent", -1, "perkiraan agent, 0 sampai 100")
	commit := fs.String("commit", "", "commit yang memuat pekerjaan")
	var files, changes, evidence, tests repeated
	fs.Var(&files, "file", "file yang diubah")
	fs.Var(&changes, "change", "perubahan")
	fs.Var(&evidence, "evidence", "bukti, JENIS:REF")
	fs.Var(&tests, "test", "hasil test, NAMA=pass|fail|skip")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "foundry command:", err)
		return 1
	}
	if *dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fail(err)
		}
		*dir = wd
	}
	st, err := core.NewStore(*dir)
	if err != nil {
		return fail(err)
	}
	if !st.Exists() {
		return fail(fmt.Errorf("%s bukan project Foundry", st.Dir))
	}
	if *source == "" {
		*source = "cli"
	}
	svc := pipeline.New(st, pipeline.ConfigAI{})
	print := func(v any, text func()) int {
		if *asJSON {
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fail(err)
			}
			fmt.Fprintln(stdout, string(b))
			return 0
		}
		text()
		return 0
	}

	switch sub {
	case "next", "start":
		var out *pipeline.NextResult
		if sub == "start" || *claim {
			out, err = svc.ClaimCommand(id, *session, *source)
		} else {
			out, err = svc.NextCommand(*session)
		}
		if err != nil {
			return fail(err)
		}
		return print(out, func() { printNext(stdout, out) })
	case "show":
		if id == "" {
			return fail(errors.New("sebutkan id command"))
		}
		if *history {
			d, err := svc.CommandDetail(id)
			if err != nil {
				return fail(err)
			}
			return print(d, func() { printHistory(stdout, d) })
		}
		b, err := svc.Command(id)
		if err != nil {
			return fail(err)
		}
		return print(b, func() { printBrief(stdout, b) })
	case "status":
		snap, err := svc.ProgressView()
		if err != nil {
			return fail(err)
		}
		return print(snap, func() { printStatus(stdout, snap) })
	case "ingest":
		applied, rejected, err := svc.IngestInbox()
		if err != nil {
			return fail(err)
		}
		return print(map[string]int{"applied": applied, "rejected": rejected}, func() {
			fmt.Fprintf(stdout, "%d laporan diterima, %d ditolak\n", applied, rejected)
		})
	}

	status, ok := reportFor[sub]
	if !ok {
		fmt.Fprint(stderr, commandUsage)
		return 2
	}
	if id == "" {
		return fail(errors.New("sebutkan id command"))
	}
	in := progress.ReportIn{ID: *reportID, Command: id, Status: status, Summary: *summary, Reason: *reason,
		Files: files, Changes: changes, Session: *session, Source: *source}
	if *percent >= 0 {
		in.Percent = percent
	}
	if *commit != "" {
		in.Evidence = append(in.Evidence, core.Evidence{Kind: "commit", Ref: *commit})
	}
	for _, e := range evidence {
		kind, ref, found := strings.Cut(e, ":")
		if !found {
			kind, ref = "other", e
		}
		in.Evidence = append(in.Evidence, core.Evidence{Kind: kind, Ref: ref})
	}
	for _, t := range tests {
		name, result, found := strings.Cut(t, "=")
		if !found {
			return fail(fmt.Errorf("--test %q: tulis NAMA=pass|fail|skip", t))
		}
		in.Tests = append(in.Tests, core.TestRun{Name: name, Result: result})
	}
	res, err := svc.ReportProgress(in, "agent")
	if err != nil {
		return fail(err)
	}
	return print(res, func() {
		fmt.Fprintf(stdout, "%s: %s\n", res.Command, res.Status)
		if res.Reason != "" {
			fmt.Fprintln(stdout, res.Reason)
		}
		if res.Warning != "" {
			fmt.Fprintln(stdout, "Peringatan:", res.Warning)
		}
	})
}

func printNext(w io.Writer, n *pipeline.NextResult) {
	if n.Command == "" {
		fmt.Fprintln(w, "Tidak ada command untuk dikerjakan:", n.Reason)
		return
	}
	if n.Resumed {
		fmt.Fprintln(w, "Lanjutkan, session ini sudah memegang", n.Command)
	}
	printBrief(w, n.Brief)
}

func printBrief(w io.Writer, b *pipeline.CommandBrief) {
	fmt.Fprintf(w, "Status: %s\n\n%s", b.Progress.Phase, b.Markdown)
	for _, d := range b.Dependencies {
		fmt.Fprintf(w, "\nDependency %s (%s): %s\n", d.ID, d.Status, d.Summary)
	}
	fmt.Fprintln(w, "\n"+b.Report)
}

func printHistory(w io.Writer, d *progress.CommandDetail) {
	for _, r := range d.Reports {
		fmt.Fprintf(w, "%s %s %s %s\n", r.At.Format("2006-01-02 15:04:05"), r.Status, r.Source, r.Summary)
	}
	for _, c := range d.Changes {
		fmt.Fprintf(w, "v%d r%d %s %s: %s\n", c.Version, c.Revision, c.Action, strings.Join(c.Fields, ", "), c.Reason)
	}
}

func printStatus(w io.Writer, s progress.Snapshot) {
	o := s.Overview
	fmt.Fprintf(w, "Workflow v%d: %d%% · %d command\n", s.WorkflowVersion, o.Percent, o.Total)
	fmt.Fprintf(w, "Selesai %d · Dikerjakan %d · Belum %d · Terhambat %d · Gagal %d · Ditinjau %d · Direvisi %d\n",
		o.Completed, o.InProgress, o.Pending, o.Blocked, o.Failed, o.Review, o.Revision)
	for _, c := range s.Commands {
		fmt.Fprintf(w, "%-10s %-18s %s\n", c.ID, c.Phase, c.Name)
	}
}
