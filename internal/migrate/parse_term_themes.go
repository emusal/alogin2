package migrate

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParsedTheme holds one row from the legacy term_themes file.
// Format:  locale  theme_name
type ParsedTheme struct {
	Locale string
	Theme  string
}

// ParseTermThemes reads the term_themes file.
func ParseTermThemes(path string) ([]*ParsedTheme, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open term_themes: %w", err)
	}
	defer f.Close()

	var themes []*ParsedTheme
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		themes = append(themes, &ParsedTheme{
			Locale: fields[0],
			Theme:  fields[1],
		})
	}
	return themes, scanner.Err()
}
