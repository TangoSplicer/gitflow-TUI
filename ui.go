package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	boxStyle          lipgloss.Style
	detailBoxStyle    lipgloss.Style
	tabActiveStyle    lipgloss.Style
	tabInactiveStyle  lipgloss.Style
	helpStyle         lipgloss.Style
	itemStyle         lipgloss.Style
	selectedItemStyle lipgloss.Style
	dirStyle          lipgloss.Style
	successStyle      lipgloss.Style
	failStyle         lipgloss.Style
	pendingStyle      lipgloss.Style
	dividerStyle      lipgloss.Style
	detailMetaStyle   lipgloss.Style
	repoBadgeStyle    lipgloss.Style
	draftBadgeStyle   lipgloss.Style
)

func initStyles(primaryColor string, borderColor string) {
	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(borderColor)).Padding(0, 1)
	detailBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	tabActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(primaryColor)).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color(primaryColor))
	tabInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 1)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(primaryColor)).Bold(true)
	dirStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	detailMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	repoBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("87")).Bold(true)
	draftBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Background(lipgloss.Color("236")).Padding(0, 1).MarginLeft(1)
}

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

func updateViewport(m model) model {
	if !m.ready {
		return m
	}
	usableSpace := (m.width / 2) - 6
	if !m.isDesktop {
		usableSpace = m.width - 6
	}
	if usableSpace < 10 {
		usableSpace = 10
	}

	list := m.lists[m.activeTab]
	var sb strings.Builder

	if m.activeTab == TabCICD {
		runs := getFilteredRuns(list.Runs, m.filterQuery)
		if len(runs) > 0 {
			run := runs[list.Cursor]
			sb.WriteString(fmt.Sprintf("# %s\n\n", run.Name))
			sb.WriteString(fmt.Sprintf("**Repository:** `%s`\n\n", getRepoName(run.Url)))
			sb.WriteString(fmt.Sprintf("- **Status:** %s\n", run.Status))
			sb.WriteString(fmt.Sprintf("- **Conclusion:** %s\n", run.Conclusion))
			sb.WriteString(fmt.Sprintf("- **Ran:** %s\n", timeAgo(run.CreatedAt)))
		}
	} else if m.activeTab == TabFiles {
		files := getFilteredFiles(list.Files, m.filterQuery)
		if len(files) > 0 {
			file := files[list.Cursor]
			absPath := filepath.Join(list.CurrentDir, file.Name)
			fType := "File"
			if file.IsDir {
				fType = "Directory"
			}
			sb.WriteString(fmt.Sprintf("# %s\n\n", file.Name))
			sb.WriteString(fmt.Sprintf("**Path:** `%s`\n\n", absPath))
			sb.WriteString(fmt.Sprintf("- **Type:** %s\n", fType))
			sb.WriteString(fmt.Sprintf("- **Size:** %s\n", formatSize(file.Size)))
			sb.WriteString(fmt.Sprintf("- **Modified:** %s\n", timeAgo(file.ModTime)))
			sb.WriteString(fmt.Sprintf("- **Permissions:** `%s`\n", file.Mode.String()))
		}
	} else if m.activeTab == TabInbox {
		notifs := getFilteredNotifs(list.Notifications, m.filterQuery)
		if len(notifs) > 0 {
			notif := notifs[list.Cursor]
			sb.WriteString(fmt.Sprintf("# %s\n\n", notif.Subject.Title))
			sb.WriteString(fmt.Sprintf("**Repository:** `%s`\n\n", notif.Repository.FullName))
			sb.WriteString(fmt.Sprintf("- **Reason:** %s\n", notif.Reason))
			sb.WriteString(fmt.Sprintf("- **Type:** %s\n", notif.Subject.Type))
			sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", timeAgo(notif.UpdatedAt)))
		}
	} else {
		items := getFilteredItems(list.Items, m.filterQuery)
		if len(items) > 0 {
			item := items[list.Cursor]
			sb.WriteString(fmt.Sprintf("# %s\n\n", item.Title))
			draftStatus := ""
			if item.Draft {
				draftStatus = " **[DRAFT]** "
			}
			sb.WriteString(fmt.Sprintf("**Author:** @%s | **Opened:** %s %s\n\n", item.User.Login, timeAgo(item.CreatedAt), draftStatus))
			body := item.Body
			if body == "" {
				body = "_No description provided._"
			}
			sb.WriteString("---\n\n" + body)
		}
	}

	r, _ := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(usableSpace))
	rendered, err := r.Render(sb.String())
	if err != nil {
		rendered = sb.String()
	}

	rendered = strings.TrimSpace(rendered)
	m.vp.SetContent(rendered)
	return m
}

