# 🌊 GitFlow TUI

A blazing fast, mobile-first, power-user terminal user interface (TUI) for GitHub. Built entirely in Go, GitFlow sits on top of the standard GitHub CLI (`gh`) to provide a beautiful, asynchronous, and deeply integrated dashboard for your repositories. 

Designed specifically to run flawlessly in environments like Termux on Android, as well as standard desktop terminals.

## ✨ Features

- **Multi-Tab Dashboard:** Seamlessly navigate between Pull Requests, Issues, CI/CD Pipelines, Local Files, and your GitHub Notifications Inbox.
- **Native Interactions:** Read rich Markdown, submit comments, and create new Pull Requests or Issues entirely within the TUI using native floating forms.
- **The Ghost Handoff:** Press `t` on any Pull Request to instantly stash your current work, check out the PR branch to test the code locally, and perfectly restore your original environment the moment you exit.
- **CI/CD Management:** Monitor GitHub Actions in real-time. View logs, cancel stuck runs, or trigger reruns with single keystrokes.
- **Dynamic Settings:** A built-in configuration editor allows you to completely remap the application's primary and border colors on the fly.
- **Fuzzy Filtering:** Type `/` to instantly filter hundreds of PRs, files, or notifications locally without waiting for API calls.

## 🚀 Installation & Setup

### Prerequisites
1. **Go:** (1.21 or higher recommended)
2. **GitHub CLI (`gh`):** Installed and authenticated.
   ```bash
   gh auth login
3. **​Git: Installed and configured.                                                                                         ### Building from Source                                      ```bash
git clone [https://github.com/TangoSplicer/gitflow-tui.git](https://github.com/TangoSplicer/gitflow-tui.git)
cd gitflow-tui
go mod tidy
go build -ldflags="-s -w" -o gitflow .
                                                              ### Move the binary to your system path (e.g., in Termux):    mv gitflow $PREFIX/bin/                                                                                                      Command Palette
​GitFlow operates primarily via single-keystroke commands. Press ? anywhere in the app to open the Help Overlay.
​Global Navigation:
​tab / l: Next tab
​shift+tab / h: Previous tab
​/: Filter active list
​,: Open Settings Menu
​?: Toggle Help Menu
​q: Quit
​Interactions (PRs & Issues):
​+ / C: Create new PR / Issue
​r: Reply (Open commenting engine)
​t: Ghost Handoff (Checkout PR locally)
​v: View changed files (Diff)
​m: Merge PR
​c: Close PR/Issue
​enter / o: Open in Browser
​Interactions (CI/CD & Inbox):
​v: View workflow logs
​w / x: Rerun / Cancel workflow
​e: Mark notification as read
​🙏 Acknowledgments & Technologies
​This project stands on the shoulders of giants. A massive thank you to the open-source community, specifically:
​Charmbracelet: The absolute pioneers of modern CLI tooling.
​Bubble Tea: The Elm-inspired framework that powers the entire asynchronous state machine of this app.
​Lip Gloss: For the beautiful, responsive styling, borders, and layout rendering.
​Bubbles: For the robust viewport and textarea components.
​Huh: For the elegant, dynamic form engine used in PR creation and our settings menu.
​Glamour: For making Markdown look incredible inside a terminal.
​Atotto Clipboard: For handling seamless, cross-platform clipboard copy/pasting.
​GitHub CLI: For providing the robust, secure authentication and API layer that powers our data fetching.
​Built for the Power User Era.
