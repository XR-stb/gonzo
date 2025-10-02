package tui

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

var (
	debugLogger *log.Logger
	debugOnce   sync.Once
)

// initDebugLogger initializes the debug logger
func initDebugLogger() {
	debugOnce.Do(func() {
		file, err := os.OpenFile("gonzo_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// Fallback to stdout if file creation fails
			debugLogger = log.New(os.Stdout, "DEBUG ", log.LstdFlags)
		} else {
			debugLogger = log.New(file, "DEBUG ", log.LstdFlags)
		}
	})
}

// debugLog writes debug information to the log file
func debugLog(format string, args ...interface{}) {
	initDebugLogger()
	debugLogger.Printf(format, args...)
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatLogEntry formats a log entry with colors
func (m *DashboardModel) formatLogEntry(entry LogEntry, availableWidth int, isSelected bool) string {
	// Use receive time for display
	timestamp := entry.Timestamp.Format("15:04:05")

	// If selected, apply selection style to entire row
	if isSelected {
		// Format the entire row without individual component styling
		severity := fmt.Sprintf("%-5s", entry.Severity)

		var logLine string
		if m.showColumns {
			// Extract host.name and service.name from OTLP attributes
			host := entry.Attributes["host.name"]
			service := entry.Attributes["service.name"]

			// Truncate to fit column width
			if len(host) > 12 {
				host = host[:9] + "..."
			}
			if len(service) > 16 {
				service = service[:13] + "..."
			}

			// Format fixed-width columns
			hostCol := fmt.Sprintf("%-12s", host)
			serviceCol := fmt.Sprintf("%-16s", service)

			// Calculate remaining space for message
			// Use same calculation as non-selected: availableWidth - 18 - columnsWidth
			columnsWidth := 30 // 12 + 16 + 2 spaces
			maxMessageLen := availableWidth - 18 - columnsWidth
			if maxMessageLen < 10 {
				maxMessageLen = 10
			}

			message := entry.Message
			if len(message) > maxMessageLen {
				message = message[:maxMessageLen-3] + "..."
			}

			logLine = fmt.Sprintf("%s %-5s %s %s %s", timestamp, severity, hostCol, serviceCol, message)
		} else {
			// Calculate space for message - use same as non-selected: availableWidth - 18
			maxMessageLen := availableWidth - 18
			if maxMessageLen < 10 {
				maxMessageLen = 10
			}

			message := entry.Message
			if len(message) > maxMessageLen {
				message = message[:maxMessageLen-3] + "..."
			}

			logLine = fmt.Sprintf("%s %-5s %s", timestamp, severity, message)
		}

		// Apply selection style to entire line
		selectedStyle := lipgloss.NewStyle().
			Background(ColorBlue).
			Foreground(ColorWhite)
		return selectedStyle.Render(logLine)
	}

	// Normal (non-selected) formatting with individual component colors
	severityColor := GetSeverityColor(entry.Severity)

	styledSeverity := lipgloss.NewStyle().
		Foreground(severityColor).
		Bold(true).
		Render(fmt.Sprintf("%-5s", entry.Severity))

	styledTimestamp := lipgloss.NewStyle().
		Foreground(ColorGray).
		Render(timestamp)

	// Extract Host and Service columns if enabled
	var hostCol, serviceCol string
	columnsWidth := 0
	if m.showColumns {
		// Extract host.name and service.name from OTLP attributes
		host := entry.Attributes["host.name"]
		service := entry.Attributes["service.name"]

		// Truncate to fit column width (12 chars / 16 chars)
		if len(host) > 12 {
			host = host[:9] + "..."
		}
		if len(service) > 16 {
			service = service[:13] + "..."
		}

		// Style the columns
		hostCol = lipgloss.NewStyle().
			Foreground(ColorGreen).
			Render(fmt.Sprintf("%-12s", host))

		serviceCol = lipgloss.NewStyle().
			Foreground(ColorBlue).
			Render(fmt.Sprintf("%-16s", service))

		columnsWidth = 30 // 12 + 16 + 2 spaces
	}

	// Truncate message if too long
	message := entry.Message

	maxMessageLen := availableWidth - 18 - columnsWidth // Account for timestamp, severity, and columns
	if maxMessageLen < 10 {
		maxMessageLen = 10 // Absolute minimum
	}
	if len(message) > maxMessageLen {
		message = message[:maxMessageLen-3] + "..."
	}

	// Apply search term highlighting to message (word-level highlighting)
	if m.searchTerm != "" {
		message = m.highlightText(message, m.searchTerm)
	}

	// Create the complete log line
	var logLine string
	if m.showColumns {
		logLine = fmt.Sprintf("%s %s %s %s %s", styledTimestamp, styledSeverity, hostCol, serviceCol, message)
	} else {
		logLine = fmt.Sprintf("%s %s %s", styledTimestamp, styledSeverity, message)
	}

	return logLine
}

// highlightText highlights search term within text (for 's' command)
func (m *DashboardModel) highlightText(text, searchTerm string) string {
	if searchTerm == "" {
		return text
	}

	// Case-insensitive search
	lowerText := strings.ToLower(text)
	lowerSearch := strings.ToLower(searchTerm)

	// Find all occurrences
	var result strings.Builder
	lastIndex := 0

	for {
		index := strings.Index(lowerText[lastIndex:], lowerSearch)
		if index == -1 {
			// No more matches, append the rest
			result.WriteString(text[lastIndex:])
			break
		}

		// Calculate actual position in original text
		actualIndex := lastIndex + index

		// Append text before match
		result.WriteString(text[lastIndex:actualIndex])

		// Append highlighted match
		highlightStyle := lipgloss.NewStyle().
			Background(ColorYellow). // Yellow for word highlighting
			Foreground(ColorBlack).
			Bold(true)

		result.WriteString(highlightStyle.Render(text[actualIndex : actualIndex+len(searchTerm)]))

		// Move past this match
		lastIndex = actualIndex + len(searchTerm)
	}

	return result.String()
}

// containsWord checks if a word appears in text using word boundary matching
// This matches how words are extracted for frequency analysis
func (m *DashboardModel) containsWord(text, word string) bool {
	if word == "" {
		return false
	}

	// Convert both to lowercase for case-insensitive matching
	lowerText := strings.ToLower(text)
	lowerWord := strings.ToLower(word)

	// Use regex to match word boundaries - this ensures we match whole words
	// even when they're surrounded by punctuation
	pattern := `\b` + regexp.QuoteMeta(lowerWord) + `\b`
	matched, err := regexp.MatchString(pattern, lowerText)
	if err != nil {
		// Fallback to simple contains if regex fails
		return strings.Contains(lowerText, lowerWord)
	}

	return matched
}

// wrapTextToWidth wraps text to fit within the specified width
func (m *DashboardModel) wrapTextToWidth(text string, width int) string {
	if width <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	var wrappedLines []string

	for _, line := range lines {
		debugLog("Processing line: %q", line)
		debugLog("visibleWidth: %q -> %d", line, lipgloss.Width(line))

		// Use lipgloss.Width to get visual width (ignoring ANSI sequences)
		if lipgloss.Width(line) <= width {
			debugLog("Line fits within width %d, keeping as-is", width)
			wrappedLines = append(wrappedLines, line)
			continue
		}

		debugLog("Line exceeds width %d, need to wrap", width)

		// Check if line contains spaces for word-based wrapping
		words := strings.Fields(line)
		debugLog("Split into %d words: %v", len(words), words)

		// If only one "word" (no spaces), do character-based wrapping
		if len(words) <= 1 {
			debugLog("Line has no spaces, using character-based wrapping")
			// Character-based wrapping for continuous text
			remaining := line
			for len(remaining) > 0 {
				// Find the maximum characters that fit within width
				maxChars := width
				if lipgloss.Width(remaining[:min(len(remaining), maxChars)]) <= width {
					// Try to fit more characters
					for maxChars < len(remaining) && lipgloss.Width(remaining[:maxChars+1]) <= width {
						maxChars++
					}
				} else {
					// Reduce characters until it fits
					for maxChars > 0 && lipgloss.Width(remaining[:maxChars]) > width {
						maxChars--
					}
				}

				if maxChars <= 0 {
					maxChars = 1 // At least one character
				}

				chunk := remaining[:maxChars]
				debugLog("Adding chunk: %q (width: %d)", chunk, lipgloss.Width(chunk))
				wrappedLines = append(wrappedLines, chunk)
				remaining = remaining[maxChars:]
			}
			continue
		}

		// Word-based wrapping for text with spaces
		currentLine := ""
		for i, word := range words {
			debugLog("Processing word %d: %q", i, word)

			// Test if adding this word would exceed width
			testLine := currentLine
			if testLine != "" {
				testLine += " "
			}
			testLine += word

			testLineWidth := lipgloss.Width(testLine)
			debugLog("testLine: %q -> width: %d (limit: %d)", testLine, testLineWidth, width)

			// Use lipgloss.Width for visual width calculation
			if testLineWidth > width {
				debugLog("testLine exceeds width, need to wrap")
				// If current line has content, save it and start new line with current word
				if currentLine != "" {
					debugLog("Saving current line: %q (width: %d)", currentLine, lipgloss.Width(currentLine))
					wrappedLines = append(wrappedLines, currentLine)
					currentLine = word
					debugLog("Starting new line with word: %q", word)
				} else {
					debugLog("Single word is longer than width")
					// Single word is longer than width, need to break it
					currentLine = word
					// For very long words, we might need character-level breaking
					if lipgloss.Width(currentLine) > width {
						debugLog("Word %q is longer than width %d, keeping as-is", word, width)
						// This is tricky with ANSI sequences, so we'll just keep the word as-is
						// and let it overflow rather than risk breaking ANSI sequences
						currentLine = word
					}
				}
			} else {
				debugLog("testLine fits, updating currentLine")
				currentLine = testLine
			}
			debugLog("currentLine after processing word %d: %q (width: %d)", i, currentLine, lipgloss.Width(currentLine))
		}

		// Add remaining content
		if currentLine != "" {
			debugLog("Adding final line: %q (width: %d)", currentLine, lipgloss.Width(currentLine))
			wrappedLines = append(wrappedLines, currentLine)
		}
	}

	return strings.Join(wrappedLines, "\n")
}
