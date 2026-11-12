package work

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// A detail round writes one command and reads the blueprint artifacts that
// command implements. Read whole, a large artifact (technology plan,
// repository structure) costs tens of KB per round although the command
// needs a few of its sections. scopeArtifact keeps the sections that concern
// the command, ranked by the words and ids they share with it, within a
// budget; the headings of the sections left out are listed, so the planner
// knows they exist. A small artifact is always read whole.

// mdSection is one heading and its own text, up to the next heading.
type mdSection struct {
	heading string // the heading line; "" for the text before the first one
	level   int
	parent  int // enclosing section, -1 at top level
	text    string
}

var mdHeading = regexp.MustCompile(`^(#{1,6})\s+\S`)

// splitSections cuts Markdown at its headings; a heading inside a code
// fence is text.
func splitSections(body string) []mdSection {
	secs := []mdSection{{parent: -1}}
	var cur strings.Builder
	fence := ""
	flush := func() { secs[len(secs)-1].text = cur.String(); cur.Reset() }
	for line := range strings.SplitAfterSeq(body, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case fence != "":
			if strings.HasPrefix(trim, fence) {
				fence = ""
			}
		case strings.HasPrefix(trim, "```"), strings.HasPrefix(trim, "~~~"):
			fence = trim[:3]
		default:
			if m := mdHeading.FindStringSubmatch(line); m != nil {
				flush()
				level := len(m[1])
				parent := len(secs) - 1
				for parent >= 0 && (secs[parent].level >= level || secs[parent].level == 0) {
					if secs[parent].level == 0 {
						parent = -1
						break
					}
					parent = secs[parent].parent
				}
				secs = append(secs, mdSection{heading: strings.TrimRight(line, "\n"), level: level, parent: parent})
			}
		}
		cur.WriteString(line)
	}
	flush()
	return secs
}

// scopeQuery is what a command is about: weighted words and exact ids.
type scopeQuery struct {
	words   map[string]float64
	anchors []string // ids such as REQ-031, FEAT-016.2, DEC-004
}

var anchorID = regexp.MustCompile(`\b[A-Z]{2,6}-\d+(?:\.\d+)?(?:-AC\d+)?\b`)

// stopwords are frequent words that say nothing about a section's topic.
var stopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`yang dan untuk dengan dari pada atau ini itu tidak akan dalam oleh sebagai adalah setiap harus bisa dapat juga agar saat jika bila tanpa lebih hanya semua serta sudah belum ada satu dua per the and for with from that this not will into when each must can are was were has have its all any via use used using`) {
		stopwords[w] = true
	}
}

func scopeWords(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(w)) >= 3 && !stopwords[w] {
			out = append(out, w)
		}
	}
	return out
}

// add puts text into the query with weight w.
func (q *scopeQuery) add(text string, w float64) {
	if q.words == nil {
		q.words = map[string]float64{}
	}
	for _, t := range scopeWords(text) {
		q.words[t] = max(q.words[t], w)
	}
	for _, id := range anchorID.FindAllString(text, -1) {
		if !slices.Contains(q.anchors, id) {
			q.anchors = append(q.anchors, id)
		}
	}
}

// rank scores each section against q (BM25 over the artifact's sections,
// plus a bonus per id the section names).
func rank(secs []mdSection, q scopeQuery) []float64 {
	const k1, b, anchorBonus = 1.2, 0.75, 3.0
	n := float64(len(secs))
	tf := make([]map[string]int, len(secs))
	df := map[string]int{}
	total := 0
	lengths := make([]int, len(secs))
	for i, s := range secs {
		tf[i] = map[string]int{}
		words := scopeWords(s.heading + "\n" + s.text)
		lengths[i] = len(words)
		total += len(words)
		for _, w := range words {
			if _, ok := q.words[w]; ok {
				if tf[i][w] == 0 {
					df[w]++
				}
				tf[i][w]++
			}
		}
	}
	avg := math.Max(1, float64(total)/math.Max(1, n))
	scores := make([]float64, len(secs))
	for i, s := range secs {
		for w, f := range tf[i] {
			idf := math.Log(1 + (n-float64(df[w])+0.5)/(float64(df[w])+0.5))
			ff := float64(f)
			scores[i] += q.words[w] * idf * ff * (k1 + 1) / (ff + k1*(1-b+b*float64(lengths[i])/avg))
		}
		// The heading names the topic: a word in it counts again.
		for _, w := range scopeWords(s.heading) {
			if wt, ok := q.words[w]; ok {
				scores[i] += wt
			}
		}
		for _, id := range q.anchors {
			if strings.Contains(s.text, id) || strings.Contains(s.heading, id) {
				scores[i] += anchorBonus
			}
		}
	}
	return scores
}

