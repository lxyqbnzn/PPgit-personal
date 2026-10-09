package platform

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var dialog sync.Mutex

const windowsFolderPickerScript = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form
$d = New-Object System.Windows.Forms.FolderBrowserDialog
try {
    $owner.ShowInTaskbar = $false
    $owner.FormBorderStyle = [System.Windows.Forms.FormBorderStyle]::None
    $owner.StartPosition = [System.Windows.Forms.FormStartPosition]::Manual
    $area = [System.Windows.Forms.Screen]::FromPoint([System.Windows.Forms.Cursor]::Position).WorkingArea
    $owner.Location = New-Object System.Drawing.Point(($area.Left + [int]($area.Width / 2)), ($area.Top + [int]($area.Height / 2)))
    $owner.Size = New-Object System.Drawing.Size(1, 1)
    $owner.Opacity = 0
    $owner.TopMost = $true
    $owner.Show()
    $owner.Activate()
    $d.Description = 'PPGit: Select project folder'
    $d.ShowNewFolderButton = $false
    if ($d.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) {
        [Console]::Write($d.SelectedPath)
    }
} finally {
    $d.Dispose()
    $owner.Dispose()
}
`

func BrowseFolder() (string, error) {
	return BrowseFolderContext(context.Background())
}

func BrowseFolderContext(parent context.Context) (string, error) {
	if !dialog.TryLock() {
		return "", errors.New("A folder picker is already open.")
	}
	defer dialog.Unlock()
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", windowsFolderPickerScript)
	case "darwin":
		cmd = exec.CommandContext(ctx, "/usr/bin/osascript", "-e", `try
POSIX path of (choose folder with prompt "PPGit: Select project folder")
on error number -128
return ""
end try`)
	default:
		return "", errors.New("Folder picker is supported on Windows and macOS. Enter an absolute path.")
	}
	hideWindow(cmd)
	raw, err := cmd.Output()
	if err != nil {
		return "", errors.New("Unable to open folder picker. Enter an absolute path instead.")
	}
	return strings.TrimSpace(string(raw)), nil
}

func OpenBrowser(url string) error {
	// @Copyright Electric Reverse

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("/usr/bin/open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
