package clipboard

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// HasFile 检测剪贴板当前是否包含文件，并返回文件路径数组
func HasFile() ([]string, error) {
	switch runtime.GOOS {
	case "windows":
		return hasFileWindows()
	case "linux":
		return hasFileLinux()
	case "darwin":
		return hasFileDarwin()
	default:
		return nil, fmt.Errorf("unsupported platform")
	}
}

// WriteFiles 将本地文件路径数组以原生文件格式写入剪贴板
func WriteFiles(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return writeFilesWindows(paths)
	case "linux":
		return writeFilesLinux(paths)
	case "darwin":
		return writeFilesDarwin(paths)
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func hasFileWindows() ([]string, error) {
	// 使用 PowerShell 获取剪贴板中的文件列表
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "(Get-Clipboard -Format FileDropList).FullName")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func writeFilesWindows(paths []string) error {
	// 拼接字符串: 'C:\path1','C:\path2'
	var quotedPaths []string
	for _, p := range paths {
		quotedPaths = append(quotedPaths, fmt.Sprintf("'%s'", strings.ReplaceAll(p, "'", "''")))
	}
	psCommand := fmt.Sprintf("Set-Clipboard -Path %s", strings.Join(quotedPaths, ","))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCommand)
	return cmd.Run()
}

func hasFileLinux() ([]string, error) {
	// 尝试 wl-paste (Wayland)
	cmd := exec.Command("wl-paste", "-t", "text/uri-list")
	out, err := cmd.Output()
	if err != nil {
		// 回退 xclip (X11)
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o", "-t", "text/uri-list")
		out, err = cmd.Output()
		if err != nil {
			return nil, nil // 没有文件或不支持
		}
	}

	return parseURIList(string(out))
}

func writeFilesLinux(paths []string) error {
	var uris []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err == nil {
			// Convert to file URI
			uris = append(uris, "file://"+url.PathEscape(abs))
		}
	}
	content := strings.Join(uris, "\r\n")

	// 尝试 wl-copy
	cmd := exec.Command("wl-copy", "-t", "text/uri-list")
	cmd.Stdin = bytes.NewBufferString(content)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// 回退 xclip
	cmd = exec.Command("xclip", "-selection", "clipboard", "-t", "text/uri-list", "-i")
	cmd.Stdin = bytes.NewBufferString(content)
	return cmd.Run()
}

func hasFileDarwin() ([]string, error) {
	// macOS 用 AppleScript 检查剪贴板中的文件
	script := `
	try
		set theList to the clipboard as «class furl»
		if class of theList is list then
			set posixList to {}
			repeat with i from 1 to count of theList
				set end of posixList to POSIX path of (item i of theList)
			end repeat
			set text item delimiters to "\n"
			return posixList as text
		else
			return POSIX path of theList
		end if
	on error
		return ""
	end try
	`
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func writeFilesDarwin(paths []string) error {
	// Use AppleScript to set clipboard to file
	var applescriptPaths []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err == nil {
			applescriptPaths = append(applescriptPaths, fmt.Sprintf(`POSIX file "%s"`, abs))
		}
	}

	if len(applescriptPaths) == 0 {
		return nil
	}

	script := fmt.Sprintf(`set the clipboard to {%s}`, strings.Join(applescriptPaths, ", "))
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func parseURIList(data string) ([]string, error) {
	lines := strings.Split(data, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "file://") {
			parsed, err := url.Parse(line)
			if err == nil {
				// Get unescaped path
				result = append(result, parsed.Path)
			}
		}
	}
	return result, nil
}
