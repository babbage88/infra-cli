package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	downloadFileCmdUrlFlag         string
	downloadFileCmdDestinationPath string
)

// progressWriter wraps io.Writer so we can track download progress
type progressWriter struct {
	io.Writer
	total    int64
	progress int64
	updateCh chan float64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	if err == nil {
		pw.progress += int64(n)
		pw.updateCh <- float64(pw.progress) / float64(pw.total)
	}
	return n, err
}

// Bubble Tea progress model
type model struct {
	progress progress.Model
	percent  float64
	done     bool
}

func newModel() model {
	return model{
		progress: progress.New(progress.WithDefaultGradient()),
	}
}

type progressMsg float64
type doneMsg struct{}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		m.percent = float64(msg)
		if m.percent >= 1.0 {
			return m, func() tea.Msg { return doneMsg{} }
		}
	case doneMsg:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	if m.done {
		return "✅ Download complete!\n"
	}
	return fmt.Sprintf("Downloading...\n%s\n", m.progress.ViewAs(m.percent))
}

// resolveDestination figures out the actual output filepath.
func resolveDestination(dest string, rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Extract filename from URL path
	filename := path.Base(parsedURL.Path)
	if filename == "" || filename == "/" || filename == "." {
		filename = "downloaded.file"
	}

	// If destination is a directory, append filename
	fi, err := os.Stat(dest)
	if err == nil && fi.IsDir() {
		return filepath.Join(dest, filename), nil
	}

	// Otherwise, treat destination as full file path
	return dest, nil
}

// Actual downloader
func downloadFile(filepath string, url string, updates chan float64) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	total := resp.ContentLength
	pw := &progressWriter{
		Writer:   out,
		total:    total,
		updateCh: updates,
	}

	_, err = io.Copy(pw, resp.Body)
	if err != nil {
		return err
	}

	close(updates)
	return nil
}

var downloadFileCmd = &cobra.Command{
	Use:     "get-file",
	Aliases: []string{"dl", "download", "download-file", "grab"},
	Short:   "Download file from a URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		if downloadFileCmdUrlFlag == "" {
			return fmt.Errorf("--url is required")
		}

		dest, err := resolveDestination(downloadFileCmdDestinationPath, downloadFileCmdUrlFlag)
		if err != nil {
			return err
		}

		// Channel to receive progress updates
		updateCh := make(chan float64, 1)

		// Bubble Tea program
		p := tea.NewProgram(newModel())

		// Run downloader in background
		go func() {
			err := downloadFile(dest, downloadFileCmdUrlFlag, updateCh)
			if err != nil {
				slog.Error("❌ Download failed", "error", err.Error())
				os.Exit(1)
			}
		}()

		// Listen for progress updates
		go func() {
			for percent := range updateCh {
				p.Send(progressMsg(percent))
			}
			p.Send(doneMsg{})
		}()

		// Start Bubble Tea UI
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running progress UI: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadFileCmd)

	downloadFileCmd.Flags().StringVarP(&downloadFileCmdUrlFlag, "url", "u", "", "Source URL for the file to be downloaded")
	downloadFileCmd.Flags().StringVar(&downloadFileCmdDestinationPath, "dst", ".", "Download destination (file or directory)")
}