func (m model) renderCommenting() string {
	title := selectedItemStyle.Render(" 📝 Write a Comment ")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2).Render(title + "\n\n" + m.ta.View() + "\n\n" + helpStyle.Render(" ctrl+s: submit • esc: cancel "))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderCreating() string {
	titleStr := " Create Pull Request "
	if m.createTarget == "issue" {
		titleStr = " Create Issue "
	}
	title := selectedItemStyle.Render(fmt.Sprintf(" ✨ %s ", titleStr))

	safeWidth := m.width - 6
	if safeWidth > 60 {
		safeWidth = 60
	}
	box := lipgloss.NewStyle().Width(safeWidth).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2).Render(title + "\n\n" + m.form.View() + "\n\n" + helpStyle.Render(" esc: cancel • enter: next "))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// NEW: Settings Form Renderer
func (m model) renderSettings() string {
	title := selectedItemStyle.Render(" ⚙  Preferences ")

	safeWidth := m.width - 6
	if safeWidth > 60 {
		safeWidth = 60
	}

	box := lipgloss.NewStyle().
		Width(safeWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Render(title + "\n\n" + m.form.View() + "\n\n" + helpStyle.Render(" esc: cancel • enter: next • save to apply "))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderHelp() string {
	title := selectedItemStyle.Render(" ⌨  GitFlow Command Palette ")

	leftCol := `
  GLOBAL
  tab / l   : Next tab
  s+tab / h : Prev tab
  /         : Filter
  y         : Copy
  + / C     : Create PR/Issue
  ,         : Settings
  ?         : Help
  q         : Quit
  
  SCROLLING
  up / k    : Move up
  down / j  : Move down
  pgup/pgdn : Scroll pane
  R         : Refresh`

	rightCol := `
  PRs & ISSUES
  r         : Reply (Comment)
  t / T     : Ghost Handoff
  v         : View diff
  m         : Merge
  c         : Close
  enter / o : Open Browser

  PIPELINES & INBOX
  v         : View logs
  w / x     : Rerun / Cancel
  e         : Mark read`

	var content string
	if m.isDesktop {
		content = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(35).Foreground(lipgloss.Color("250")).Render(leftCol), lipgloss.NewStyle().Width(35).Foreground(lipgloss.Color("250")).Render(rightCol))
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render(leftCol), "\n", lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render(rightCol))
	}

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2).Render(title + "\n" + content + "\n\n" + helpStyle.Render(" press esc, q, or ? to resume "))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderDashboard() string {
	var sb strings.Builder
	tabNames := []string{" PRs", " Issues", " CI/CD", " Files", " Inbox"}
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
	if m.activeTab == TabPRs || m.activeTab == TabIssues {
		helpText = "r:reply • t:test • v:diff • m:merge"
	} else if m.activeTab == TabCICD {
		helpText = "v:logs • x:cancel • w:rerun"
	} else if m.activeTab == TabFiles {
		helpText = "enter:open"
	} else if m.activeTab == TabInbox {
		helpText = "e:mark read • o:open"
	}

	helpText = fmt.Sprintf("/:filter • y:copy • ,:cfg • ?:help • %s", helpText)

	if m.state == StateFiltering {
		helpText = fmt.Sprintf("🔍 Filter: %s█", m.filterQuery)
	} else if m.filterQuery != "" {
		helpText = fmt.Sprintf("🔍 Filter: %s • %s", m.filterQuery, helpText)
	}

	safeWidth := m.width - 4
	if safeWidth < 10 {
		safeWidth = 10
	}
	helpText = truncate(helpText, safeWidth)

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

	visH := m.listHeight
	if !m.isDesktop {
		visH = m.listHeight / 2
	}
	if visH < 2 {
		visH = 2
	}

	if list.Error != "" {
		sb.WriteString(fmt.Sprintf(" %s\n", list.Error))
		for i := 1; i < visH; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	} else if list.IsLoading {
		sb.WriteString(" Working... Please wait.\n")
		for i := 1; i < visH; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	var limit int
	if m.activeTab == TabCICD {
		limit = len(getFilteredRuns(list.Runs, m.filterQuery))
	}
	if m.activeTab == TabFiles {
		limit = len(getFilteredFiles(list.Files, m.filterQuery))
	}
	if m.activeTab == TabInbox {
		limit = len(getFilteredNotifs(list.Notifications, m.filterQuery))
	}
	if m.activeTab == TabPRs || m.activeTab == TabIssues {
		limit = len(getFilteredItems(list.Items, m.filterQuery))
	}

	if limit == 0 {
		sb.WriteString(" No items found.\n")
		for i := 1; i < visH; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	end := list.ViewportStart + visH
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

		} else if m.activeTab == TabInbox {
			notifs := getFilteredNotifs(list.Notifications, m.filterQuery)
			notif := notifs[i]
			cleanTitle := truncate(notif.Subject.Title, usableSpace)
			icon := " "
			if !notif.Unread {
				icon = " "
			}
			rowStr = fmt.Sprintf("%s %s", pendingStyle.Render(icon), cleanTitle)

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
	for i := limit; i < visH; i++ {
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m model) renderDetailOnly(availableWidth int) string {
	if !m.ready {
		return "\n Loading...\n"
	}
	return m.vp.View()
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
	helpMenu := helpStyle.Render("e/enter: edit local file • esc/q: back • ?: help")

	return lipgloss.NewStyle().Margin(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, box, "\n"+helpMenu))
}
