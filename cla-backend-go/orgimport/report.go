// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// MaxRawEmailBytes is the SES SendRawEmail limit (v1 API).
	MaxRawEmailBytes = 10 * 1024 * 1024
	// maxZipAttachment keeps the whole message under the SES limit with base64 overhead.
	maxZipAttachment = 6 * 1024 * 1024
	// maxInlinePlanRows caps the inline plan table; plan.csv is attached in full.
	maxInlinePlanRows = 2000
	// maxLogTailBytes caps the inline run.log tail; run.log is attached in full.
	maxLogTailBytes = 96 * 1024
)

// RunInfo describes one CLI run for the report e-mail and the AWS log header.
type RunInfo struct {
	Stage      string
	Command    string
	Apply      bool
	Args       []string
	Start      time.Time
	End        time.Time
	Runner     string
	RunURL     string
	Repository string
	Artifact   string
	Revision   string
	OutDir     string
	LogGroup   string
	LogStream  string
	Summary    string
	Err        string
	Workflow   WorkflowInputs
}

// WorkflowInputs mirrors the org-import-sweep.yml dispatch inputs for the "how to apply" command.
type WorkflowInputs struct {
	Routes        string
	Tranche       string
	IDs           string
	Mapping       string
	Decisions     string
	SharedDomains string
}

// Mode is dry-run or apply.
func (i RunInfo) Mode() string {
	if i.Apply {
		return ModeApply
	}
	return ModeDryRun
}

// Failed reports whether the run ended with an error.
func (i RunInfo) Failed() bool { return i.Err != "" }

// Attachment is one e-mail attachment.
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// Report is the complete decision record of a run: subject, HTML and text bodies, attachments.
type Report struct {
	Subject     string
	HTML        string
	Text        string
	Attachments []Attachment
}

// BuildReport renders the run as a self-contained decision record. plan or audit may be nil.
func BuildReport(info RunInfo, plan *Plan, audit *AuditResult, runLog []byte) Report {
	status := "OK"
	if info.Failed() {
		status = "FAILED"
	}
	summary := info.Summary
	if summary == "" {
		summary = "no summary"
	}
	r := Report{Subject: fmt.Sprintf("[EasyCLA org-import][%s] %s %s: %s (%s)", info.Stage, info.Command, info.Mode(), summary, status)}

	var h, t strings.Builder
	h.WriteString("<html><body style=\"font-family:Arial,Helvetica,sans-serif;font-size:13px\">")
	fmt.Fprintf(&h, "<h2>EasyCLA org-import: %s %s on %s &mdash; %s</h2>", esc(info.Command), esc(info.Mode()), esc(info.Stage), status)
	fmt.Fprintf(&t, "EasyCLA org-import: %s %s on %s - %s\n\n", info.Command, info.Mode(), info.Stage, status)

	header := [][2]string{
		{"Stage", info.Stage}, {"Command", info.Command}, {"Mode", info.Mode()}, {"Result", status},
		{"Summary", summary},
		{"Started (UTC)", info.Start.UTC().Format(time.RFC3339)}, {"Finished (UTC)", info.End.UTC().Format(time.RFC3339)},
		{"Duration", info.End.Sub(info.Start).Round(time.Second).String()},
		{"Runner", info.Runner}, {"Arguments", strings.Join(info.Args, " ")},
		{"Build revision", info.Revision}, {"Output directory", info.OutDir},
	}
	if info.RunURL != "" {
		header = append(header, [2]string{"GitHub Actions run", info.RunURL})
	}
	if info.Artifact != "" {
		header = append(header, [2]string{"Actions artifact", info.Artifact})
	}
	if info.LogGroup != "" {
		header = append(header, [2]string{"CloudWatch Logs", info.LogGroup + " / " + info.LogStream})
	}
	if info.Err != "" {
		header = append(header, [2]string{"Error", info.Err})
	}
	h.WriteString("<table border=\"1\" cellpadding=\"4\" cellspacing=\"0\">")
	for _, kv := range header {
		v := esc(kv[1])
		if kv[0] == "GitHub Actions run" {
			v = fmt.Sprintf("<a href=\"%s\">%s</a>", v, v)
		}
		fmt.Fprintf(&h, "<tr><th align=\"left\">%s</th><td>%s</td></tr>", esc(kv[0]), v)
		fmt.Fprintf(&t, "%-20s %s\n", kv[0]+":", kv[1])
	}
	h.WriteString("</table>")
	t.WriteString("\n")

	if plan != nil {
		writePlanSections(&h, &t, info, plan)
	}
	if audit != nil {
		writeAuditSections(&h, &t, audit)
	}

	tail := logTail(runLog)
	fmt.Fprintf(&h, "<h3>run.log%s</h3><pre style=\"font-size:11px;white-space:pre-wrap\">%s</pre>", tailNote(runLog, tail), esc(string(tail)))
	fmt.Fprintf(&t, "== run.log%s ==\n%s\n", tailNote(runLog, tail), string(tail))

	r.Attachments = attachments(info.OutDir, runLog)
	if len(r.Attachments) > 0 {
		h.WriteString("<h3>Attachments</h3><ul>")
		t.WriteString("\n== Attachments ==\n")
		for _, a := range r.Attachments {
			fmt.Fprintf(&h, "<li>%s (%d bytes)</li>", esc(a.Name), len(a.Data))
			fmt.Fprintf(&t, "- %s (%d bytes)\n", a.Name, len(a.Data))
		}
		h.WriteString("</ul>")
	}
	h.WriteString("</body></html>")
	r.HTML, r.Text = h.String(), t.String()
	return r
}

