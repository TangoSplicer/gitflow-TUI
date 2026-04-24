package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	boxStyle          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
	detailBoxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	tabActiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color("212"))
	tabInactiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 1)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	itemStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

	dirStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	failStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	pendingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	dividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	detailMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	repoBadgeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("87")).Bold(true)
	draftBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Background(lipgloss.Color("236")).Padding(0, 1).MarginLeft(1)
)

func getRepoName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		return parts[3] + "/" + parts[4]
	}
	return ""
}
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		if max > 3 {
			return string(runes[:max-3]) + "..."
		}
		return string(runes[:max])
	}
	return s
}
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m model) renderDashboard() string {
	var sb strings.Builder
	tabNames := []string{" PRs", " Issues", " CI/CD", " Files"}
	var tabs []string
	for i, name := range tabNames {
		if i == m.activeTab {
			tabs = append(tabs, tabActiveStyle.Render(name))
		} else {
			tabs = append(tabs, tabInactiveStyle.Render(name))
		}
	}

	pollIndicator := ""
	if m.isPolling {
		pollIndicator = helpStyle.Render(" (auto-refreshing...)")
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...) + pollIndicator + "\n\n")

	if m.isDesktop {
		sb.WriteString(m.renderDesktopBody())
	} else {
		sb.WriteString(m.renderMobileBody())
	}

	var helpText string
	if m.activeTab == TabPRs {
		helpText = "v: diff • m: merge • c: close"
	} else if m.activeTab == TabCICD {
		helpText = "v: logs • x: cancel • w: rerun"
	} else if m.activeTab == TabFiles {
		helpText = "enter: open/edit"
	} else {
		helpText = "o: open in browser"
	}

	helpText = fmt.Sprintf("/: filter • %s • q: quit", helpText)

	if m.state == StateFiltering {
		helpText = selectedItemStyle.Render(fmt.Sprintf("🔍 Filter: %s█", m.filterQuery))
	} else if m.filterQuery != "" {
		helpText = fmt.Sprintf("🔍 Filter: %s (esc to clear) • ", m.filterQuery) + helpText
	}

	sb.WriteString("\n" + helpStyle.Render(helpText))
	return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
}

