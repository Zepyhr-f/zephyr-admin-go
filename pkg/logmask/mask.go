// Package logmask centralises log line masking for the sysadmin log monitor.
//
// Mask rewrites a single log line so that any value associated with a
// sensitive key (password / token / secret / authorization …) is replaced by
// the literal placeholder `***`. The matcher is intentionally lenient: it
// covers shell-style assignments, JSON object fields, and HTTP `Authorization`
// header lines, regardless of casing.
//
// The rule set is deliberately conservative so the function is safe to call
// per-line on streaming downloads. It does NOT promise to catch every
// imaginable encoding of a credential — only the shapes the gateway, auth
// and identity services are known to emit. New sensitive fields can be
// appended to `sensitiveKeys` without touching call sites.
package logmask

import (
	"regexp"
	"strings"
)

// sensitiveKeys is the set of keys whose values we mask. Match is case
// insensitive; word boundaries are enforced by the regexes below.
var sensitiveKeys = []string{
	"password",
	"passwd",
	"pwd",
	"secret",
	"token",
	"api_key",
	"apikey",
	"authorization",
}

var (
	// keyValueRe matches either:
	//   key=value (shell / query-string style, value is run of non-space, non-quote, non-&)
	//   "key": "value"  / "key":"value"  (JSON style)
	// The key is anchored on a non-word boundary so we don't mask a substring
	// like `not_a_password=xxx` when `password` happens to be inside a longer
	// identifier — but we DO mask `user_password=xxx` because the suffix is a
	// real key. Erring on the side of more masking is preferred for log data.
	keyValueRe = regexp.MustCompile(
		`(?i)("?\b(?:` + strings.Join(sensitiveKeys, "|") + `)\b"?)` +
			`(\s*[:=]\s*)` +
			`(?:` +
			`"((?:[^"\\]|\\.)*)"` + // group 3: double-quoted value
			`|` +
			`([^\s,&"}\]]+)` + // group 4: bare value
			`)`,
	)

	// httpHeaderRe matches a stand-alone HTTP header line like
	//   Authorization: Bearer abc.def.ghi
	// when it is at the start of the line or after a `|` separator (go-zero
	// JSON content can embed full request dumps inside a `content` field).
	httpHeaderRe = regexp.MustCompile(
		`(?im)(^|[|\s])(Authorization)(\s*:\s*)([^\r\n]+)`,
	)
)

// Mask rewrites `line` with sensitive values redacted. The function is pure
// and safe for concurrent use.
func Mask(line string) string {
	if line == "" {
		return line
	}

	masked := keyValueRe.ReplaceAllStringFunc(line, func(m string) string {
		sub := keyValueRe.FindStringSubmatch(m)
		// sub[1]=key, sub[2]=sep, sub[3]=quoted-val, sub[4]=bare-val
		if sub[3] != "" || (len(sub) > 3 && strings.Contains(m, `"`)) {
			return sub[1] + sub[2] + `"***"`
		}
		return sub[1] + sub[2] + "***"
	})

	masked = httpHeaderRe.ReplaceAllString(masked, `${1}${2}${3}***`)

	return masked
}
