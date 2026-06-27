package helper

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mozillazg/go-unidecode"
)

// GetConfigDir resolves the Qalibre config/settings directory.
// Priority:
// 1. Env var CALIBRE_DBPATH
// 2. If a file `.HOMEDIR` exists in the executable's directory, use ~/.qalibre
// 3. Otherwise, use the executable's directory (the repository root or current dir)
func GetConfigDir() string {
	if env := os.Getenv("CALIBRE_DBPATH"); env != "" {
		return env
	}

	execPath, err := os.Executable()
	var execDir string
	if err == nil {
		execDir = filepath.Dir(execPath)
	} else {
		execDir = "."
	}

	// Check if .HOMEDIR marker is present in execDir
	if _, err := os.Stat(filepath.Join(execDir, ".HOMEDIR")); err == nil {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".qalibre")
		}
	}

	// Fallback to execDir
	return execDir
}

// GetValidFilename sanitizes a string to be used as a valid filename.
// Mirrors get_valid_filename in cps/helper.py.
func GetValidFilename(val string, replaceWhitespace bool, chars int, forceUnidecode bool, configUnicode bool) (string, error) {
	if len(val) == 0 {
		return "", errors.New("filename cannot be empty")
	}

	if val[len(val)-1] == '.' {
		val = val[:len(val)-1] + "_"
	}

	val = strings.ReplaceAll(val, "/", "_")
	val = strings.ReplaceAll(val, ":", "_")
	val = strings.ReplaceAll(val, "\x00", "")

	if configUnicode || forceUnidecode {
		val = unidecode.Unidecode(val)
	}

	if replaceWhitespace {
		// Replace * + : \ " / < > ? with _
		re := regexp.MustCompile(`[*+:\\"/<>?]+`)
		val = re.ReplaceAllString(val, "_")
		// Replace | with ,
		rePipe := regexp.MustCompile(`[|]+`)
		val = rePipe.ReplaceAllString(val, ",")
	}

	// Limit length
	valBytes := []byte(val)
	if len(valBytes) > chars {
		valBytes = valBytes[:chars]
	}
	val = strings.TrimSpace(string(valBytes))

	if len(val) == 0 {
		return "", errors.New("filename cannot be empty")
	}
	return val, nil
}

// SplitAuthors splits author strings by & or ; and trims whitespace.
func SplitAuthors(values []string) []string {
	var list []string
	re := regexp.MustCompile(`[&;]`)
	for _, val := range values {
		authors := re.Split(val, -1)
		for _, author := range authors {
			author = strings.TrimSpace(author)
			if author == "" {
				continue
			}
			commas := strings.Count(author, ",")
			if commas == 1 {
				parts := strings.Split(author, ",")
				list = append(list, strings.TrimSpace(parts[1])+" "+strings.TrimSpace(parts[0]))
			} else if commas > 1 {
				parts := strings.Split(author, ",")
				for _, p := range parts {
					if trimmed := strings.TrimSpace(p); trimmed != "" {
						list = append(list, trimmed)
					}
				}
			} else {
				list = append(list, author)
			}
		}
	}
	return list
}

// GetSortedAuthor sorts a single author name (e.g. "First Last" -> "Last, First").
func GetSortedAuthor(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	if strings.Contains(val, ",") {
		return val
	}

	parts := strings.Fields(val)
	if len(parts) == 0 {
		return val
	}

	suffixRe := regexp.MustCompile(`^(?i)(JR|SR|I{1,3}|IV)\.?$`)
	lastIdx := len(parts) - 1

	if suffixRe.MatchString(parts[lastIdx]) {
		if len(parts) > 1 {
			// Suffix exists, e.g. "John Smith Jr." -> last is Jr., last-1 is Smith
			lastName := parts[lastIdx-1]
			firstNames := strings.Join(parts[:lastIdx-1], " ")
			return fmt.Sprintf("%s, %s %s", lastName, firstNames, parts[lastIdx])
		}
		return val
	}

	if len(parts) == 1 {
		return val
	}

	lastName := parts[lastIdx]
	firstNames := strings.Join(parts[:lastIdx], " ")
	return fmt.Sprintf("%s, %s", lastName, firstNames)
}

// CheckEmail validates an email string
func CheckEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// CheckUsername validates a username string (alphanumeric/spaces/underscores/hyphens, 2-64 chars)
func CheckUsername(name string) bool {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 64 {
		return false
	}
	// simple validation
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ' ') {
			return false
		}
	}
	return true
}

// CheckValidDomain checks if the email's domain is in the allowed/denied registrations list
func CheckValidDomain(email string, allowedDomains []string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])

	// Check if allowed domains has any matching wildcard
	for _, pattern := range allowedDomains {
		pattern = strings.ToLower(pattern)
		if pattern == "%" || pattern == "%.%" {
			return true
		}
		// simple glob matching
		if strings.HasPrefix(pattern, "%.") {
			suffix := pattern[2:]
			if strings.HasSuffix(domain, suffix) {
				return true
			}
		}
		if pattern == domain {
			return true
		}
	}
	return false
}
