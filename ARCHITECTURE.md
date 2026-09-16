# Architecture & Technical Decisions

## 1. Project Overview
This project is a high-performance Terminal User Interface (TUI) for Git, replacing clunky web interfaces and generic terminal commands with a heavily tailored, minimalist visual experience.
- **Core Features**:
  - Main View: Minimalist Git log list (commit hash, message, branch, author, date).
  - Split Detail View: Selected commit expands into a file tree on the left and a highlighted diff view on the right.
  - Interaction: Fully keyboard driven (Up/Down, Enter/Right arrow).

## 2. Tech Stack
- **Language**: Go (Strict typing, strong performance, compiled binary).
- **TUI Framework**: Charmbracelet `bubbletea` for Elm architecture state management.
- **Styling Framework**: Charmbracelet `lipgloss` for modern layout and colors.
- **Git Integration**: Direct execution of `os/exec` wrappers around native Git commands or utilizing `go-git` for complex diff parsing.

## 3. Architecture Pattern
- **Model-Update-View (MUV)**
  - Elm architecture enforced by Bubble Tea.
  - State and logic are completely separated from rendering.
  - Business logic (parsing Git objects, parsing diffs) resides under `core/`.

## 4. Engineering Standards
- **Language**: English only for code, variables, and comments.
- **Naming Conventions**: It is strictly forbidden to use abbreviated variable or function names (e.g., use `fetchCommitHistory` instead of `fetchLog`). Use explicit and descriptive names.
- **Documentation/Comments**: Every struct and function MUST have a descriptive, non-sloppy comment above it.
  - *Format*: `// [Name] is the [entity] that handles...`
  - *Narrative Flow*: When describing the flow inside a function, comments must be narrative and discursive. It is forbidden to use numbered lists.
- **Git History**: Linux Kernel Style.
  - Conventional Commits (`feat(scope): title`).
  - Commit messages must be written as verbose, descriptive paragraphs explaining the rationale behind the changes.
  - It is strictly forbidden to use bullet points or lists in the commit message body.
  - It is strictly forbidden to start the commit body with phrases like "This commit...". Start directly with the rationale or action.
  - Commit body must be hard-wrapped at 72 characters.
- **Workflow & Tooling**:
  - `Makefile`: Central hub for all commands (test, lint, build).
  - Pre-commit hooks to enforce formatting (`go fmt`) and linting (`golangci-lint`).
  - **CI/CD**: GitHub Actions orchestrates automated quality gates on every push/PR (Lint, Test, Format).
