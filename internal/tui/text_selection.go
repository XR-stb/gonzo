package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// startTextSelection 开始文本选择
func (m *DashboardModel) startTextSelection(x, y int) {
	debugLog(fmt.Sprintf("Starting text selection at (%d, %d)", x, y))
	m.textSelectionActive = true
	m.selectionStartX = x
	m.selectionStartY = y
	m.selectionEndX = x
	m.selectionEndY = y
	m.isDragging = true
	m.selectionText = ""
}

// updateTextSelection 更新文本选择范围
func (m *DashboardModel) updateTextSelection(x, y int) {
	if !m.textSelectionActive {
		return
	}
	debugLog(fmt.Sprintf("Updating text selection to (%d, %d)", x, y))
	m.selectionEndX = x
	m.selectionEndY = y
	m.extractSelectedText()
}

// endTextSelection 结束文本选择
func (m *DashboardModel) endTextSelection() {
	debugLog(fmt.Sprintf("Ending text selection, active: %v, text: '%s'", m.textSelectionActive, m.selectionText))
	m.isDragging = false
	if m.textSelectionActive {
		m.extractSelectedText()
	}
}

// clearTextSelection 清除文本选择
func (m *DashboardModel) clearTextSelection() {
	m.textSelectionActive = false
	m.selectionStartX = 0
	m.selectionStartY = 0
	m.selectionEndX = 0
	m.selectionEndY = 0
	m.selectionText = ""
	m.isDragging = false
}

// extractSelectedText 提取选中的文本
func (m *DashboardModel) extractSelectedText() {
	if !m.textSelectionActive {
		return
	}

	// 确定选择区域的边界
	startX := min(m.selectionStartX, m.selectionEndX)
	endX := max(m.selectionStartX, m.selectionEndX)
	startY := min(m.selectionStartY, m.selectionEndY)
	endY := max(m.selectionStartY, m.selectionEndY)

	var selectedLines []string

	// 根据当前显示的内容提取文本
	if m.showModal && m.currentLogEntry != nil {
		// 从模态框中提取文本
		selectedLines = m.extractTextFromModal(startX, endX, startY, endY)
	} else {
		// 从主界面提取文本
		selectedLines = m.extractTextFromMainView(startX, endX, startY, endY)
	}

	m.selectionText = strings.Join(selectedLines, "\n")
}

// extractTextFromModal 从模态框中提取选中的文本
func (m *DashboardModel) extractTextFromModal(startX, endX, startY, endY int) []string {
	var lines []string

	// 获取当前模态框的内容
	var content string
	if m.modalActiveSection == "info" {
		content = m.infoViewport.View()
	} else if m.modalActiveSection == "chat" {
		content = m.chatViewport.View()
	} else {
		content = m.infoViewport.View()
	}

	contentLines := strings.Split(content, "\n")

	for lineIdx := startY; lineIdx <= endY && lineIdx < len(contentLines); lineIdx++ {
		line := contentLines[lineIdx]

		// 移除 ANSI 转义序列以获取纯文本
		cleanLine := stripAnsiSequences(line)

		if lineIdx == startY && lineIdx == endY {
			// 单行选择
			if startX < len(cleanLine) {
				endPos := min(endX, len(cleanLine))
				if endPos > startX {
					lines = append(lines, cleanLine[startX:endPos])
				}
			}
		} else if lineIdx == startY {
			// 第一行
			if startX < len(cleanLine) {
				lines = append(lines, cleanLine[startX:])
			}
		} else if lineIdx == endY {
			// 最后一行
			endPos := min(endX, len(cleanLine))
			if endPos > 0 {
				lines = append(lines, cleanLine[:endPos])
			}
		} else {
			// 中间行
			lines = append(lines, cleanLine)
		}
	}

	return lines
}

// extractTextFromMainView 从主界面提取选中的文本
func (m *DashboardModel) extractTextFromMainView(startX, endX, startY, endY int) []string {
	var lines []string

	// 这里需要根据当前活动区域提取文本
	switch m.activeSection {
	case SectionLogs:
		// 从日志区域提取文本
		lines = m.extractTextFromLogs(startX, endX, startY, endY)
	default:
		// 从其他区域提取文本（暂时返回空）
		lines = []string{}
	}

	return lines
}

