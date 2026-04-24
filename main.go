package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type AppState int

const (
	StateLoadingToken AppState = iota
	StateDashboard
	StateLoadingFiles
	StateFileViewer
	StateFiltering // New state for capturing text input
)
const (
	TabPRs = iota
	TabIssues
	TabCICD
	TabFiles
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
	TotalCount    int
	Page          int
	IsLoading     bool
	HasLoaded     bool
	Error         string
	Cursor        int
	ViewportStart int
	CurrentDir    string
}

type model struct {
	ctx         context.Context
	cancel      context.CancelFunc
	state       AppState
	githubToken string
	err         error
	activeTab   int
	lists       [TabCount]ListData
	width       int
	height      int
	listHeight  int
	isDesktop   bool
	isPolling   bool

	files             []PRFile
	fileCursor        int
	fileViewportStart int

	filterQuery string // New: Stores the active search string
}

// Helper functions for Dynamic Fuzzy Filtering
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

func doTick() tea.Cmd {
	return tea.Tick(time.Second*10, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd {
	m.lists[TabFiles].CurrentDir = "."
	return tea.Batch(fetchGitHubToken(m.ctx), doTick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, doTick())
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
			m.listHeight = m.height - 10
		} else {
			m.isDesktop = false
			m.listHeight = m.height - 18
		}
		if m.listHeight < 4 {
			m.listHeight = 4
		}
		return m, nil

	case tokenMsg:
		m.githubToken = string(msg)
		m.state = StateDashboard
		m.lists[TabPRs].IsLoading = true
		return m, dispatchFetch(m.ctx, m.githubToken, TabPRs, 1, "")

	case actionCompleteMsg:
		m.lists[msg.Tab].IsLoading = true
		m.isPolling = true
		return m, dispatchFetch(m.ctx, m.githubToken, msg.Tab, 1, m.lists[msg.Tab].CurrentDir)

	case editorFinishedMsg:
		if m.activeTab == TabFiles {
			return m, dispatchFetch(m.ctx, m.githubToken, TabFiles, 1, m.lists[TabFiles].CurrentDir)
		}
		return m, nil

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
		return m, nil

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
		return m, nil

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
		return m, nil

	case filesMsg:
		m.files = msg
		m.fileCursor = 0
		m.fileViewportStart = 0
		m.state = StateFileViewer
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}

		// FILTERING ENGINE
		if m.state == StateFiltering {
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
			return m, nil
		}

		if m.state == StateFileViewer {
			switch msg.String() {
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

		switch msg.String() {
		case "q":
			m.cancel()
			return m, tea.Quit
		case "esc":
			if m.filterQuery != "" {
				m.filterQuery = ""
				m.lists[m.activeTab].Cursor = 0
				m.lists[m.activeTab].ViewportStart = 0
			}
		case "/": // Trigger Filter Mode
			m.state = StateFiltering
			return m, nil

		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % TabCount
			m.filterQuery = ""
			m.lists[m.activeTab].Cursor = 0
			m.lists[m.activeTab].ViewportStart = 0 // Reset filter on tab change
			if !m.lists[m.activeTab].HasLoaded && !m.lists[m.activeTab].IsLoading {
				m.lists[m.activeTab].IsLoading = true
				return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir)
			}
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + TabCount) % TabCount
			m.filterQuery = ""
			m.lists[m.activeTab].Cursor = 0
			m.lists[m.activeTab].ViewportStart = 0
			if !m.lists[m.activeTab].HasLoaded && !m.lists[m.activeTab].IsLoading {
				m.lists[m.activeTab].IsLoading = true
				return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir)
			}

		case "up", "k":
			list := &m.lists[m.activeTab]
			if list.Cursor > 0 {
				list.Cursor--
				if list.Cursor < list.ViewportStart {
					list.ViewportStart = list.Cursor
				}
			}
		case "down", "j":
			list := &m.lists[m.activeTab]
			limit := len(getFilteredItems(list.Items, m.filterQuery))
			if m.activeTab == TabCICD {
				limit = len(getFilteredRuns(list.Runs, m.filterQuery))
			}
			if m.activeTab == TabFiles {
				limit = len(getFilteredFiles(list.Files, m.filterQuery))
			}

			if list.Cursor < limit-1 {
				list.Cursor++
				if list.Cursor >= list.ViewportStart+m.listHeight {
					list.ViewportStart = list.Cursor - m.listHeight + 1
				}
			}

		// Unifying 'v' to mean "View Details" (Files for PR, Logs for CI/CD)
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
					// Pipe GitHub logs directly to standard less pager with color rendering
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
		case "r":
			m.lists[m.activeTab].IsLoading = true
			m.lists[m.activeTab].Error = ""
			m.isPolling = true
			return m, dispatchFetch(m.ctx, m.githubToken, m.activeTab, 1, m.lists[m.activeTab].CurrentDir)
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
	}
	return m.renderDashboard()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := tea.NewProgram(model{ctx: ctx, cancel: cancel, state: StateLoadingToken}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Fatal Error: %v\n", err)
	}
}
