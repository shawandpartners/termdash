# TermDash Feature Catalog

Complete catalog of features to integrate into Wave Terminal.

## Feature Matrix: What Wave Has vs What We Need to Build

### Already Built Into Wave (Free)
| Feature | Wave Equivalent | Notes |
|---------|----------------|-------|
| Terminal PTY management | Block controller + shell | Rock-solid, durable sessions |
| xterm.js rendering | xterm.js 5.5 + WebGL | Handles TUI apps perfectly |
| Terminal resize | FitAddon + backend sync | Automatic |
| Multiple sessions/tabs | Tab + workspace system | Multi-window support too |
| Session persistence | Durable sessions | Survives restarts natively |
| Dark theme | Built-in themes | Customizable |
| AI chat | WaveAI panel | Multi-provider (OpenAI, Anthropic planned) |
| File browser | Preview view | Directory nav, file editing |
| Keyboard shortcuts | Keybinding system | Configurable |
| Split panes | Block layout system | Drag-and-drop |
| Web browser view | WebView block | Embedded Chromium |
| System monitoring | Sysinfo view | CPU plots, metrics |
| Code editor | Monaco integration | Syntax highlighting, diffs |
| Configuration system | Settings + metadata | Hot-reload, hierarchical |
| Drag and drop | DnD provider | Built-in |
| Widget system | widgets.json | Easy to extend |

### Needs Building (TermDash-Specific Features)

#### Priority 1: Claude Code Integration
| Feature | Description | Complexity |
|---------|-------------|-----------|
| Claude Code launcher widget | One-click launch `claude` CLI in a terminal block | Low |
| Claude session ID tracking | Track `--session-id` per block for resume | Medium |
| Claude resume/continue | Resume archived Claude sessions with `--resume`/`--continue` | Medium |
| Claude status detection | Detect needs-input/active/idle from terminal output | Medium |
| Bell notification | Audio + visual alert when Claude finishes | Low |

#### Priority 2: Session Management Enhancements
| Feature | Description | Complexity |
|---------|-------------|-----------|
| Session archive | Archive sessions (preserve state, remove from active) | Medium |
| Session resume from archive | Restore archived session with same config | Medium |
| Session context menu | Right-click Archive/Delete on tabs | Low |
| Session status indicators | Color-coded dots (active/idle/needs-input/exited) | Low |
| Auto-generated session titles | AI summary of session content (3-8 word titles) | Medium |

#### Priority 3: Task Panel
| Feature | Description | Complexity |
|---------|-------------|-----------|
| Task panel view | Right sidebar with task list (CRUD) | Medium |
| Task persistence | Store tasks in Wave's object store | Medium |
| AI task suggestions | Generate tasks from session context | Medium |
| Task drag-drop reorder | Reorder via drag within panel | Low |
| Task injection | Paste task text into active terminal | Low |
| Task filtering | Active/Archived/All tabs | Low |
| Task hotkeys | Cmd+Shift+S suggest, Cmd+Shift+N toggle, etc. | Low |

#### Priority 4: Dashboard / Master View
| Feature | Description | Complexity |
|---------|-------------|-----------|
| Session overview grid | Tile grid showing all sessions with status | Medium |
| Cross-session AI chat | Ask Claude about all active sessions | Medium |

#### Priority 5: Compound Engineering
| Feature | Description | Complexity |
|---------|-------------|-----------|
| Learnings extraction | Extract insights from archived sessions | Medium |
| Learnings injection | Auto-inject relevant learnings into new Claude sessions | Medium |
| Learnings persistence | Store in Wave's object store | Low |

#### Priority 6: Transcript System
| Feature | Description | Complexity |
|---------|-------------|-----------|
| Input/output tracking | Log cleaned user input and terminal output | Medium |
| ANSI stripping | Clean escape codes from stored output | Low |
| Full-text search | Search across all session transcripts | Medium |
| Transcript backfill summaries | Generate titles for old sessions on startup | Low |

## Data Storage Mapping

| TermDash Storage | Wave Terminal Equivalent |
|-----------------|------------------------|
| `~/.termdash/sessions.json` | Wave's SQLite database (WaveObject store) |
| `~/.termdash/transcripts/*.jsonl` | Block file storage (`filestore.WFS`) |
| `~/.termdash/learnings.json` | Custom WaveObject type or config |
| `~/.termdash/images/` | Block file attachments |
| `localStorage` (tasks) | WaveObject store (persistent) |

## Claude Code CLI Flags Reference

| Flag | Purpose | When Used |
|------|---------|-----------|
| `--session-id <uuid>` | Start new session with specific ID | New Claude session |
| `--resume <uuid>` | Resume specific past session | Resume from archive |
| `--continue` | Resume most recent session in cwd | Resume without session ID |
| `--append-system-prompt '<text>'` | Inject system context | New session with learnings |
| `-p` | Pipe mode (non-interactive) | Summary/learnings generation |
| `--model haiku` | Use fast model | Summary/learnings (cost-efficient) |
