package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type AppState int

const (
	StateLoadingToken AppState = iota
	StateDashboard
	StateLoadingFiles
	StateFileViewer
	StateFiltering
	StateHelp
	StateCommenting
	StateCreating
	StateSettings
)
const (
	TabPRs = iota
	TabIssues
	TabCICD
	TabFiles
	TabInbox
	TabCount
)

type SearchResponse struct {
	TotalCount int          `json:"total_count"`
	Items      []GitHubItem `json:"items"`
}
type GitHubItem struct {
	Title     string    `json:"title"`
	Number    int       `json:"number"`
	State     string    `json:"state"`
	HtmlUrl   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	Draft     bool      `json:"draft"`
	Body      string    `json:"body"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}
type RunItem struct {
	DatabaseId int       `json:"databaseId"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"createdAt"`
	Url        string    `json:"url"`
}
type ListData struct {
	Items         []GitHubItem
	Runs          []RunItem
	Files         []LocalFile
	Notifications []NotificationItem
	TotalCount    int
	Page          int
	IsLoading     bool
	HasLoaded     bool
	Error         string
	Cursor        int
	ViewportStart int
	CurrentDir    string
}

type AppConfig struct {
	DefaultTab      int    `json:"default_tab"`
	RefreshInterval int    `json:"refresh_interval"`
	PrimaryColor    string `json:"primary_color"`
	BorderColor     string `json:"border_color"`
}

type model struct {
	ctx               context.Context
	cancel            context.CancelFunc
	state             AppState
	githubToken       string
	err               error
	activeTab         int
	lists             [TabCount]ListData
	width             int
	height            int
	listHeight        int
	isDesktop         bool
	isPolling         bool
	files             []PRFile
	fileCursor        int
	fileViewportStart int
	filterQuery       string
	config            AppConfig

	vp            viewport.Model
	ta            textarea.Model
	commentTarget string

	form         *huh.Form
	createTarget string
	tempConfig   AppConfig

	ready         bool
	previousState AppState
}

func loadConfig() AppConfig {
	cfg := AppConfig{DefaultTab: 0, RefreshInterval: 10, PrimaryColor: "212", BorderColor: "63"}
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return cfg
	}

	configDir := filepath.Join(home, ".config", "gitflow")
	configPath := filepath.Join(configDir, "config.json")

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &cfg)
	} else {
		if err := os.MkdirAll(configDir, 0755); err == nil {
			data, _ := json.MarshalIndent(cfg, "", "  ")
			os.WriteFile(configPath, data, 0644)
		}
	}
	return cfg
}

func saveConfig(cfg AppConfig) {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return
	}

	configDir := filepath.Join(home, ".config", "gitflow")
	configPath := filepath.Join(configDir, "config.json")

	os.MkdirAll(configDir, 0755)
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		os.WriteFile(configPath, data, 0644)
	}
}

func getFilteredItems(items []GitHubItem, query string) []GitHubItem {
	if query == "" {
		return items
	}
	var res []GitHubItem
	q := strings.ToLower(query)
	for _, i := range items {
		if strings.Contains(strings.ToLower(i.Title), q) || strings.Contains(fmt.Sprintf("%d", i.Number), q) {
			res = append(res, i)
		}
	}
	return res
}
func getFilteredRuns(runs []RunItem, query string) []RunItem {
	if query == "" {
		return runs
	}
	var res []RunItem
	q := strings.ToLower(query)
	for _, r := range runs {
		if strings.Contains(strings.ToLower(r.Name), q) {
			res = append(res, r)
		}
	}
	return res
}
func getFilteredFiles(files []LocalFile, query string) []LocalFile {
	if query == "" {
		return files
	}
	var res []LocalFile
	q := strings.ToLower(query)
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.Name), q) || f.Name == ".." {
			res = append(res, f)
		}
	}
	return res
}
func getFilteredNotifs(notifs []NotificationItem, query string) []NotificationItem {
	if query == "" {
		return notifs
	}
	var res []NotificationItem
	q := strings.ToLower(query)
	for _, n := range notifs {
		if strings.Contains(strings.ToLower(n.Subject.Title), q) || strings.Contains(strings.ToLower(n.Repository.FullName), q) {
			res = append(res, n)
		}
	}
	return res
}

func doTick(interval int) tea.Cmd {
	return tea.Tick(time.Second*time.Duration(interval), func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd {
	m.lists[TabFiles].CurrentDir = "."
	return tea.Batch(fetchGitHubToken(m.ctx), doTick(m.config.RefreshInterval))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if (m.state == StateCreating || m.state == StateSettings) && m.form != nil {
		form, cmd := m.form.Update(msg)
		cmds = append(cmds, cmd)

		if f, ok := form.(*huh.Form); ok {
			m.form = f

			if m.form.State == huh.StateCompleted {
				if m.state == StateCreating {
					title := m.form.GetString("title")
					body := m.form.GetString("body")
					c := exec.Command("gh", m.createTarget, "create", "--title", title, "--body", body)
					m.state = StateDashboard
					m.lists[m.activeTab].IsLoading = true
					cmds = append(cmds, tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} }))
					return m, tea.Batch(cmds...)
				} else if m.state == StateSettings {
					// FIX: Explicitly extract from the form using keys to bypass ghost pointers!
					if tabVal, ok := m.form.Get("default_tab").(int); ok {
						m.config.DefaultTab = tabVal
					}
					m.config.PrimaryColor = m.form.GetString("primary_color")
					m.config.BorderColor = m.form.GetString("border_color")

					saveConfig(m.config)
					initStyles(m.config.PrimaryColor, m.config.BorderColor)
					m.state = StateDashboard
					return m, tea.Batch(cmds...)
				}
			}

			if m.form.State == huh.StateAborted {
				m.state = StateDashboard
				return m, tea.Batch(cmds...)
			}
		}

		if k, ok := msg.(tea.KeyMsg); ok {
			if k.String() == "ctrl+c" {
				m.cancel()
				return m, tea.Quit
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.state == StateCommenting {
		var cmd tea.Cmd
		m.ta, cmd = m.ta.Update(msg)
		cmds = append(cmds, cmd)

		if k, ok := msg.(tea.KeyMsg); ok {
			if k.String() == "ctrl+c" {
				m.cancel()
				return m, tea.Quit
			}
			if k.String() == "esc" {
				m.state = StateDashboard
				return m, tea.Batch(cmds...)
			}
			if k.String() == "ctrl+s" {
				body := m.ta.Value()
				if strings.TrimSpace(body) != "" {
					cmdTarget := "pr"
					if m.activeTab == TabIssues {
						cmdTarget = "issue"
					}
					c := exec.Command("gh", cmdTarget, "comment", m.commentTarget, "--body", body)
					m.state = StateDashboard
					m.ta.Reset()
					cmds = append(cmds, tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} }))
					return m, tea.Batch(cmds...)
				}
			}
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {

	case tickMsg:
		cmds = append(cmds, doTick(m.config.RefreshInterval))
		if m.state == StateDashboard && m.lists[m.activeTab].HasLoaded && !m.lists[m.activeTab].IsLoading && m.activeTab != TabFiles {
			m.isPolling = true
			cmds = append(cmds, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir))
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if m.width >= 80 {
			m.isDesktop = true
			m.listHeight = m.height - 11
		} else {
			m.isDesktop = false
			m.listHeight = m.height - 13
		}
		if m.listHeight < 4 {
			m.listHeight = 4
		}

		detailWidth := (m.width / 2) - 4
		vpHeight := m.listHeight
		if !m.isDesktop {
			detailWidth = m.width - 4
			vpHeight = m.listHeight / 2
		}
		if vpHeight < 2 {
			vpHeight = 2
		}

		if !m.ready {
			m.vp = viewport.New(detailWidth, vpHeight)
			m.ta = textarea.New()
			m.ta.Placeholder = "Write your markdown comment here..."
			m.ta.CharLimit = 5000
			m.ready = true
		} else {
			m.vp.Width = detailWidth
			m.vp.Height = vpHeight
		}

		taWidth := m.width - 10
		if taWidth < 20 {
			taWidth = 20
		}
		m.ta.SetWidth(taWidth)
		m.ta.SetHeight(10)
		m = updateViewport(m)
		return m, tea.Batch(cmds...)

	case tokenMsg:
		m.githubToken = string(msg)
		m.state = StateDashboard
		m.lists[m.activeTab].IsLoading = true
		cmds = append(cmds, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir))
		return m, tea.Batch(cmds...)

	case actionCompleteMsg:
		m.lists[msg.Tab].IsLoading = true
		m.isPolling = true
		cmds = append(cmds, dispatchFetch(m.ctx, m.githubToken, msg.Tab, 1, m.lists[msg.Tab].CurrentDir))
		return m, tea.Batch(cmds...)

	case editorFinishedMsg:
		if m.activeTab == TabFiles {
			cmds = append(cmds, dispatchFetch(m.ctx, m.githubToken, TabFiles, 1, m.lists[TabFiles].CurrentDir))
		} else {
			m.lists[m.activeTab].IsLoading = true
			m.isPolling = true
			cmds = append(cmds, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir))
		}
		return m, tea.Batch(cmds...)

	case localFilesMsg:
		tab := msg.Tab
		if msg.Err != nil {
			m.lists[tab].Error = msg.Err.Error()
		} else {
			m.lists[tab].Files = msg.Files
			m.lists[tab].CurrentDir = msg.Dir
			m.lists[tab].TotalCount = len(msg.Files)
			m.lists[tab].Error = ""
			m.lists[tab].Cursor = 0
			m.lists[tab].ViewportStart = 0
		}
		m.lists[tab].IsLoading = false
		m.lists[tab].HasLoaded = true
		m = updateViewport(m)
		return m, tea.Batch(cmds...)
	case notificationsMsg:
		tab := msg.Tab
		if msg.Err != nil {
			m.lists[tab].Error = msg.Err.Error()
		} else {
			m.lists[tab].Notifications = msg.Data
			m.lists[tab].TotalCount = len(msg.Data)
			m.lists[tab].Error = ""
		}
		m.lists[tab].IsLoading = false
		m.lists[tab].HasLoaded = true
		m.isPolling = false
		m = updateViewport(m)
		return m, tea.Batch(cmds...)
	case itemsMsg:
		tab := msg.Tab
		if m.isPolling && m.lists[tab].Page <= 1 {
			m.lists[tab].Items = msg.Data.Items
		} else {
			m.lists[tab].Items = append(m.lists[tab].Items, msg.Data.Items...)
		}
		m.lists[tab].TotalCount = msg.Data.TotalCount
		m.lists[tab].IsLoading = false
		m.lists[tab].HasLoaded = true
		m.isPolling = false
		if m.lists[tab].Page == 0 {
			m.lists[tab].Page = 1
		}
		m = updateViewport(m)
		return m, tea.Batch(cmds...)
	case runsMsg:
		tab := msg.Tab
		if msg.Err != nil {
			m.lists[tab].Error = msg.Err.Error()
		} else {
			m.lists[tab].Runs = msg.Runs
			m.lists[tab].TotalCount = len(msg.Runs)
			m.lists[tab].Error = ""
		}
		m.lists[tab].IsLoading = false
		m.lists[tab].HasLoaded = true
		m.isPolling = false
		m = updateViewport(m)
		return m, tea.Batch(cmds...)
	case filesMsg:
		m.files = msg
		m.fileCursor = 0
		m.fileViewportStart = 0
		m.state = StateFileViewer
		return m, tea.Batch(cmds...)
	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}
		switch m.state {
		case StateFiltering:
			return m.handleFilteringKeys(msg)
		case StateFileViewer:
			return m.handleFileViewerKeys(msg)
		case StateHelp:
			return m.handleHelpKeys(msg)
		default:
			return m.handleDashboardKeys(msg)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?":
		m.state = m.previousState
	}
	return m, nil
}

func (m model) handleFilteringKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.state = StateDashboard
	case "backspace":
		if len(m.filterQuery) > 0 {
			m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
			m.lists[m.activeTab].Cursor = 0
			m.lists[m.activeTab].ViewportStart = 0
		}
	case "space":
		m.filterQuery += " "
		m.lists[m.activeTab].Cursor = 0
		m.lists[m.activeTab].ViewportStart = 0
	default:
		if len(msg.String()) == 1 {
			m.filterQuery += msg.String()
			m.lists[m.activeTab].Cursor = 0
			m.lists[m.activeTab].ViewportStart = 0
		}
	}
	m = updateViewport(m)
	return m, nil
}

func (m model) handleFileViewerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.previousState = m.state
		m.state = StateHelp
		return m, nil
	case "esc", "q":
		m.state = StateDashboard
	case "up", "k":
		if m.fileCursor > 0 {
			m.fileCursor--
			if m.fileCursor < m.fileViewportStart {
				m.fileViewportStart = m.fileCursor
			}
		}
	case "down", "j":
		listH := m.height - 8
		if listH < 5 {
			listH = 5
		}
		if m.fileCursor < len(m.files)-1 {
			m.fileCursor++
			if m.fileCursor >= m.fileViewportStart+listH {
				m.fileViewportStart = m.fileCursor - listH + 1
			}
		}
	case "e", "enter":
		file := m.files[m.fileCursor]
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		c := exec.Command(editor, file.Path)
		return m, tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
	}
	return m, nil
}

func (m model) handleDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.cancel()
		return m, tea.Quit
	case "?":
		m.previousState = m.state
		m.state = StateHelp
		return m, nil
	case "esc":
		if m.filterQuery != "" {
			m.filterQuery = ""
			m.lists[m.activeTab].Cursor = 0
			m.lists[m.activeTab].ViewportStart = 0
			m = updateViewport(m)
		}
	case "/":
		m.state = StateFiltering
		return m, nil

	case ",":
		m.tempConfig = m.config

		m.form = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[int]().
					Key("default_tab").
					Title("Default Startup Tab").
					Options(
						huh.NewOption("PRs", TabPRs),
						huh.NewOption("Issues", TabIssues),
						huh.NewOption("CI/CD", TabCICD),
						huh.NewOption("Files", TabFiles),
						huh.NewOption("Inbox", TabInbox),
					).
					Value(&m.tempConfig.DefaultTab),
				huh.NewInput().
					Key("primary_color").
					Title("Primary App Color (ANSI 0-255 or Hex)").
					Value(&m.tempConfig.PrimaryColor),
				huh.NewInput().
					Key("border_color").
					Title("App Border Color (ANSI 0-255 or Hex)").
					Value(&m.tempConfig.BorderColor),
			),
		).WithTheme(huh.ThemeDracula())

		m.state = StateSettings
		return m, m.form.Init()

	case "C", "+":
		m.createTarget = "pr"
		if m.activeTab == TabIssues {
			m.createTarget = "issue"
		}
		titleLabel := "Pull Request Title"
		if m.createTarget == "issue" {
			titleLabel = "Issue Title"
		}

		m.form = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Key("title").
					Title(titleLabel).
					Validate(func(str string) error {
						if len(strings.TrimSpace(str)) == 0 {
							return fmt.Errorf("title cannot be empty")
						}
						return nil
					}),
				huh.NewText().
					Key("body").
					Title("Description (Markdown)").
					Lines(8),
			),
		).WithTheme(huh.ThemeDracula())

		m.state = StateCreating
		return m, m.form.Init()

	case "pgdown", "pgup", "ctrl+d", "ctrl+u":
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd

	case "y":
		val := ""
		if m.activeTab == TabFiles {
			files := getFilteredFiles(m.lists[TabFiles].Files, m.filterQuery)
			if len(files) > 0 {
				val = filepath.Join(m.lists[TabFiles].CurrentDir, files[m.lists[TabFiles].Cursor].Name)
			}
		} else if m.activeTab == TabCICD {
			runs := getFilteredRuns(m.lists[TabCICD].Runs, m.filterQuery)
			if len(runs) > 0 {
				val = runs[m.lists[TabCICD].Cursor].Url
			}
		} else if m.activeTab == TabPRs || m.activeTab == TabIssues {
			items := getFilteredItems(m.lists[m.activeTab].Items, m.filterQuery)
			if len(items) > 0 {
				val = items[m.lists[m.activeTab].Cursor].HtmlUrl
			}
		} else if m.activeTab == TabInbox {
			notifs := getFilteredNotifs(m.lists[TabInbox].Notifications, m.filterQuery)
			if len(notifs) > 0 {
				val = strings.Replace(notifs[m.lists[TabInbox].Cursor].Subject.Url, "api.github.com/repos", "github.com", 1)
				val = strings.Replace(val, "/pulls/", "/pull/", 1)
			}
		}
		if val != "" {
			clipboard.WriteAll(val)
		}
		return m, nil

	case "tab", "right", "l":
		m.activeTab = (m.activeTab + 1) % TabCount
		m.filterQuery = ""
		m.lists[m.activeTab].Cursor = 0
		m.lists[m.activeTab].ViewportStart = 0
		m.vp.GotoTop()
		if !m.lists[m.activeTab].HasLoaded && !m.lists[m.activeTab].IsLoading {
			m.lists[m.activeTab].IsLoading = true
			return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir)
		}
		m = updateViewport(m)
	case "shift+tab", "left", "h":
		m.activeTab = (m.activeTab - 1 + TabCount) % TabCount
		m.filterQuery = ""
		m.lists[m.activeTab].Cursor = 0
		m.lists[m.activeTab].ViewportStart = 0
		m.vp.GotoTop()
		if !m.lists[m.activeTab].HasLoaded && !m.lists[m.activeTab].IsLoading {
			m.lists[m.activeTab].IsLoading = true
			return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir)
		}
		m = updateViewport(m)

	case "up", "k":
		list := &m.lists[m.activeTab]
		if list.Cursor > 0 {
			list.Cursor--
			if list.Cursor < list.ViewportStart {
				list.ViewportStart = list.Cursor
			}
		}
		m.vp.GotoTop()
		m = updateViewport(m)
	case "down", "j":
		list := &m.lists[m.activeTab]
		limit := len(getFilteredItems(list.Items, m.filterQuery))
		if m.activeTab == TabCICD {
			limit = len(getFilteredRuns(list.Runs, m.filterQuery))
		}
		if m.activeTab == TabFiles {
			limit = len(getFilteredFiles(list.Files, m.filterQuery))
		}
		if m.activeTab == TabInbox {
			limit = len(getFilteredNotifs(list.Notifications, m.filterQuery))
		}

		visH := m.listHeight
		if !m.isDesktop {
			visH = m.listHeight / 2
		}
		if visH < 2 {
			visH = 2
		}

		if list.Cursor < limit-1 {
			list.Cursor++
			if list.Cursor >= list.ViewportStart+visH {
				list.ViewportStart = list.Cursor - visH + 1
			}
		}
		m.vp.GotoTop()
		m = updateViewport(m)

	case "t", "T":
		if m.activeTab == TabPRs {
			items := getFilteredItems(m.lists[TabPRs].Items, m.filterQuery)
			if len(items) > 0 {
				pr := items[m.lists[TabPRs].Cursor]
				script := fmt.Sprintf(`
clear; echo "👻 GHOST HANDOFF INITIATED"; echo "========================="
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then echo "❌ Error: Not inside a git repository."; sleep 3; exit 1; fi
ORIGINAL=$(git branch --show-current); STASHED=false
if ! git diff --quiet || ! git diff --cached --quiet; then git stash; STASHED=true; fi
if ! gh pr checkout %s; then echo "❌ Error: Could not checkout PR."; if [ "$STASHED" = true ]; then git stash pop; fi; sleep 3; exit 1; fi
echo "🟢 ENVIRONMENT READY. Type 'exit' to return."; echo "========================="
${SHELL:-sh}
echo "👻 RESTORING ENVIRONMENT..."; git checkout $ORIGINAL
if [ "$STASHED" = true ]; then git stash pop; fi; sleep 1
`, pr.HtmlUrl)
				c := exec.Command("bash", "-c", script)
				return m, tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
			}
		}

	case "v":
		if m.activeTab == TabPRs {
			items := getFilteredItems(m.lists[TabPRs].Items, m.filterQuery)
			if len(items) > 0 {
				pr := items[m.lists[TabPRs].Cursor]
				m.state = StateLoadingFiles
				return m, fetchPRFiles(m.ctx, pr.HtmlUrl)
			}
		} else if m.activeTab == TabCICD {
			runs := getFilteredRuns(m.lists[TabCICD].Runs, m.filterQuery)
			if len(runs) > 0 {
				run := runs[m.lists[TabCICD].Cursor]
				c := exec.Command("sh", "-c", fmt.Sprintf("gh run view %d --log | less -R", run.DatabaseId))
				return m, tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
			}
		}

	case "m":
		if m.activeTab == TabPRs {
			items := getFilteredItems(m.lists[TabPRs].Items, m.filterQuery)
			if len(items) > 0 {
				pr := items[m.lists[TabPRs].Cursor]
				m.lists[TabPRs].IsLoading = true
				return m, executeAction(m.ctx, TabPRs, "merge", pr.HtmlUrl)
			}
		}
	case "c":
		if m.activeTab == TabPRs {
			items := getFilteredItems(m.lists[TabPRs].Items, m.filterQuery)
			if len(items) > 0 {
				pr := items[m.lists[TabPRs].Cursor]
				m.lists[TabPRs].IsLoading = true
				return m, executeAction(m.ctx, TabPRs, "close", pr.HtmlUrl)
			}
		}
	case "x":
		if m.activeTab == TabCICD {
			runs := getFilteredRuns(m.lists[TabCICD].Runs, m.filterQuery)
			if len(runs) > 0 {
				run := runs[m.lists[TabCICD].Cursor]
				m.lists[TabCICD].IsLoading = true
				return m, executeAction(m.ctx, TabCICD, "cancel", fmt.Sprintf("%d", run.DatabaseId))
			}
		}
	case "w":
		if m.activeTab == TabCICD {
			runs := getFilteredRuns(m.lists[TabCICD].Runs, m.filterQuery)
			if len(runs) > 0 {
				run := runs[m.lists[TabCICD].Cursor]
				m.lists[TabCICD].IsLoading = true
				return m, executeAction(m.ctx, TabCICD, "rerun", fmt.Sprintf("%d", run.DatabaseId))
			}
		}

	case "enter", "o", "e":
		if m.activeTab == TabFiles {
			files := getFilteredFiles(m.lists[TabFiles].Files, m.filterQuery)
			if len(files) > 0 {
				file := files[m.lists[TabFiles].Cursor]
				if file.IsDir {
					newDir := filepath.Join(m.lists[TabFiles].CurrentDir, file.Name)
					m.lists[TabFiles].IsLoading = true
					m.filterQuery = ""
					return m, dispatchFetch(m.ctx, m.githubToken, TabFiles, 1, newDir)
				} else {
					editor := os.Getenv("EDITOR")
					if editor == "" {
						editor = "nano"
					}
					c := exec.Command(editor, filepath.Join(m.lists[TabFiles].CurrentDir, file.Name))
					return m, tea.ExecProcess(c, func(err error) tea.Msg { return editorFinishedMsg{err} })
				}
			}
		} else if m.activeTab == TabInbox && msg.String() == "e" {
			notifs := getFilteredNotifs(m.lists[TabInbox].Notifications, m.filterQuery)
			if len(notifs) > 0 {
				notif := notifs[m.lists[TabInbox].Cursor]
				m.lists[TabInbox].IsLoading = true
				return m, executeAction(m.ctx, TabInbox, "mark_read", notif.Id)
			}
		}

		url := ""
		if m.activeTab == TabCICD {
			runs := getFilteredRuns(m.lists[TabCICD].Runs, m.filterQuery)
			if len(runs) > 0 {
				url = runs[m.lists[TabCICD].Cursor].Url
			}
		} else if m.activeTab == TabPRs || m.activeTab == TabIssues {
			items := getFilteredItems(m.lists[m.activeTab].Items, m.filterQuery)
			if len(items) > 0 {
				url = items[m.lists[m.activeTab].Cursor].HtmlUrl
			}
		}

		if m.activeTab == TabInbox && (msg.String() == "enter" || msg.String() == "o") {
			notifs := getFilteredNotifs(m.lists[TabInbox].Notifications, m.filterQuery)
			if len(notifs) > 0 {
				url = strings.Replace(notifs[m.lists[TabInbox].Cursor].Subject.Url, "api.github.com/repos", "github.com", 1)
				url = strings.Replace(url, "/pulls/", "/pull/", 1)
			}
		}

		if url != "" {
			exec.Command("termux-open-url", url).Start()
			exec.Command("xdg-open", url).Start()
			exec.Command("open", url).Start()
		}

	case "n":
		list := &m.lists[m.activeTab]
		if (m.activeTab == TabPRs || m.activeTab == TabIssues) && !list.IsLoading && len(list.Items) < list.TotalCount {
			list.Page++
			list.IsLoading = true
			return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, list.Page, "")
		}

	case "R":
		m.lists[m.activeTab].IsLoading = true
		m.lists[m.activeTab].Error = ""
		m.isPolling = true
		return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir)

	case "r":
		if m.activeTab == TabPRs || m.activeTab == TabIssues {
			items := getFilteredItems(m.lists[m.activeTab].Items, m.filterQuery)
			if len(items) > 0 {
				m.commentTarget = items[m.lists[m.activeTab].Cursor].HtmlUrl
				m.state = StateCommenting
				m.ta.Reset()
				m.ta.Focus()
				return m, textarea.Blink
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press 'ctrl+c' to quit.\n", m.err)
	}
	if m.width == 0 {
		return "\n  Initializing layout...\n"
	}

	switch m.state {
	case StateLoadingToken:
		return "\n  Looking for gh cli token...\n"
	case StateLoadingFiles:
		return "\n  Fetching file tree...\n"
	case StateFileViewer:
		return m.renderFileViewer()
	case StateHelp:
		return m.renderHelp()
	case StateCommenting:
		return m.renderCommenting()
	case StateCreating:
		return m.renderCreating()
	case StateSettings:
		return m.renderSettings()
	}
	return m.renderDashboard()
}

func main() {
	cfg := loadConfig()
	initStyles(cfg.PrimaryColor, cfg.BorderColor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startTab := cfg.DefaultTab
	if startTab < 0 || startTab >= TabCount {
		startTab = 0
	}

	m := model{ctx: ctx, cancel: cancel, state: StateLoadingToken, config: cfg, activeTab: startTab}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Fatal Error: %v\n", err)
	}
}
