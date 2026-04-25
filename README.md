# GitFlow TUI

A radically fast, universally responsive GitHub command center built for the terminal. 

GitFlow TUI was engineered with a unique constraint: it was built entirely from a mobile terminal device. Because of this, it features a custom Adaptive Breakpoint engine that seamlessly scales from a highly compressed, stacked mobile layout into a rich, split-pane desktop dashboard the moment your screen width allows it.

No YAML configuration files. No heavy electron wrappers. Just a compiled Go binary that uses your current directory to figure out what you need to see.

## Features

* **Universal Adaptive Layout:** Automatically pivots between Mobile (stacked) and Desktop (split-pane) layouts based on terminal column width.
* **Global & Local Context:** The PR and Issues tabs aggregate your tasks globally across all repositories. The CI/CD and Files tabs automatically infer your context based on the local git repository you run the tool from.
* **The Action Engine:** Squash and merge PRs, close issues, or cancel hanging CI/CD pipelines directly from the dashboard.
* **Editor Handoff Architecture:** Browse your local repository file tree. Pressing enter on a file suspends the dashboard, hands terminal control to your system editor (Neovim, Nano, Helix), and seamlessly resumes the TUI the second you save and quit.
* **Deep Observability:** View color-coded diff stats for Pull Requests, or stream raw, colorized GitHub Action failure logs directly into the terminal pager.
* **Fuzzy Filtering:** Instantly search across hundreds of PRs, pipelines, or files by typing queries directly into the UI.
* **Real-Time Polling:** Background timers silently poll the GitHub API, updating CI/CD pipeline statuses from pending to success while you watch.

## Prerequisites

GitFlow TUI relies on the official GitHub CLI for authentication and raw data fetching. 

1. Install Go (1.20+)
2. Install the GitHub CLI (gh)
3. Ensure you are authenticated: gh auth login

## Installation

Clone the repository and build the binary:

```bash
git clone [https://github.com/YOUR_USERNAME/gitflow-TUI.git](https://github.com/YOUR_USERNAME/gitflow-TUI.git)
cd gitflow-TUI
go build -ldflags="-s -w" -o gitflow .
mv gitflow /usr/local/bin/  # Or $PREFIX/bin/ if using Termux
## Usage
​Navigate to any local git repository on your machine and launch the dashboard:                                              cd my-project
gitflow
## Global Keybindings
​Tab / Shift+Tab: Cycle between panes (PRs, Issues, CI/CD, Files).
​j / k (or Up / Down): Navigate lists.
​/: Open the fuzzy filter search bar (Press Esc to clear).
​o: Open the highlighted item in your default web browser.
​r: Force a manual data refresh.
​q / Ctrl+C: Quit the application.                             ## Tab-Specific Actions                                       # ## Pull Requests
​m: Squash and merge the selected PR.
​c: Close the selected PR.
​v: Open the File Viewer overlay to see color-coded line additions/deletions.                                                # ## CI/CD Pipelines
​x: Cancel a running pipeline.
​w: Re-run a failed pipeline.
​v: Stream the raw pipeline execution logs into your terminal pager.                                                         # ## Local Files
​Enter: Navigate into a directory, or open a file in your $EDITOR.                                                           ## Architecture
​GitFlow TUI is built in Go using the Bubble Tea framework for the state machine and event loop, alongside Lipgloss for layout and terminal styling. It leverages a multi-file package structure separating the API interactions, UI rendering logic, and the core update loop.                                        ## License
​MIT License. Feel free to fork, break, and rebuild it.
