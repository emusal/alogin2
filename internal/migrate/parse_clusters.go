package migrate

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParsedCluster holds one row from the legacy clusters file.
// Format:  cluster_name  host1 host2 ... hostN
type ParsedCluster struct {
	Name  string
	Hosts []string
}

// ParseClusters reads the clusters file.
func ParseClusters(path string) ([]*ParsedCluster, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open clusters: %w", err)
	}
	defer f.Close()

	var clusters []*ParsedCluster
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
		clusters = append(clusters, &ParsedCluster{
			Name:  fields[0],
			Hosts: fields[1:],
		})
	}
	return clusters, scanner.Err()
}