func writePlanSections(h, t *strings.Builder, info RunInfo, plan *Plan) {
	actions := plan.ManualActions()
	fmt.Fprintf(h, "<h3>Manual actions (%d)</h3>", len(actions))
	fmt.Fprintf(t, "== Manual actions (%d) ==\n", len(actions))
	if len(actions) == 0 {
		h.WriteString("<p>None: every eligible group is registered, rewritten or ready.</p>")
		t.WriteString("None.\n")
	} else {
		h.WriteString("<p>Each row needs a human before the tool can act on it; the suggested action says how.</p>")
		writeTable(h, t, []string{"key", "old id", "route", "reason", "suggested action", "live", "org-service", "website", "domain", "new id", "action", "company ids", "company names", "error"}, manualRows(plan, actions))
	}

	if len(plan.Targets) > 0 {
		fmt.Fprintf(h, "<h3>Targets (%d)</h3><p>Rewrite destinations grouped by Salesforce Account. <code>needs_decision</code> requires a collapse or distinct decision in the decisions file (lfx-self-serve #3085).</p>", len(plan.Targets))
		fmt.Fprintf(t, "\n== Targets (%d) ==\n", len(plan.Targets))
		var rows [][]string
		for _, tg := range plan.Targets {
			decision, reviewer := "", ""
			if tg.Decision != nil {
				decision, reviewer = tg.Decision.Kind, tg.Decision.Reviewer
			}
			var ids, names []string
			for _, g := range tg.Groups {
				ids = append(ids, g.CompanyIDs()...)
				names = append(names, g.Names()...)
			}
			rows = append(rows, []string{tg.SFID, strconv.Itoa(len(tg.Groups)), strings.Join(tg.OldIDs(), "; "), strings.Join(ids, "; "), strings.Join(names, "; "), strconv.Itoa(len(tg.Existing)), decision, reviewer, tg.Status})
		}
		writeTable(h, t, []string{"target", "groups", "old ids", "company ids", "company names", "existing rows", "decision", "reviewer", "status"}, rows)
	}

	fmt.Fprintf(h, "<h3>Plan (%d groups: %d register, %d rewrite)</h3>", len(plan.Groups), len(plan.Register), len(plan.Rewrite))
	fmt.Fprintf(t, "\n== Plan (%d groups: %d register, %d rewrite) ==\n", len(plan.Groups), len(plan.Register), len(plan.Rewrite))
	var rows [][]string
	for i, g := range plan.Groups {
		if i == maxInlinePlanRows {
			fmt.Fprintf(h, "<p>Only the first %d groups are shown inline; plan.csv (attached) has all %d.</p>", maxInlinePlanRows, len(plan.Groups))
			fmt.Fprintf(t, "(only the first %d groups shown; plan.csv has all %d)\n", maxInlinePlanRows, len(plan.Groups))
			break
		}
		domain, shared := plan.domainOf(g)
		decision := ""
		if g.Decision != nil {
			decision = g.Decision.Kind + " by " + g.Decision.Reviewer
		}
		errText := ""
		if g.Err != nil {
			errText = g.Err.Error()
		}
		rows = append(rows, []string{g.Key, string(g.Shape), string(g.Route), g.ManualReason, g.Live, g.OrgStatus, g.Website(), domain, shared, g.NewID, g.Action, decision, strings.Join(g.CompanyIDs(), "; "), strings.Join(g.Names(), "; "), errText})
	}
	writeTable(h, t, []string{"key", "shape", "route", "reason", "live", "org-service", "website", "domain", "shared", "new id", "action", "decision", "company ids", "company names", "error"}, rows)

	if !info.Apply {
		h.WriteString("<h3>How to apply this plan</h3><p>After reviewing the record above, re-run the same command in apply mode. Locally:</p>")
		t.WriteString("\n== How to apply this plan ==\nLocally:\n")
		for _, c := range ApplyCommands(info) {
			fmt.Fprintf(h, "<pre style=\"background:#f4f4f4;padding:6px;white-space:pre-wrap\">%s</pre>", esc(c))
			fmt.Fprintf(t, "  %s\n", c)
		}
	}
}

