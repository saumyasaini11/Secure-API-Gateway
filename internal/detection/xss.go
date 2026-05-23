package detection

import (
	"regexp"
	"strings"
)

// xssPattern covers common cross-site scripting vectors including
// tag injection, event handlers, javascript: URIs, and URL-encoded variants.
var xssPattern = regexp.MustCompile(
	`(?i)(` +
		`<script[\s>/]` + // <script> / <script/> / <script >
		`|</script>` + // </script>
		`|javascript\s*:` + // javascript: URI scheme
		`|\bon\w+\s*=` + // onerror=, onload=, onclick=, onmouseover=, etc.
		`|<iframe[\s>/]` + // <iframe>
		`|<object[\s>/]` + // <object>
		`|<embed[\s>/]` + // <embed>
		`|<svg[\s>/]` + // <svg> (often used as XSS vector)
		`|<img[^>]+src\s*=\s*["\']?\s*javascript` + // <img src="javascript:...">
		`|\beval\s*\(` + // eval()
		`|document\.cookie` + // cookie theft
		`|document\.write\s*\(` + // document.write()
		`|window\.location` + // redirect hijack
		`|&#x[0-9a-f]+;` + // HTML hex character references
		`|%3c\s*script` + // URL-encoded <script
		`|%3cscript` + // URL-encoded <script (no space)
		`|%3c\s*iframe` + // URL-encoded <iframe
		`|onerror\s*%3d` + // URL-encoded onerror=
		`|%3d\s*alert` + // URL-encoded =alert
		`)`,
)

// detectXSS inspects a string for cross-site scripting patterns.
// Returns (attackType, severity, payloadSnippet) or ("","","") if clean.
func detectXSS(input string) (string, string, string) {
	// Decode common URL encodings before matching so bypasses are caught
	// Lower-case first so we only need one pass.
	decoded := strings.ToLower(input)
	decoded = strings.ReplaceAll(decoded, "%3c", "<")
	decoded = strings.ReplaceAll(decoded, "%3e", ">")
	decoded = strings.ReplaceAll(decoded, "%3d", "=")
	decoded = strings.ReplaceAll(decoded, "%22", "\"")
	decoded = strings.ReplaceAll(decoded, "%27", "'")
	decoded = strings.ReplaceAll(decoded, "%28", "(")
	decoded = strings.ReplaceAll(decoded, "%29", ")")
	decoded = strings.ReplaceAll(decoded, "+", " ")
	loc := xssPattern.FindStringIndex(decoded)
	if loc == nil {
		return "", "", ""
	}

	snippet := buildSnippet(input, loc[0], loc[1])
	return "xss", "high", snippet
}

// buildSnippet extracts a context window around the match position.
// Shared by both sqli.go and xss.go.
func buildSnippet(input string, matchStart, matchEnd int) string {
	const contextLen = 20
	const maxTotal = 120

	if len(input) <= maxTotal {
		return input
	}

	start := matchStart - contextLen
	if start < 0 {
		start = 0
	}
	end := matchEnd + contextLen
	if end > len(input) {
		end = len(input)
	}

	snippet := input[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(input) {
		snippet = snippet + "..."
	}
	return snippet
}
