package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const githubReleaseAPI = "https://api.github.com/repos/emusal/alogin2/releases/latest"
const githubDownloadURL = "https://github.com/emusal/alogin2/releases/download/%s/alogin-web-%s-%s"

func newUpgradeCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade alogin to the latest release",
		Long: `Check GitHub for the latest alogin release and upgrade if a newer version is available.

The current binary is replaced in-place. Data, config, and vault are not affected.

If alogin was installed via Homebrew, use 'brew upgrade alogin' instead.`,
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(yes)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func runUpgrade(yes bool) error {
	// Detect brew-managed binary
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	if strings.Contains(binPath, "Cellar") || strings.Contains(binPath, "homebrew") {
		fmt.Println("alogin is managed by Homebrew. Use 'brew upgrade alogin' instead.")
		return nil
	}

	fmt.Printf("Current version : %s\n", Version)
	fmt.Print("Checking latest release... ")

	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}

	// Strip leading 'v' for comparison
	latestVer := strings.TrimPrefix(latest, "v")
	fmt.Printf("%s\n\n", latestVer)

	if latestVer == Version {
		fmt.Println("Already up to date.")
		return nil
	}

	fmt.Printf("New version available: %s → %s\n", Version, latestVer)

	if !yes {
		fmt.Print("Upgrade? [y/N] ")
		var ans string
		fmt.Scanln(&ans)
		if strings.ToLower(strings.TrimSpace(ans)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	// Map arm64 → arm64, amd64 → amd64 (already correct for our release naming)
	downloadURL := fmt.Sprintf(githubDownloadURL, latest, goos, goarch)

	fmt.Printf("Downloading %s...\n", downloadURL)
	if err := downloadAndReplace(downloadURL, binPath); err != nil {
		return err
	}

	fmt.Printf("alogin upgraded to %s.\n", latestVer)

	// Apply any pending DB schema migrations with the new binary and report results.
	fmt.Println("Applying database schema migrations...")
	migrateCmd := exec.Command(binPath, "db-migrate")
	migrateCmd.Stdout = os.Stdout
	migrateCmd.Stderr = os.Stderr
	if err := migrateCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: db-migrate failed (%v). Run 'alogin db-migrate' manually.\n", err)
	}

	return nil
}

func fetchLatestVersion() (string, error) {
	req, err := http.NewRequest("GET", githubReleaseAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("no tag_name in GitHub response")
	}
	return rel.TagName, nil
}

func downloadAndReplace(url, binPath string) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}

	// Write to a temp file alongside the binary so rename is atomic
	tmpPath := binPath + ".new"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write binary: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, binPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	return nil
}
