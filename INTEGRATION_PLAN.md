# TermDash Integration Plan

Step-by-step plan to integrate TermDash features into Wave Terminal.

## Phase 1: Claude Code Widget & Launcher (Day 1)

### 1.1 Create Claude Code Widget
Add a "Claude Code" widget that launches `claude` CLI in a terminal block.

**Files to create:**
- None needed (pure config)

**Files to modify:**
- `pkg/wconfig/defaultconfig/widgets.json` — Add Claude Code widget definition:
```json
{
    "defwidget@claude": {
        "display:order": -4.5,
        "icon": "sparkles",
        "color": "#bc8cff",
        "label": "claude",
        "description": "Launch Claude Code session",
        "blockdef": {
            "meta": {
                "view": "term",
                "controller": "shell",
                "cmd": "claude",
                "termdash:type": "claude"
            }
        }
    }
}
```

### 1.2 Claude Session ID Tracking
Track Claude session IDs in block metadata so sessions can be resumed.

**Files to modify:**
- `pkg/blockcontroller/shellcontroller.go` — On block start, if `termdash:type` == "claude":
  - Generate UUID for `termdash:claudeSessionId` if not set
  - Build claude command with `--session-id <uuid>`
  - Store session ID in block metadata

### 1.3 Claude Resume Support
Enable resuming archived Claude sessions.

**Files to modify:**
- `pkg/blockcontroller/shellcontroller.go` — Check for `termdash:claudeSessionId` in metadata:
  - If `termdash:resume` == true AND `termdash:claudeSessionId` exists: use `claude --resume <id>`
  - If `termdash:resume` == true AND no session ID: use `claude --continue`

---

## Phase 2: Session Status & Notifications (Day 1-2)

### 2.1 Claude Status Detection
Detect when Claude is waiting for input vs actively working.

**Files to create:**
- `pkg/termdash/statusdetector.go` — Terminal output analyzer:
  - Monitor last line of terminal output
  - Match against prompt patterns (same regexes as TermDash)
  - Emit status change events

**Files to modify:**
- `pkg/blockcontroller/shellcontroller.go` — Integrate status detector for claude blocks
- Block metadata: set `termdash:status` = "active" | "idle" | "needs-input"

### 2.2 Bell Notification
Play audio + show visual indicator when Claude finishes.

**Files to modify:**
- `frontend/app/view/term/term-model.ts` — Watch `termdash:status` metadata changes
- `frontend/app/tab/tab.tsx` — Show notification badge on tab when status changes to needs-input
- `frontend/app/store/global.ts` — Add bell sound atom/handler

### 2.3 Status Indicator on Tabs
Color-code tab indicators based on Claude session status.

**Files to modify:**
- `frontend/app/tab/tab.tsx` — Read `termdash:status` from block metadata, apply color classes
- `frontend/app/tab/tab.scss` — Add status dot styles (green=active, orange=needs-input, gray=idle, red=exited)

---

## Phase 3: Auto-Generated Session Titles (Day 2)

### 3.1 Summary Engine Service
Backend service that generates titles from terminal content.

**Files to create:**
- `pkg/service/termdash/summaryservice.go` — Go service:
  - Poll active blocks every 15 seconds
  - Extract last N lines of terminal content from block file store
  - Call `claude -p --model haiku` to generate 3-8 word titles
  - Store title in block metadata `termdash:summary`
  - Emit update events

**Files to modify:**
- `pkg/service/service.go` — Register SummaryService in ServiceMap
- `cmd/server/main-server.go` — Start summary polling goroutine

### 3.2 Display Titles in UI
Show AI-generated titles on tabs.

**Files to modify:**
- `frontend/app/tab/tab.tsx` — Prefer `termdash:summary` over default tab name
- `frontend/app/block/blockutil.tsx` — Use summary in block name resolution

---

## Phase 4: Task Panel (Day 2-3)

### 4.1 Task Data Model
Store tasks as WaveObjects in the database.

**Files to create:**
- `pkg/waveobj/task.go` — Task WaveObject type:
```go
type Task struct {
    OID         string `json:"oid"`
    Version     int64  `json:"version"`
    Text        string `json:"text"`
    Done        bool   `json:"done"`
    Archived    bool   `json:"archived"`
    Suggested   bool   `json:"suggested"`
    Order       int    `json:"order"`
    CreatedTime int64  `json:"createdtime"`
    UpdatedTime int64  `json:"updatedtime"`
}
```

### 4.2 Task Service
CRUD operations for tasks.

**Files to create:**
- `pkg/service/termdash/taskservice.go` — Go service with methods:
  - `ListTasks()` — Get all tasks
  - `CreateTask(text)` — Create new task
  - `UpdateTask(id, fields)` — Update task
  - `DeleteTask(id)` — Delete task
  - `ReorderTasks(ids)` — Set order
  - `SuggestTasks(context)` — AI-generate suggestions

**Files to modify:**
- `pkg/service/service.go` — Register TaskService

### 4.3 Task Panel View
React component for the task sidebar.

