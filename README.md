# lgit

<p align="center">
  <strong style="font-size: 1.25rem;">Blazing fast, keyboard-first Terminal UI for Git history, diff exploration, and Neovim workflows.</strong>
</p>

<p align="center">
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img src="https://img.shields.io/badge/TUI-Bubbletea-FF5F87?style=flat-square" alt="Bubbletea"></a>
  <a href="https://neovim.io"><img src="https://img.shields.io/badge/Editor-Neovim-57A143?style=flat-square&logo=neovim" alt="Neovim"></a>
  <a href="https://github.com/mazzoccantelorenzo/lgit/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
</p>

---

```
 ╭─ Commits (master) ────────────────╮ ╭─ Diff Preview: [M] core/git.go ────────────────────────────╮
 │ > cb9c5dc  chore(api): sync schema│ │ @@ -94,6 +94,22 @@ func DetectBaseBranch...             │
 │   7bf334a  refactor(db): decouple │ │ -       baseBranch := "master"                             │
 │   59546f6  refactor(reports): add │ │ +       baseBranch, err := detectUpstream(current)         │
 │   2c4b252  feat(home): prompt fb  │ │ +       if err != nil {                                    │
 │                                   │ │ +           return "main", nil                             │
 │   • Lorenzo • 2026-11-12 13:36    │ │ +       }                                                  │
 ╰───────────────────────────────────╯ ╰────────────────────────────────────────────────────────────╯
   [Tab] Switch Branch   [Enter] Inspect / Edit in Neovim   [p/b] PR View   [Space] Expand Commit
```

> **lgit** is a lightweight, distraction-free Git TUI written in Go with [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). It bridges the gap between raw `git log -p` and heavy GUI tools, giving you instant commit browsing, split diff viewing, seamless Neovim integration, and smart uncommitted changes detection.

---

## ✨ Features

- ⚡ **Zero Lag & Instant Startup**: Native Go binary with minimal footprint and instant rendering across large repositories.
- 🔀 **Branch Switching on the Fly**: Cycle through local branches with `Tab` / `Shift+Tab` without leaving the interface.
- 🔎 **Real-Time Fuzzy Filtering**: Search through commits, authors, hashes, or modified files with instant fuzzy search (`/`).
- 📝 **Native Neovim Integration**: Press `Enter` on any modified file to jump straight into Neovim. When you quit Neovim, your position, diff scroll, and filters are preserved.
- 🚨 **Uncommitted Local Changes Guard**: Displays an unmistakable warning when a file in history has uncommitted (or staged) modifications in your working tree.
- 🌿 **Pull Request Cumulative Diff (`p` / `b`)**: Compare your active feature branch against its auto-detected base branch (`main` / `master` / tracking upstream) in a single unified view.
- ✍️ **Git History Reword (`e` / `f`)**: Quickly trigger `git history reword` directly from the commit list, with optional automated 72-char auto-wrap formatting.
- 🔄 **Non-Blocking Background Updates**: Checks GitHub in the background at startup and gently notifies you if your local binary is behind the remote default branch.

---

## 📦 Installation

### Prerequisites
- **Git** (2.20+)
- **Go** (1.24+ recommended)
- *(Optional)* **Neovim** (`nvim` in `$PATH` for the editor jump feature)

### Quick Build & Install

```sh
# Clone the repository
git clone https://github.com/mazzoccantelorenzo/lgit.git
cd lgit

# Build and install to ~/.local/bin
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/lgit" .
```

Ensure `$HOME/.local/bin` is in your `$PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

---

## 🚀 Usage

Run `lgit` inside any Git repository:

```sh
lgit
```

---

## ⌨️ Keybindings

### 1. Commit History View (Main Mode)

| Key | Action |
| :--- | :--- |
| `↑` / `k`, `↓` / `j` | Navigate commits |
| `Enter` / `l` / `→` | Enter commit file view and inspect modified files |
| `Tab` / `Shift+Tab` | Switch to the next / previous Git branch |
| `Space` | Toggle expanded commit message and detailed metadata |
| `Shift+↑` / `Shift+↓` | Scroll preview diff / file preview directly from commit list |
| `p` / `b` | Toggle **PR View** (compare current branch vs base branch) |
| `e` | Interactive Git commit reword (`git history reword <sha>`) |
| `f` | Auto-format & reword commit message (72-char hard wrap in Neovim) |
| `/` | Filter commits by hash, message, or author |
| `q` / `Ctrl+C` | Quit `lgit` |

### 2. File & Diff View (Detail / PR Mode)

| Key | Action |
| :--- | :--- |
| `↑` / `k`, `↓` / `j` | Navigate modified files |
| `Enter` | Open selected file in **Neovim** (returns to exact state on exit) |
| `→` / `l` | Focus diff viewport for line-by-line scrolling |
| `←` / `h` | Return focus from diff viewport to file list |
| `Esc` / `h` (at file level) | Back to commit list |
| `p` / `b` | Exit PR view back to commit list |
| `/` | Filter modified files by filename / path |

---

## ⚡ Terminal Configurations

### Ghostty
To navigate diff previews with `Shift+↑` and `Shift+↓` without terminal text selection conflicts, add the following to `~/.config/ghostty/config`:

```ini
keybind = shift+arrow_up=csi:1;2A
keybind = shift+arrow_down=csi:1;2B
```

Reload Ghostty with `⌘ + ⇧ + ,`.

---

## 🔄 Updating

`lgit` automatically checks if a newer version exists on GitHub at startup. To update your local install:

```sh
cd /path/to/lgit
git pull
go build -o "$HOME/.local/bin/lgit" .
```

---

## 🛠️ Contributing & Development

Contributions, bug reports, and PRs are welcome!

```sh
# Run tests
go test ./...

# Run tests with race detection
go test -race ./...
```

---

## 📄 License

Distributed under the **MIT License**. See `LICENSE` for more information.