func manualRows(plan *Plan, actions []ManualAction) [][]string {
	var rows [][]string
	for _, a := range actions {
		g := a.Group
		errText := ""
		if g.Err != nil {
			errText = g.Err.Error()
		}
		domain, _ := plan.domainOf(g)
		rows = append(rows, []string{g.Key, g.OldID, string(g.Route), a.Reason, a.Suggested, g.Live, g.OrgStatus, g.Website(), domain, g.NewID, g.Action, strings.Join(g.CompanyIDs(), "; "), strings.Join(g.Names(), "; "), errText})
	}
	return rows
}

func writeAuditSections(h, t *strings.Builder, audit *AuditResult) {
	fmt.Fprintf(h, "<h3>Audit (%d company rows)</h3>", len(audit.Rows))
	fmt.Fprintf(t, "\n== Audit (%d company rows) ==\n", len(audit.Rows))
	var rows [][]string
	for _, k := range sortedKeys(audit.Tiers) {
		rows = append(rows, []string{"tier", k, strconv.Itoa(audit.Tiers[k])})
	}
	routes := make([]string, 0, len(audit.Routes))
	for r := range audit.Routes {
		routes = append(routes, string(r))
	}
	sort.Strings(routes)
	for _, r := range routes {
		rows = append(rows, []string{"route", r, strconv.Itoa(audit.Routes[Route(r)])})
	}
	rows = append(rows, []string{"possible duplicate groups", "", strconv.Itoa(len(audit.Duplicates))})
	writeTable(h, t, []string{"kind", "value", "count"}, rows)
	h.WriteString("<p>audit.csv, unresolvable.csv and possible_duplicates.csv are attached (zip).</p>")
}

// ApplyCommands returns the exact local and GitHub Actions commands that re-run this plan in apply mode.
func ApplyCommands(info RunInfo) []string {
	var args []string
	for _, a := range info.Args {
		switch a {
		case "--apply", "-apply", "--yes", "-yes", "--no-email", "-no-email", "--no-aws-log", "-no-aws-log":
			continue
		}
		args = append(args, shellQuote(a))
	}
	local := fmt.Sprintf("STAGE=%s ./bin/org-import %s --apply --yes", info.Stage, strings.Join(args, " "))
	repo := info.Repository
	if repo == "" {
		repo = "linuxfoundation/easycla"
	}
	w := info.Workflow
	gh := fmt.Sprintf("gh workflow run org-import-sweep.yml -R %s -f stage=%s -f mode=apply", repo, info.Stage)
	if w.Routes != "" {
		gh += " -f routes=" + shellQuote(w.Routes)
	}
	if w.Tranche != "" && w.Tranche != "0" {
		gh += " -f tranche=" + shellQuote(w.Tranche)
	}
	if w.IDs != "" {
		gh += " -f ids=" + shellQuote(w.IDs)
	}
	for _, f := range [][2]string{{"mapping", w.Mapping}, {"decisions", w.Decisions}, {"shared_domains", w.SharedDomains}} {
		if f[1] != "" {
			gh += fmt.Sprintf(" -f %s=\"$(tr '\\n' '|' < %s)\"", f[0], shellQuote(f[1]))
		}
	}
	return []string{local, gh}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, " \t\n'\"$`\\!*?[]{}()<>|&;#~") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func writeTable(h, t *strings.Builder, head []string, rows [][]string) {
	h.WriteString("<table border=\"1\" cellpadding=\"3\" cellspacing=\"0\" style=\"font-size:11px;border-collapse:collapse\"><tr>")
	for _, c := range head {
		fmt.Fprintf(h, "<th align=\"left\">%s</th>", esc(c))
	}
	h.WriteString("</tr>")
	t.WriteString(strings.Join(head, " | ") + "\n")
	for _, row := range rows {
		h.WriteString("<tr>")
		for _, c := range row {
			fmt.Fprintf(h, "<td>%s</td>", esc(c))
		}
		h.WriteString("</tr>")
		t.WriteString(strings.Join(row, " | ") + "\n")
	}
	h.WriteString("</table>")
}

func esc(s string) string { return html.EscapeString(s) }

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func logTail(runLog []byte) []byte {
	if len(runLog) <= maxLogTailBytes {
		return runLog
	}
	tail := runLog[len(runLog)-maxLogTailBytes:]
	if i := bytes.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	return tail
}

