# tt — minimal CLI time tracker

Know where your hours actually go. No server, no account, no BS.

## Install

Download from [Releases](https://github.com/jiahaowu/tt/releases), or build from source:

```bash
go build -o tt .
```

## Quick Start

```bash
# Make it executable
chmod +x tt

# Start tracking
tt start meeting "team sync"
tt start code "implementing feature X"

# Check what you're doing
tt status

# Stop and save
tt stop "finished review"

# See your week
tt report
tt report today
tt report --json

# Browse history
tt log 20
```

## Categories

| Icon | Category | Use for |
|------|----------|---------|
| 📅 | meeting | Any meeting, 1:1, standup |
| 💻 | code | Writing code, debugging |
| 👀 | review | Code review, PR review |
| 🎯 | plan | Planning, design, strategy |
| 📋 | admin | Email, HR, paperwork |
| 🚀 | side | Side projects, learning |
| ☕ | break | Breaks, walks, meals |
| ❓ | other | Everything else |

## Report Sample

```
📊 Weekly Report
   Mon Apr 21 → Mon Apr 27

  📅 meeting  ██████████████████  12h 30m  ( 35.2%)
  💻 code     ██████████████      10h 15m  ( 28.9%)
  👀 review   ██████               4h 00m  ( 11.3%)
  📋 admin    █████                3h 00m  (  8.5%)
  ☕ break    ███                   2h 15m  (  6.3%)
  🎯 plan     ██                    1h 30m  (  4.2%)
  🚀 side     █                     0h 30m  (  1.4%)
  ❓ other    ███                   2h 30m  (  7.0%)
            ─────────────────────────────────
  📊 TOTAL                         35h 30m

  🚨 Meetings consume 35% of your time — consider declining some.
  💡 Deep work (code+review+plan): 44%
  🔴 No side project time tracked this period.
```

## Data

All data stored in `.tt.json` in the current directory. No server, no cloud. Copy the file to back up or transfer.

## License

MIT
