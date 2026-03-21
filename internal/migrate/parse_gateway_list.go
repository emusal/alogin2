package migrate

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParsedGateway holds one row from the legacy gateway_list file.
// The format is:  name  hop1 hop2 ... hopN
// where each hop is a hostname matching an entry in server_list.
type ParsedGateway struct {
	Name string
	Hops []string // ordered hop hostnames
}

// ParseGatewayList reads the gateway_list file.
func ParseGatewayList(path string) ([]*ParsedGateway, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open gateway_list: %w", err)
	}
	defer f.Close()

	var gws []*ParsedGateway
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
		gws = append(gws, &ParsedGateway{
			Name: fields[0],
			Hops: fields[1:],
		})
	}
	return gws, scanner.Err()
}
