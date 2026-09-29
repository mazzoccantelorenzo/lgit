# lgit

A small terminal UI for Git history and commit diffs.

## Install

Requires Git and Go 1.24.4 or newer.

```sh
git clone https://github.com/mazzoccantelorenzo/lgit.git
cd lgit
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/lgit" .
```

If needed, add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile.
Run `lgit` inside a Git repository.

## Ghostty

To navigate preview files with **Shift↑/Shift↓**, add this to `~/.config/ghostty/config`:

```ini
keybind = shift+arrow_up=csi:1;2A
keybind = shift+arrow_down=csi:1;2B
```

Reload Ghostty with **⌘⇧,**. These keys will no longer select terminal text.

## Updates

At startup, lgit checks GitHub in the background and warns if the installed build is behind the default branch.
To update, run `git pull` in the lgit clone and repeat the `go build` command above.
