package migrate

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParsedAlias holds one row from the legacy alias_hosts file.
// Format:  alias  user@host  or  alias  host
type ParsedAlias struct {
	Alias string
	User  string // may be empty
	Host  string
}

// ParseAliasHosts reads the alias_hosts file.
func ParseAliasHosts(path string) ([]*ParsedAlias, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open alias_hosts: %w", err)
	}
	defer f.Close()

	var aliases []*ParsedAlias
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

		a := &ParsedAlias{Alias: fields[0]}
		target := fields[1]
		if idx := strings.Index(target, "@"); idx >= 0 {
			a.User = target[:idx]
			a.Host = target[idx+1:]
		} else {
			a.Host = target
		}
		aliases = append(aliases, a)
	}
	return aliases, scanner.Err()
}