// maxOmittedList bounds the list of left-out headings; minCutSection is the
// least room worth filling with a cut section.
const (
	maxOmittedList = 1200
	minCutSection  = 600
)

// scopeArtifact returns body whole when it fits budget, otherwise the
// sections that concern q in document order, each with the headings above
// it, and a list of the headings left out.
func scopeArtifact(body string, q scopeQuery, budget int) string {
	return scopeArtifactSkip(body, q, budget, nil)
}

// scopeArtifactSkip is scopeArtifact that never keeps a section whose
// heading skip matches (the prompt carries that part elsewhere).
func scopeArtifactSkip(body string, q scopeQuery, budget int, skip func(heading string) bool) string {
	if len(body) <= budget && skip == nil {
		return body
	}
	secs := splitSections(body)
	scores := rank(secs, q)
	if skip != nil {
		for i := 1; i < len(secs); i++ {
			if skip(secs[i].heading) {
				scores[i] = -1
			}
		}
	}
	// What is left after the skipped sections fits: it is read whole.
	rest := 0
	for i, sec := range secs {
		if scores[i] >= 0 {
			rest += len(sec.text)
		}
	}
	keep := make([]bool, len(secs))
	used := 0
	// The text before the first heading says what the artifact is.
	if len(secs[0].text) <= budget/8 || rest <= budget {
		keep[0] = true
		used += len(secs[0].text)
	}
	order := make([]int, 0, len(secs))
	for i, sec := range secs {
		if i == 0 || headingOnly(sec) {
			continue // a heading alone is shown above the sections under it
		}
		if scores[i] > 0 || (rest <= budget && scores[i] == 0) {
			order = append(order, i)
		}
	}
	slices.SortStableFunc(order, func(a, b int) int {
		switch {
		case scores[a] > scores[b]:
			return -1
		case scores[a] < scores[b]:
			return 1
		}
		return a - b
	})
	// In score order: a section that fits is kept whole; the first of the
	// two best that does not fit is kept cut to part of the room left, so
	// the most relevant text is never lost to its length.
	cut := map[int]string{}
	for rankPos, i := range order {
		size := len(secs[i].text)
		if used+size <= budget {
			keep[i] = true
			used += size
			continue
		}
		room := (budget - used) * 2 / 3
		if len(cut) > 0 || rankPos > 1 || room < minCutSection {
			continue
		}
		text := secs[i].text[:room]
		if n := strings.LastIndexByte(text, '\n'); n > room/2 {
			text = text[:n+1]
		} else if n := strings.LastIndexByte(text, ' '); n > 0 {
			text = text[:n] + " "
		}
		cut[i] = strings.ToValidUTF8(text, "") + "…(dipotong)\n"
		keep[i] = true
		used += len(cut[i])
	}
	var out, omitted strings.Builder
	shown := make([]bool, len(secs))
	var showParents func(i int)
	showParents = func(i int) {
		p := secs[i].parent
		if p < 0 || shown[p] {
			return
		}
		showParents(p)
		if !keep[p] {
			out.WriteString(secs[p].heading + "\n")
		}
		shown[p] = true
	}
	left, skipped := 0, 0
	for i, s := range secs {
		if keep[i] {
			showParents(i)
			if c, ok := cut[i]; ok {
				out.WriteString(c)
			} else {
				out.WriteString(s.text)
			}
			shown[i] = true
			continue
		}
		if i == 0 || headingOnly(s) {
			continue // no text of its own: its subsections speak for it
		}
		if scores[i] < 0 {
			skipped++
			continue
		}
		left++
		if omitted.Len() < maxOmittedList {
			if omitted.Len() > 0 {
				omitted.WriteString("; ")
			}
			omitted.WriteString(strings.TrimSpace(strings.TrimLeft(s.heading, "#")))
		}
	}
	if skipped > 0 {
		fmt.Fprintf(&out, "\n(%d sections given elsewhere in this prompt or about other commands are left out)\n", skipped)
	}
	if left > 0 {
		list := omitted.String()
		if len(list) >= maxOmittedList {
			list += "; …"
		}
		fmt.Fprintf(&out, "\n(%d sections left out as not about this command: %s)\n", left, list)
	}
	return out.String()
}

func headingOnly(s mdSection) bool { return strings.TrimSpace(s.text) == strings.TrimSpace(s.heading) }
