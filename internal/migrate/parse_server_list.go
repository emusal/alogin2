package migrate

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/emusal/alogin2/internal/model"
)

// ParsedServer holds a row parsed from the legacy server_list TSV.
type ParsedServer struct {
	Protocol model.Protocol
	Host     string
	User     string
	Password string // decoded (space/tab literals expanded)
	Port     int    // 0 = use default
	Gateway  string // gateway name or empty
	Locale   string
}

// ParseServerList reads the server_list TSV file and returns all entries.
// Comment lines (starting with #) are skipped.
// The legacy <space> and <tab> tokens in the password field are decoded.
func ParseServerList(path string) ([]*ParsedServer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open server_list: %w", err)
	}
	defer f.Close()

	var servers []*ParsedServer
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		// Skip separator rows (all dashes, e.g. "- - -")
		allDash := true
		for _, f := range fields {
			if f != "-" {
				allDash = false
				break
			}
		}
		if allDash || len(fields) < 4 {
			continue
		}

		srv := &ParsedServer{
			Protocol: model.Protocol(fields[0]),
			Host:     fields[1],
			User:     fields[2],
			Password: decodePassword(fields[3]),
		}

		if len(fields) >= 5 && fields[4] != "-" {
			port, err := strconv.Atoi(fields[4])
			if err != nil {
				return nil, fmt.Errorf("server_list line %d: invalid port %q: %w", lineNum, fields[4], err)
			}
			srv.Port = port
		}
		if len(fields) >= 6 && fields[5] != "-" {
			srv.Gateway = fields[5]
		}
		if len(fields) >= 7 && fields[6] != "-" {
			srv.Locale = fields[6]
		}

		servers = append(servers, srv)
	}

	return servers, scanner.Err()
}

// decodePassword converts the legacy <space> and <tab> tokens to real characters.
func decodePassword(raw string) string {
	raw = strings.ReplaceAll(raw, "<space>", " ")
	raw = strings.ReplaceAll(raw, "<tab>", "\t")
	return raw
}
