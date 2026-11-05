package cardigann

import (
	"fmt"
	"strconv"
	"strings"

	cdefinition "github.com/Kcchouette/cardigann-go/definition"
	tmpl "github.com/Kcchouette/cardigann-go/template"
)

// applyDefinitionCompat patches a parsed Cardigann definition for gaps in
// cardigann-go v0.8.0 that break Prowlarr/Jackett YAML:
//
//  1. HTTP path/before methods are ToUpper'd without template rendering, so
//     `method: "{{ if .Keywords }}post{{ else }}get{{ end }}"` becomes the
//     literal invalid method "{{ IF .KEYWORDS }}POST…".
//  2. Prowlarr regex templates keep single backslashes after YAML decode
//     (`"\b0(\d{1})\b"`). Go text/template re-parses those as Go string
//     literals, so `\d` is invalid syntax. Re-quote them for the Go parser.
//  3. The engine maps only the `download` field into DownloadLink and ignores
//     `magnet`. Promote magnet as a download fallback (Nyaa, etc.).
//  4. fieldsToRelease only copies `seeders`/`peers` — it drops `leechers`.
//     Prowlarr YAML always uses `leechers`; promote it to `peers` so the
//     value survives (search.go treats it as leechers-only and builds total).
//  5. Prowlarr special-cases `size` via ParseUtil.GetBytes and `date` via
//     DateTimeUtil.FromUnknown. cardigann-go only strconv.ParseInt / RFC3339,
//     so human sizes (Nyaa "1.2 GiB") and unix epochs (YTS) become 0/now.
//     Append our registered `parsebytes` / `normalizedate` filters.
func applyDefinitionCompat(def *cdefinition.IndexerDefinition, rctx tmpl.Context) error {
	if def == nil {
		return nil
	}
	fixSearchTemplates(&def.Search)
	promoteMagnetDownload(&def.Search)
	promoteLeechersToPeers(&def.Search)
	promoteSizeAndDateParsers(&def.Search)
	fixLoginTemplates(&def.Login)
	if def.Download != nil {
		fixDownloadTemplates(def.Download)
	}
	return resolveTemplatedMethods(def, rctx)
}

// promoteMagnetDownload makes magnet links usable when cardigann-go only reads
// the `download` field. If `download` already exists, keep its value and fall
// back to magnet when empty (prefer_magnet / missing .torrent link).
func promoteMagnetDownload(s *cdefinition.Search) {
	magnet, hasMagnet := s.Fields["magnet"]
	if !hasMagnet {
		return
	}
	if _, hasDownload := s.Fields["download"]; !hasDownload {
		s.Fields["download"] = magnet
		return
	}
	s.Fields["_download_pref"] = s.Fields["download"]
	s.Fields["download"] = cdefinition.Field{
		Text:     `{{ if .Result._download_pref }}{{ .Result._download_pref }}{{ else }}{{ .Result.magnet }}{{ end }}`,
		Optional: true,
	}
}

// promoteLeechersToPeers copies the Cardigann `leechers` field onto `peers`
// when `peers` is absent. cardigann-go's fieldsToRelease ignores `leechers`
// (Prowlarr instead accumulates leechers into Torznab peers). The catalog has
// hundreds of `leechers` selectors and zero `peers` field names.
func promoteLeechersToPeers(s *cdefinition.Search) {
	leechers, hasLeechers := s.Fields["leechers"]
	if !hasLeechers {
		return
	}
	if _, hasPeers := s.Fields["peers"]; hasPeers {
		return
	}
	s.Fields["peers"] = leechers
}

// promoteSizeAndDateParsers appends Replacearr filters that restore Prowlarr
// field-parser behavior for size and publish date.
func promoteSizeAndDateParsers(s *cdefinition.Search) {
	if field, ok := s.Fields["size"]; ok {
		field.Filters = appendFilter(field.Filters, "parsebytes")
		s.Fields["size"] = field
	}
	if field, ok := s.Fields["date"]; ok {
		field.Filters = appendFilter(field.Filters, "normalizedate")
		s.Fields["date"] = field
	}
}

func fixSearchTemplates(s *cdefinition.Search) {
	for i := range s.Paths {
		p := &s.Paths[i]
		p.Path = fixTemplateString(p.Path)
		p.Method = fixTemplateString(p.Method)
		fixStringMap(p.Inputs)
	}
	for i := range s.Before {
		b := &s.Before[i]
		b.Path = fixTemplateString(b.Path)
		b.Method = fixTemplateString(b.Method)
		fixStringMap(b.Inputs)
	}
	fixStringMap(s.Inputs)
	fixHeaderMap(s.Headers)
	fixFilterSteps(s.KeywordsFilters)
	for name, field := range s.Fields {
		field.Text = fixTemplateString(field.Text)
		field.Default = fixTemplateString(field.Default)
		fixFilterSteps(field.Filters)
		s.Fields[name] = field
	}
}

