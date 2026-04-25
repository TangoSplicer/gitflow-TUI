package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tokenMsg string
type itemsMsg struct {
	Tab  int
	Data SearchResponse
}
type runsMsg struct {
	Tab  int
	Runs []RunItem
	Err  error
}
type notificationsMsg struct {
	Tab  int
	Data []NotificationItem
	Err  error
}
type errMsg struct{ err error }
type tickMsg time.Time
type actionCompleteMsg struct{ Tab int }
type editorFinishedMsg struct{ err error }

type PRFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}
type prViewResponse struct {
	Files []PRFile `json:"files"`
}
type filesMsg []PRFile

type LocalFile struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	Mode    os.FileMode
}
type localFilesMsg struct {
	Tab   int
	Dir   string
	Files []LocalFile
	Err   error
}

// Notification Models
type NotificationItem struct {
	Id        string    `json:"id"`
	Reason    string    `json:"reason"`
	Unread    bool      `json:"unread"`
	UpdatedAt time.Time `json:"updated_at"`
	Subject   struct {
		Title string `json:"title"`
		Type  string `json:"type"`
		Url   string `json:"url"`
	} `json:"subject"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func fetchGitHubToken(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(reqCtx, "gh", "auth", "token")
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
		out, err := cmd.Output()
		if err != nil {
			if reqCtx.Err() == context.Canceled {
				return nil
			}
			return errMsg{fmt.Errorf("failed to get gh token: %w", err)}
		}
		return tokenMsg(strings.TrimSpace(string(out)))
	}
}

func fetchNotifications(ctx context.Context, tab int) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(reqCtx, "gh", "api", "notifications")
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
		out, err := cmd.Output()
		if err != nil {
			if reqCtx.Err() == context.Canceled {
				return nil
			}
			return notificationsMsg{Tab: tab, Err: fmt.Errorf("failed to fetch notifications: %w", err)}
		}
		var notifs []NotificationItem
		if err := json.Unmarshal(out, &notifs); err != nil {
			return notificationsMsg{Tab: tab, Err: fmt.Errorf("parse error: %w", err)}
		}
		return notificationsMsg{Tab: tab, Data: notifs}
	}
}

func fetchRuns(ctx context.Context, tab int) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(reqCtx, "gh", "run", "list", "--limit", "30", "--json", "databaseId,name,status,conclusion,createdAt,url")
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
		out, err := cmd.Output()
		if err != nil {
			if reqCtx.Err() == context.Canceled {
				return nil
			}
			return runsMsg{Tab: tab, Err: fmt.Errorf("Not a repo. cd into a git directory to view CI.")}
		}
		var runs []RunItem
		if err := json.Unmarshal(out, &runs); err != nil {
			return runsMsg{Tab: tab, Err: fmt.Errorf("Failed to parse runs: %v", err)}
		}
		return runsMsg{Tab: tab, Runs: runs}
	}
}

func fetchTabItems(ctx context.Context, token string, tab int, page int) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		query := "is:issue+is:open+assignee:@me"
		if tab == TabPRs {
			query = "is:pr+is:open+author:@me"
		}
		req, _ := http.NewRequestWithContext(reqCtx, "GET", fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=30&page=%d", query, page), nil)
		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("Accept", "application/vnd.github.v3+json")
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			if reqCtx.Err() == context.Canceled {
				return nil
			}
			return errMsg{fmt.Errorf("network error: %w", err)}
		}
		defer resp.Body.Close()
		var result SearchResponse
		json.NewDecoder(resp.Body).Decode(&result)
		return itemsMsg{Tab: tab, Data: result}
	}
}

func fetchLocalFiles(tab int, dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return localFilesMsg{Tab: tab, Err: err}
		}

		var files []LocalFile
		abs, _ := filepath.Abs(dir)
		if abs != "/" {
			files = append(files, LocalFile{Name: "..", IsDir: true})
		}

		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, LocalFile{Name: e.Name(), IsDir: e.IsDir(), Size: info.Size(), ModTime: info.ModTime(), Mode: info.Mode()})
		}

		sort.Slice(files, func(i, j int) bool {
			if files[i].Name == ".." {
				return true
			}
			if files[j].Name == ".." {
				return false
			}
			if files[i].IsDir && !files[j].IsDir {
				return true
			}
			if !files[i].IsDir && files[j].IsDir {
				return false
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})
		return localFilesMsg{Tab: tab, Dir: dir, Files: files}
	}
}

func dispatchFetch(ctx context.Context, token string, tab int, page int, dir string) tea.Cmd {
	if tab == TabCICD {
		return fetchRuns(ctx, tab)
	}
	if tab == TabInbox {
		return fetchNotifications(ctx, tab)
	}
	if tab == TabFiles {
		if dir == "" {
			dir = "."
		}
		return fetchLocalFiles(tab, dir)
	}
	return fetchTabItems(ctx, token, tab, page)
}

func executeAction(ctx context.Context, tab int, action string, target string) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		var args []string
		switch action {
		case "merge":
			args = []string{"pr", "merge", target, "--squash"}
		case "close":
			args = []string{"pr", "close", target}
		case "cancel":
			args = []string{"run", "cancel", target}
		case "rerun":
			args = []string{"run", "rerun", target}
		case "mark_read":
			args = []string{"api", "-X", "PATCH", fmt.Sprintf("notifications/threads/%s", target)}
		}
		cmd := exec.CommandContext(reqCtx, "gh", args...)
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
		if _, err := cmd.CombinedOutput(); err != nil {
			if reqCtx.Err() == context.Canceled {
				return nil
			}
			return errMsg{fmt.Errorf("action failed (%s): %w", action, err)}
		}
		return actionCompleteMsg{Tab: tab}
	}
}

func fetchPRFiles(ctx context.Context, url string) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(reqCtx, "gh", "pr", "view", url, "--json", "files")
		cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
		out, err := cmd.Output()
		if err != nil {
			if reqCtx.Err() == context.Canceled {
				return nil
			}
			return errMsg{fmt.Errorf("failed to fetch files: %w", err)}
		}
		var resp prViewResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return errMsg{fmt.Errorf("failed to parse files: %w", err)}
		}
		return filesMsg(resp.Files)
	}
}
