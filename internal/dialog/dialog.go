package dialog

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SaveFileDialog 弹出操作系统原生的文件保存对话框
// 返回用户选择的保存路径；用户取消时返回空字符串和 nil error
func SaveFileDialog(defaultName string) (string, error) {
	switch runtime.GOOS {
	case "linux":
		return saveFileLinux(defaultName)
	case "windows":
		return saveFileWindows(defaultName)
	case "darwin":
		return saveFileDarwin(defaultName)
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func saveFileLinux(defaultName string) (string, error) {
	// 优先尝试 zenity
	if path, err := exec.LookPath("zenity"); err == nil && path != "" {
		cmd := exec.Command("zenity", "--file-selection", "--save", "--confirm-overwrite",
			"--title=保存文件", "--filename="+defaultName)
		out, err := cmd.Output()
		if err != nil {
			// 用户按了取消按钮，zenity 返回非零退出码
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	// 回退 kdialog
	if path, err := exec.LookPath("kdialog"); err == nil && path != "" {
		cmd := exec.Command("kdialog", "--getsavefilename", defaultName, "*")
		out, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("no dialog tool found (install zenity or kdialog)")
}

func saveFileWindows(defaultName string) (string, error) {
	// 使用 PowerShell 的 SaveFileDialog
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.SaveFileDialog
$dialog.Title = '保存文件'
$dialog.FileName = '%s'
$result = $dialog.ShowDialog()
if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
    Write-Output $dialog.FileName
}
`, strings.ReplaceAll(defaultName, "'", "''"))

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func saveFileDarwin(defaultName string) (string, error) {
	ext := filepath.Ext(defaultName)
	nameOnly := strings.TrimSuffix(defaultName, ext)

	script := fmt.Sprintf(`
set savePath to choose file name with prompt "保存文件" default name "%s"
return POSIX path of savePath
`, nameOnly+ext)

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		// 用户取消
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