func (m model) renderDesktopBody() string {
	listWidth := (m.width / 2) - 4
	detailWidth := (m.width / 2) - 4
	leftBox := boxStyle.Width(listWidth).Height(m.listHeight).Render(m.renderListOnly(listWidth))
	rightBox := detailBoxStyle.Width(detailWidth).Height(m.listHeight).Render(m.renderDetailOnly(detailWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (m model) renderMobileBody() string {
	boxWidth := m.width - 4
	listContent := m.renderListOnly(boxWidth)
	detailContent := "\n" + dividerStyle.Render(strings.Repeat("─", boxWidth-4)) + "\n" + m.renderDetailOnly(boxWidth)
	return boxStyle.Width(boxWidth).Render(listContent + detailContent)
}

func (m model) renderListOnly(availableWidth int) string {
	var sb strings.Builder
	list := m.lists[m.activeTab]
	usableSpace := availableWidth - 16
	if usableSpace < 10 {
		usableSpace = 10
	}

	if list.Error != "" {
		sb.WriteString(fmt.Sprintf(" %s\n", list.Error))
		for i := 1; i < m.listHeight; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	} else if list.IsLoading {
		sb.WriteString(" Working... Please wait.\n")
		for i := 1; i < m.listHeight; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	// Apply filtering
	var limit int
	if m.activeTab == TabCICD {
		limit = len(getFilteredRuns(list.Runs, m.filterQuery))
	}
	if m.activeTab == TabFiles {
		limit = len(getFilteredFiles(list.Files, m.filterQuery))
	}
	if m.activeTab == TabPRs || m.activeTab == TabIssues {
		limit = len(getFilteredItems(list.Items, m.filterQuery))
	}

	if limit == 0 {
		sb.WriteString(" No items match filter.\n")
		for i := 1; i < m.listHeight; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	end := list.ViewportStart + m.listHeight
	if end > limit {
		end = limit
	}

	for i := list.ViewportStart; i < end; i++ {
		var rowStr string
		if m.activeTab == TabCICD {
			runs := getFilteredRuns(list.Runs, m.filterQuery)
			run := runs[i]
			cleanTitle := truncate(run.Name, usableSpace)
			icon, statusColor := "", itemStyle
			if run.Status == "completed" {
				if run.Conclusion == "success" {
					icon, statusColor = successStyle.Render("✓"), successStyle
				} else {
					icon, statusColor = failStyle.Render("✗"), failStyle
				}
			} else {
				icon, statusColor = pendingStyle.Render("⟳"), pendingStyle
			}
			rowStr = fmt.Sprintf("%s %s", icon, statusColor.Bold(true).Render(cleanTitle))

		} else if m.activeTab == TabFiles {
			files := getFilteredFiles(list.Files, m.filterQuery)
			file := files[i]
			cleanTitle := truncate(file.Name, usableSpace)
			if file.IsDir {
				rowStr = fmt.Sprintf(" %s", dirStyle.Render(cleanTitle+"/"))
			} else {
				rowStr = fmt.Sprintf(" %s", itemStyle.Render(cleanTitle))
			}

		} else {
			items := getFilteredItems(list.Items, m.filterQuery)
			item := items[i]
			cleanTitle := truncate(strings.ReplaceAll(strings.ReplaceAll(item.Title, "\n", " "), "\r", ""), usableSpace)
			rowStr = fmt.Sprintf("#%-4d %s", item.Number, cleanTitle)
		}

		if i == list.Cursor {
			sb.WriteString(selectedItemStyle.Render(" ▶ ") + rowStr + "\n")
		} else {
			sb.WriteString("   " + rowStr + "\n")
		}
	}
	for i := limit; i < m.listHeight; i++ {
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m model) renderDetailOnly(availableWidth int) string {
	var sb strings.Builder
	list := m.lists[m.activeTab]
	usableSpace := availableWidth - 6

	if m.activeTab == TabCICD {
		runs := getFilteredRuns(list.Runs, m.filterQuery)
		if len(runs) > 0 {
			run := runs[list.Cursor]
			sb.WriteString(repoBadgeStyle.Render(getRepoName(run.Url)) + "\n")
			sb.WriteString(selectedItemStyle.Render(truncate(run.Name, usableSpace)) + "\n\n")
			meta := fmt.Sprintf("ID: %d\nStatus: %s\nRan: %s", run.DatabaseId, run.Status, timeAgo(run.CreatedAt))
			sb.WriteString(detailMetaStyle.Render(meta))
		}
	} else if m.activeTab == TabFiles {
		files := getFilteredFiles(list.Files, m.filterQuery)
		if len(files) > 0 {
			file := files[list.Cursor]
			absPath := filepath.Join(list.CurrentDir, file.Name)
			sb.WriteString(repoBadgeStyle.Render(truncate(absPath, usableSpace)) + "\n\n")

			fType := "File"
			if file.IsDir {
				fType = "Directory"
			}
			meta := fmt.Sprintf("Type: %s\nSize: %s\nModified: %s\nPerms: %s", fType, formatSize(file.Size), timeAgo(file.ModTime), file.Mode.String())
			sb.WriteString(detailMetaStyle.Render(meta))
		}
	} else {
		items := getFilteredItems(list.Items, m.filterQuery)
		if len(items) > 0 {
			item := items[list.Cursor]
			sb.WriteString(repoBadgeStyle.Render(getRepoName(item.HtmlUrl)) + "\n")
			sb.WriteString(selectedItemStyle.Render(truncate(strings.ReplaceAll(item.Title, "\n", " "), usableSpace)) + "\n\n")

			meta := fmt.Sprintf("Author: @%s\nOpened: %s", item.User.Login, timeAgo(item.CreatedAt))
			if item.Draft {
				sb.WriteString(detailMetaStyle.Render(meta) + draftBadgeStyle.Render("DRAFT"))
			} else {
				sb.WriteString(detailMetaStyle.Render(meta))
			}
		}
	}
	return sb.String()
}

func (m model) renderFileViewer() string {
	var sb strings.Builder
	boxWidth := m.width - 4
	listHeight := m.height - 8
	if listHeight < 5 {
		listHeight = 5
	}

	sb.WriteString(tabActiveStyle.Render(" PR Files Changed"))
	sb.WriteString(fmt.Sprintf(detailMetaStyle.Render(" (%d files)"), len(m.files)))
	sb.WriteString("\n\n")

	usableSpace := boxWidth - 25
	end := m.fileViewportStart + listHeight
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := m.fileViewportStart; i < end; i++ {
		file := m.files[i]
		cleanPath := truncate(file.Path, usableSpace)
		addStr := successStyle.Render(fmt.Sprintf("+%-3d", file.Additions))
		subStr := failStyle.Render(fmt.Sprintf("-%-3d", file.Deletions))
		rowStr := fmt.Sprintf("%s %s  %s", addStr, subStr, cleanPath)

		if i == m.fileCursor {
			sb.WriteString(selectedItemStyle.Render(" ▶ ") + rowStr + "\n")
		} else {
			sb.WriteString(itemStyle.Render("   "+rowStr) + "\n")
		}
	}
	for i := len(m.files); i < listHeight; i++ {
		sb.WriteString("\n")
	}
	box := boxStyle.Width(boxWidth).Render(sb.String())
	helpMenu := helpStyle.Render("e/enter: edit local file • esc/q: back • ↑/↓: move")

	return lipgloss.NewStyle().Margin(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, box, "\n"+helpMenu))
}