func fixLoginTemplates(login *cdefinition.Login) {
	login.Path = fixTemplateString(login.Path)
	login.SubmitPath = fixTemplateString(login.SubmitPath)
	fixStringMap(login.Inputs)
	fixHeaderMap(login.Headers)
}

func fixDownloadTemplates(d *cdefinition.Download) {
	d.Method = fixTemplateString(d.Method)
	fixHeaderMap(d.Headers)
	for i := range d.Before {
		step := &d.Before[i]
		step.Path = fixTemplateString(step.Path)
		step.Method = fixTemplateString(step.Method)
		fixStringMap(step.Inputs)
	}
	for i := range d.Selectors {
		fixFilterSteps(d.Selectors[i].Filters)
	}
}

func fixStringMap(m map[string]string) {
	for k, v := range m {
		m[k] = fixTemplateString(v)
	}
}

func fixHeaderMap(m map[string][]string) {
	for k, vals := range m {
		for i, v := range vals {
			vals[i] = fixTemplateString(v)
		}
		m[k] = vals
	}
}

func fixFilterSteps(steps []cdefinition.FilterStep) {
	for i := range steps {
		for j, arg := range steps[i].Args {
			steps[i].Args[j] = fixTemplateString(arg)
		}
	}
}

func resolveTemplatedMethods(def *cdefinition.IndexerDefinition, rctx tmpl.Context) error {
	for i := range def.Search.Paths {
		method, err := renderMethod(def.Search.Paths[i].Method, rctx)
		if err != nil {
			return fmt.Errorf("search path %d method: %w", i, err)
		}
		def.Search.Paths[i].Method = method
	}
	for i := range def.Search.Before {
		method, err := renderMethod(def.Search.Before[i].Method, rctx)
		if err != nil {
			return fmt.Errorf("search before %d method: %w", i, err)
		}
		def.Search.Before[i].Method = method
	}
	if def.Download != nil {
		method, err := renderMethod(def.Download.Method, rctx)
		if err != nil {
			return fmt.Errorf("download method: %w", err)
		}
		def.Download.Method = method
		for i := range def.Download.Before {
			method, err := renderMethod(def.Download.Before[i].Method, rctx)
			if err != nil {
				return fmt.Errorf("download before %d method: %w", i, err)
			}
			def.Download.Before[i].Method = method
		}
	}
	return nil
}

func renderMethod(method string, rctx tmpl.Context) (string, error) {
	method = strings.TrimSpace(method)
	if method == "" || !strings.Contains(method, "{{") {
		return method, nil
	}
	rendered, err := tmpl.Render(method, rctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(rendered), nil
}

// fixTemplateString re-encodes double-quoted literals inside {{ }} actions so
// Prowlarr-style regex escapes survive Go text/template parsing.
func fixTemplateString(src string) string {
	if src == "" || !strings.Contains(src, "{{") || !strings.Contains(src, `"`) {
		return src
	}
	var b strings.Builder
	rest := src
	for {
		open := strings.Index(rest, "{{")
		if open < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:open+2])
		rest = rest[open+2:]
		close := strings.Index(rest, "}}")
		if close < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(fixActionQuotes(rest[:close]))
		b.WriteString("}}")
		rest = rest[close+2:]
	}
	return b.String()
}

// fixActionQuotes rewrites "..." literals inside one template action body.
func fixActionQuotes(action string) string {
	var b strings.Builder
	i := 0
	for i < len(action) {
		if action[i] != '"' {
			b.WriteByte(action[i])
			i++
			continue
		}
		content, next, ok := lenientUnquote(action[i:])
		if !ok {
			b.WriteByte(action[i])
			i++
			continue
		}
		b.WriteString(strconv.Quote(content))
		i += next
	}
	return b.String()
}

// lenientUnquote parses a Cardigann/Prowlarr double-quoted template literal.
// Only \" and \\ are treated as escapes; sequences like \d and \b stay as
// backslash + letter (regex intent), unlike Go's strconv.Unquote.
func lenientUnquote(s string) (content string, n int, ok bool) {
	if len(s) < 2 || s[0] != '"' {
		return "", 0, false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), i + 1, true
		}
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '"' || next == '\\' {
				b.WriteByte(next)
				i += 2
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(next)
			i += 2
			continue
		}
		if c == '\\' {
			return "", 0, false
		}
		b.WriteByte(c)
		i++
	}
	return "", 0, false
}