**Files to create:**
- `frontend/app/view/taskpanel/taskpanel-model.ts` — ViewModel with task atoms
- `frontend/app/view/taskpanel/taskpanel.tsx` — React component:
  - Task list with checkboxes
  - Add task input
  - Filter tabs (Active/Archived/All)
  - Drag-drop reorder
  - Delete buttons
  - AI suggest button
  - Task injection (drag to terminal or hotkey)
- `frontend/app/view/taskpanel/taskpanel.scss` — Styling

**Files to modify:**
- `frontend/app/block/block.tsx` — Register TaskPanelViewModel in BlockRegistry
- `pkg/wconfig/defaultconfig/widgets.json` — Add task panel widget

### 4.4 Task Hotkeys
Keyboard shortcuts for task management.

**Files to modify:**
- `frontend/app/store/keymodel.ts` — Add Cmd+Shift+S (suggest), Cmd+Shift+N (toggle done), etc.

---

## Phase 5: Session Archive & Search (Day 3-4)

### 5.1 Archive Service
Archive and restore session blocks.

**Files to create:**
- `pkg/service/termdash/archiveservice.go` — Go service:
  - `ArchiveBlock(blockId)` — Save block state, mark as archived in metadata
  - `RestoreBlock(blockId)` — Restore block, re-create terminal
  - `ListArchived()` — Get all archived blocks
  - `SearchArchive(query)` — Full-text search

### 5.2 Archive UI
Archive browser in sidebar or dedicated view.

**Files to create:**
- `frontend/app/view/archive/archive-model.ts`
- `frontend/app/view/archive/archive.tsx` — Searchable list of archived sessions with Resume/Delete buttons
- `frontend/app/view/archive/archive.scss`

**Files to modify:**
- `frontend/app/block/block.tsx` — Register ArchiveViewModel
- `pkg/wconfig/defaultconfig/widgets.json` — Add archive widget

### 5.3 Context Menu on Tabs
Right-click menu for archive/delete.

**Files to modify:**
- `frontend/app/tab/tab.tsx` — Add context menu with Archive and Delete options

---

## Phase 6: Transcript System (Day 4)

### 6.1 Transcript Recording
Record cleaned terminal content to block file store.

**Files to create:**
- `pkg/termdash/transcript.go` — Transcript recorder:
  - Hook into shell controller output stream
  - Strip ANSI codes, deduplicate, clean animation noise
  - Track user input vs output
  - Write JSONL entries to block file store

**Files to modify:**
- `pkg/blockcontroller/shellcontroller.go` — Integrate transcript recorder

### 6.2 Full-Text Search
Search across all transcripts.

**Files to create:**
- `pkg/service/termdash/searchservice.go` — Full-text search across transcript files

---

## Phase 7: Compound Engineering (Day 5)

### 7.1 Learnings Engine
Extract and inject learnings.

**Files to create:**
- `pkg/service/termdash/learningsservice.go` — Go service:
  - `ExtractLearnings(blockId)` — AI-extract insights from transcript
  - `GetRelevantLearnings(cwd)` — Score and select relevant learnings
  - `BuildContext(cwd)` — Format learnings for `--append-system-prompt`

### 7.2 Auto-Inject Learnings
Inject into new Claude sessions automatically.

**Files to modify:**
- `pkg/blockcontroller/shellcontroller.go` — On Claude block start, call learnings service to build context and append to command

---

## Phase 8: Dashboard (Day 5-6)

### 8.1 Session Dashboard View
Overview of all active sessions.

**Files to create:**
- `frontend/app/view/dashboard/dashboard-model.ts`
- `frontend/app/view/dashboard/dashboard.tsx` — Grid of session tiles with status/summary
- `frontend/app/view/dashboard/dashboard.scss`

### 8.2 Cross-Session AI Chat
Chat with context from all sessions.

**Files to create:**
- `pkg/service/termdash/chatservice.go` — Go service:
  - Aggregates transcript context from all active sessions
  - Calls Claude CLI with combined context
  - Returns response

**Files to modify:**
- `frontend/app/view/dashboard/dashboard.tsx` — Chat input/output UI

---

## Architecture Decisions

### Block Metadata Keys (termdash namespace)
```
termdash:type              # "claude" | "shell" (session type)
termdash:claudeSessionId   # UUID for Claude session tracking
termdash:resume            # boolean — resume Claude session on start
termdash:status            # "active" | "idle" | "needs-input" | "exited"
termdash:summary           # AI-generated session title
termdash:archived          # boolean
termdash:archivedAt        # timestamp
termdash:transcriptFile    # path to transcript data
```

### New Go Packages
```
pkg/termdash/              # Core TermDash logic
  statusdetector.go        # Terminal output analysis
  transcript.go            # Transcript recording
pkg/service/termdash/      # RPC services
  summaryservice.go
  taskservice.go
  archiveservice.go
  searchservice.go
  learningsservice.go
  chatservice.go
```

### New Frontend Packages
```
frontend/app/view/
  taskpanel/               # Task management panel
  archive/                 # Archive browser
  dashboard/               # Session dashboard
```