// extractTextFromLogs 从日志区域提取文本
func (m *DashboardModel) extractTextFromLogs(startX, endX, startY, endY int) []string {
	var lines []string

	// 计算日志显示区域的起始位置
	logStartY := m.calculateRequiredChartsHeight()

	// 调整坐标到日志区域内的相对位置
	relativeStartY := startY - logStartY
	relativeEndY := endY - logStartY

	if relativeStartY < 0 {
		relativeStartY = 0
	}

	// 获取当前显示的日志条目
	visibleLogs := m.getVisibleLogEntries()

	for lineIdx := relativeStartY; lineIdx <= relativeEndY && lineIdx < len(visibleLogs); lineIdx++ {
		if lineIdx < 0 || lineIdx >= len(visibleLogs) {
			continue
		}

		logEntry := visibleLogs[lineIdx]
		logText := m.formatLogEntryForCopy(logEntry)

		if lineIdx == relativeStartY && lineIdx == relativeEndY {
			// 单行选择
			if startX < len(logText) {
				endPos := min(endX, len(logText))
				if endPos > startX {
					lines = append(lines, logText[startX:endPos])
				}
			}
		} else if lineIdx == relativeStartY {
			// 第一行
			if startX < len(logText) {
				lines = append(lines, logText[startX:])
			}
		} else if lineIdx == relativeEndY {
			// 最后一行
			endPos := min(endX, len(logText))
			if endPos > 0 {
				lines = append(lines, logText[:endPos])
			}
		} else {
			// 中间行
			lines = append(lines, logText)
		}
	}

	return lines
}

// getVisibleLogEntries 获取当前可见的日志条目
func (m *DashboardModel) getVisibleLogEntries() []LogEntry {
	// 返回当前过滤后的日志条目
	return m.logEntries
}

// formatLogEntryForCopy 格式化日志条目用于复制
func (m *DashboardModel) formatLogEntryForCopy(entry LogEntry) string {
	// 创建完整的日志行文本
	parts := []string{entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Severity}

	if m.showColumns {
		parts = append(parts, getHostFromEntry(entry), getServiceFromEntry(entry))
	}

	parts = append(parts, entry.Message)

	return strings.Join(parts, " | ")
}

// copySelectedText 复制选中的文本到剪贴板
func (m *DashboardModel) copySelectedText() tea.Cmd {
	debugLog(fmt.Sprintf("copySelectedText called: active=%v, text='%s'", m.textSelectionActive, m.selectionText))

	// 确定要复制的文本
	textToCopy := "Test copy functionality"
	if m.selectionText != "" {
		textToCopy = m.selectionText
	}

	// 尝试多种复制方法
	success := false
	var lastErr error

	// 方法1: 使用OSC 52序列（适用于SSH环境）
	if copyWithOSC52(textToCopy) {
		success = true
		debugLog("Successfully copied using OSC 52")
	} else {
		// 方法2: 尝试传统剪贴板
		err := clipboard.WriteAll(textToCopy)
		if err == nil {
			success = true
			debugLog("Successfully copied using traditional clipboard")
		} else {
			lastErr = err
			debugLog(fmt.Sprintf("Traditional clipboard failed: %v", err))
		}
	}

	if success {
		return func() tea.Msg {
			return fmt.Sprintf("已复制: %s", textToCopy)
		}
	} else {
		return func() tea.Msg {
			return fmt.Sprintf("复制失败: %v", lastErr)
		}
	}
}

// copyWithOSC52 使用OSC 52序列复制文本（适用于SSH环境）
func copyWithOSC52(text string) bool {
	// 检查是否在SSH环境中
	if os.Getenv("SSH_CLIENT") == "" && os.Getenv("SSH_TTY") == "" {
		return false // 不在SSH环境中，跳过OSC 52
	}

	// 将文本编码为base64
	encoded := base64.StdEncoding.EncodeToString([]byte(text))

	// 构造OSC 52序列
	osc52 := fmt.Sprintf("\033]52;c;%s\033\\", encoded)

	// 直接写入到stdout
	_, err := os.Stdout.Write([]byte(osc52))
	return err == nil
}

// stripAnsiSequences 移除 ANSI 转义序列
func stripAnsiSequences(text string) string {
	// 简单的 ANSI 序列移除
	result := ""
	inEscape := false

	for i, r := range text {
		if r == '\x1b' && i+1 < len(text) && text[i+1] == '[' {
			inEscape = true
			continue
		}

		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}

		result += string(r)
	}

	return result
}

// renderTextSelection 渲染文本选择高亮
func (m *DashboardModel) renderTextSelection(content string) string {
	if !m.textSelectionActive {
		return content
	}

	// 这里可以添加选择高亮的渲染逻辑
	// 暂时返回原内容
	return content
}

// isPointInSelection 检查点是否在选择区域内
func (m *DashboardModel) isPointInSelection(x, y int) bool {
	if !m.textSelectionActive {
		return false
	}

	startX := min(m.selectionStartX, m.selectionEndX)
	endX := max(m.selectionStartX, m.selectionEndX)
	startY := min(m.selectionStartY, m.selectionEndY)
	endY := max(m.selectionStartY, m.selectionEndY)

	return x >= startX && x <= endX && y >= startY && y <= endY
}

// getHostFromEntry 从日志条目中提取主机信息
func getHostFromEntry(entry LogEntry) string {
	if host, exists := entry.Attributes["host"]; exists {
		return host
	}
	return "unknown"
}

// getServiceFromEntry 从日志条目中提取服务信息
func getServiceFromEntry(entry LogEntry) string {
	if service, exists := entry.Attributes["service.name"]; exists {
		return service
	}
	if service, exists := entry.Attributes["service"]; exists {
		return service
	}
	return "unknown"
}