func tailNote(runLog, tail []byte) string {
	if len(tail) == len(runLog) {
		return ""
	}
	return fmt.Sprintf(" (last %d of %d bytes; full log attached)", len(tail), len(runLog))
}

// attachments returns manual_actions.csv and plan.csv (or audit.csv) inline, the run log, and a zip
// of the whole output directory when it fits.
func attachments(outDir string, runLog []byte) []Attachment {
	var out []Attachment
	for _, name := range []string{"manual_actions.csv", "plan.csv", "targets.csv", "to_salesforce.csv", "audit.csv", "unresolvable.csv", "possible_duplicates.csv"} {
		data, err := os.ReadFile(filepath.Clean(filepath.Join(outDir, name)))
		if err == nil && len(data) > 0 {
			out = append(out, Attachment{Name: name, ContentType: "text/csv", Data: data})
		}
	}
	if len(runLog) > 0 {
		out = append(out, Attachment{Name: "run.log", ContentType: "text/plain", Data: runLog})
	}
	if zipped, err := ZipDir(outDir); err == nil && len(zipped) > 0 && len(zipped) <= maxZipAttachment {
		out = append(out, Attachment{Name: filepath.Base(outDir) + ".zip", ContentType: "application/zip", Data: zipped})
	}
	return out
}

// ZipDir zips the regular files directly under dir (reports, state, run.log).
func ZipDir(dir string) ([]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		data, readErr := os.ReadFile(filepath.Clean(filepath.Join(dir, e.Name())))
		if readErr != nil {
			return nil, readErr
		}
		w, createErr := zw.Create(e.Name())
		if createErr != nil {
			return nil, createErr
		}
		if _, err = w.Write(data); err != nil {
			return nil, err
		}
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MIME renders the report as a raw RFC 5322 message (multipart/mixed with a multipart/alternative body).
// Attachments are dropped largest-first until the message fits MaxRawEmailBytes.
func (r Report) MIME(from string, to []string) ([]byte, error) {
	atts := append([]Attachment(nil), r.Attachments...)
	for {
		msg, err := r.mime(from, to, atts)
		if err != nil {
			return nil, err
		}
		if len(msg) <= MaxRawEmailBytes || len(atts) == 0 {
			return msg, nil
		}
		largest := 0
		for i, a := range atts {
			if len(a.Data) > len(atts[largest].Data) {
				largest = i
			}
		}
		atts = append(atts[:largest], atts[largest+1:]...)
	}
}

func (r Report) mime(from string, to []string, atts []Attachment) ([]byte, error) {
	var buf bytes.Buffer
	mixed := multipart.NewWriter(&buf)
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", r.Subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	buf.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mixed.Boundary())

	var alt bytes.Buffer
	altW := multipart.NewWriter(&alt)
	for _, part := range []struct{ ctype, body string }{{"text/plain; charset=utf-8", r.Text}, {"text/html; charset=utf-8", r.HTML}} {
		hdr := textproto.MIMEHeader{"Content-Type": {part.ctype}, "Content-Transfer-Encoding": {"base64"}}
		w, err := altW.CreatePart(hdr)
		if err != nil {
			return nil, err
		}
		if err = writeBase64(w, []byte(part.body)); err != nil {
			return nil, err
		}
	}
	if err := altW.Close(); err != nil {
		return nil, err
	}
	bodyPart, err := mixed.CreatePart(textproto.MIMEHeader{"Content-Type": {fmt.Sprintf("multipart/alternative; boundary=%q", altW.Boundary())}})
	if err != nil {
		return nil, err
	}
	if _, err = bodyPart.Write(alt.Bytes()); err != nil {
		return nil, err
	}
	for _, a := range atts {
		hdr := textproto.MIMEHeader{
			"Content-Type":              {fmt.Sprintf("%s; name=%q", a.ContentType, a.Name)},
			"Content-Disposition":       {fmt.Sprintf("attachment; filename=%q", a.Name)},
			"Content-Transfer-Encoding": {"base64"},
		}
		w, partErr := mixed.CreatePart(hdr)
		if partErr != nil {
			return nil, partErr
		}
		if partErr = writeBase64(w, a.Data); partErr != nil {
			return nil, partErr
		}
	}
	if err = mixed.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeBase64(w io.Writer, data []byte) error {
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 0 {
		n := 76
		if n > len(enc) {
			n = len(enc)
		}
		if _, err := io.WriteString(w, enc[:n]+"\r\n"); err != nil {
			return err
		}
		enc = enc[n:]
	}
	return nil
}

// ParseRecipients splits a comma/semicolon/whitespace separated address list.
func ParseRecipients(s string) []string {
	var out []string
	for _, a := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' }) {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}
