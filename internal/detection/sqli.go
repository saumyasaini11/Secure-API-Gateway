package detection

import (
	"regexp"
	"strings"
)

// sqliPattern covers the most common SQL injection techniques.
// It is compiled once at init time for performance.
var sqliPattern = regexp.MustCompile(
	`(?i)(` +
		`\bor\s+\d+\s*=\s*\d+` + // OR 1=1
		`|\bunion\s+(all\s+)?select\b` + // UNION [ALL] SELECT
		`|\bdrop\s+table\b` + // DROP TABLE
		`|\binsert\s+into\b` + // INSERT INTO
		`|\bdelete\s+from\b` + // DELETE FROM
		`|\bupdate\s+\w+\s+set\b` + // UPDATE ... SET
		`|--\s` + // SQL comment with trailing space
		`|;\s*--` + // statement terminator + comment
		`|\bsleep\s*\(` + // sleep()
		`|\bbenchmark\s*\(` + // BENCHMARK()
		`|\bxp_cmdshell\b` + // SQL Server shell escape
		`|\bexec\s*\(` + // exec()
		`|\binformation_schema\b` + // schema enumeration
		`|\bchar\s*\(\s*\d` + // CHAR(n) encoding
		`|\bcast\s*\(` + // CAST()
		`|\bconvert\s*\(` + // CONVERT()
		`|\bwaitfor\s+delay\b` + // WAITFOR DELAY (SQL Server)
		`|\bload_file\s*\(` + // LOAD_FILE()
		`|\binto\s+outfile\b` + // INTO OUTFILE
		`|\bselect\s+.*\bfrom\b` + // SELECT ... FROM
		`)`,
)

// detectSQLi inspects a string for SQL injection patterns.
// Returns (attackType, severity, payloadSnippet) or ("","","") if clean.
func detectSQLi(input string) (string, string, string) {
	// Normalise common URL encodings before matching
	normalised := strings.ReplaceAll(input, "%20", " ")
	normalised = strings.ReplaceAll(normalised, "+", " ")
	normalised = strings.ReplaceAll(normalised, "%27", "'")
	normalised = strings.ReplaceAll(normalised, "%22", "\"")

	loc := sqliPattern.FindStringIndex(normalised)
	if loc == nil {
		return "", "", ""
	}

	snippet := buildSnippet(input, loc[0], loc[1])
	return "sqli", "high", snippet
}
