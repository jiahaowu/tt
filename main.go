package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ─── Data Model ──────────────────────────────────────────────────────────────

type Entry struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Start    int64  `json:"start"`
	End      int64  `json:"end,omitempty"`
	Note     string `json:"note,omitempty"`
}

func (e Entry) Duration() time.Duration {
	end := e.End
	if end == 0 {
		end = time.Now().Unix()
	}
	return time.Duration(end-e.Start) * time.Second
}

func (e Entry) DurationMin() float64 {
	return e.Duration().Minutes()
}

type Data struct {
	Entries []Entry `json:"entries"`
	Current *Entry  `json:"current,omitempty"`
}

var categories = []string{
	"meeting", "code", "review", "plan", "admin", "side", "break", "other",
}

var categoryEmoji = map[string]string{
	"meeting": "📅", "code": "💻", "review": "👀", "plan": "🎯",
	"admin": "📋", "side": "🚀", "break": "☕", "other": "❓",
}

var validCats map[string]bool

func init() {
	validCats = make(map[string]bool, len(categories))
	for _, c := range categories {
		validCats[c] = true
	}
}

// ─── Storage ─────────────────────────────────────────────────────────────────

func dataFilePath() string {
	// First check current directory
	local := filepath.Join(".", ".tt.json")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	// Default: current directory (will create on first save)
	return local
}

func loadData() (*Data, error) {
	path := dataFilePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Data{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &d, nil
}

func saveData(d *Data) error {
	path := dataFilePath()
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

// ─── Commands ────────────────────────────────────────────────────────────────

func cmdStart(category, note string) {
	category = strings.ToLower(strings.TrimSpace(category))
	if !validCats[category] {
		fmt.Printf("❌ Unknown category \"%s\"\n\nValid categories:\n", category)
		cmdCategories()
		os.Exit(1)
	}

	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	if d.Current != nil {
		elapsed := d.Current.Duration()
		fmt.Printf("⚠️  Already tracking: %s %s for %s\n", categoryEmoji[d.Current.Category], d.Current.Category, fmtDuration(elapsed))
		fmt.Println("   Run `tt stop` first, or `tt stop` + `tt start` to switch.")
		os.Exit(1)
	}

	d.Current = &Entry{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Category: category,
		Start:    time.Now().Unix(),
		Note:     note,
	}

	if err := saveData(d); err != nil {
		die("save: %v", err)
	}

	now := time.Now().Format("15:04")
	fmt.Printf("%s %s started at %s", categoryEmoji[category], category, now)
	if note != "" {
		fmt.Printf(" — %s", note)
	}
	fmt.Println()
}

func cmdStop(note string) {
	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	if d.Current == nil {
		fmt.Println("☕ Nothing is being tracked right now.")
		return
	}

	if note != "" {
		d.Current.Note = note
	}

	d.Current.End = time.Now().Unix()
	d.Entries = append(d.Entries, *d.Current)
	d.Current = nil

	if err := saveData(d); err != nil {
		die("save: %v", err)
	}

	entry := d.Entries[len(d.Entries)-1]
	fmt.Printf("✅ %s %s — %s (%s)\n",
		categoryEmoji[entry.Category],
		entry.Category,
		fmtDuration(time.Duration(entry.End-entry.Start) * time.Second),
		entry.Note,
	)
}

func cmdStatus() {
	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	if d.Current == nil {
		fmt.Println("☕ Not tracking anything right now.")
		fmt.Println("   Use `tt start <category>` to begin.")
		return
	}

	e := d.Current
	elapsed := e.Duration()
	fmt.Printf("%s %s — started at %s, elapsed: %s\n",
		categoryEmoji[e.Category],
		e.Category,
		time.Unix(e.Start, 0).Format("15:04"),
		fmtDuration(elapsed),
	)

	// Warning if sitting too long
	if elapsed >= 2*time.Hour {
		fmt.Printf("⚠️  You've been at this for %s. Time for a break?\n", fmtDuration(elapsed))
	}

	if e.Note != "" {
		fmt.Printf("   📝 %s\n", e.Note)
	}

	// Today's summary
	todayEntries := entriesSince(d.Entries, todayStart())
	var todayTotal time.Duration
	for _, entry := range todayEntries {
		todayTotal += entry.Duration()
	}
	if d.Current != nil {
		todayTotal += d.Current.Duration()
	}
	fmt.Printf("   📊 Today total (including current): %s\n", fmtDuration(todayTotal))
}

func cmdReport(period string, jsonOutput bool) {
	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	// Add current session to entries for calculation
	allEntries := make([]Entry, len(d.Entries))
	copy(allEntries, d.Entries)

	var cutoff time.Time
	switch period {
	case "today":
		cutoff = todayStart()
	case "week":
		cutoff = weekStart()
	case "month":
		cutoff = monthStart()
	case "all":
		cutoff = time.Time{}
	default:
		period = "week"
		cutoff = weekStart()
	}

	if d.Current != nil {
		allEntries = append(allEntries, *d.Current)
	}

	filtered := filterEntries(allEntries, cutoff)

	if len(filtered) == 0 {
		fmt.Printf("📭 No entries for %s\n", periodTitle(period))
		return
	}

	if jsonOutput {
		printJSONReport(filtered, period)
		return
	}

	printHumanReport(filtered, period)
}

func cmdLog(n int) {
	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	allEntries := make([]Entry, len(d.Entries))
	copy(allEntries, d.Entries)
	if d.Current != nil {
		allEntries = append(allEntries, *d.Current)
	}

	// Sort by start time, newest first
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Start > allEntries[j].Start
	})

	if n <= 0 || n > len(allEntries) {
		n = len(allEntries)
	}
	if n > len(allEntries) {
		n = len(allEntries)
	}

	fmt.Printf("📋 Last %d entries:\n\n", n)
	for i := 0; i < n; i++ {
		e := allEntries[i]
		dur := e.Duration()
		start := time.Unix(e.Start, 0)
		end := ""
		if e.End != 0 {
			end = " → " + time.Unix(e.End, 0).Format("15:04")
		} else {
			end = " → now"
		}
		fmt.Printf("  %s %-8s %s%s  %5s", categoryEmoji[e.Category], e.Category, start.Format("Mon 01/02 15:04"), end, fmtDuration(dur))
		if e.Note != "" {
			fmt.Printf("  — %s", e.Note)
		}
		fmt.Println()
	}
}

func cmdCategories() {
	fmt.Println("📦 Available categories:")
	for _, c := range categories {
		fmt.Printf("  %s %s\n", categoryEmoji[c], c)
	}
}

// ─── Report Formatters ──────────────────────────────────────────────────────

type catStat struct {
	name string
	min  float64
	pct  float64
}

func printHumanReport(entries []Entry, period string) {
	// Group by category
	catTimes := make(map[string]float64)
	for _, e := range entries {
		catTimes[e.Category] += e.DurationMin()
	}

	var totalMin float64
	for _, m := range catTimes {
		totalMin += m
	}

	// Sort categories by time (descending)
	var stats []catStat
	for name, minutes := range catTimes {
		stats = append(stats, catStat{name, minutes, (minutes / totalMin) * 100})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].min > stats[j].min })

	// Header
	fmt.Printf("\n📊 %s Report\n", periodTitle(period))
	if period == "week" {
		fmt.Printf("   %s → %s\n\n", weekStart().Format("Mon Jan 02"), time.Now().Format("Mon Jan 02"))
	} else if period == "today" {
		fmt.Printf("   %s\n\n", time.Now().Format("Monday, Jan 02"))
	} else if period == "month" {
		fmt.Printf("   %s\n\n", time.Now().Format("January 2006"))
	} else {
		fmt.Println()
	}

	barWidth := 20
	for _, s := range stats {
		filled := int(s.pct / 100 * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		h := int(s.min / 60)
		m := int(s.min) % 60
		fmt.Printf("  %s %-8s %s  %2dh%02dm  (%5.1f%%)\n",
			categoryEmoji[s.name], s.name, bar, h, m, s.pct)
	}

	// Separator
	fmt.Printf("  %s\n", strings.Repeat("─", 58))

	// Total
	th := int(totalMin / 60)
	tm := int(totalMin) % 60
	fmt.Printf("  %s %-8s            %2dh%02dm\n", "📊", "TOTAL", th, tm)

	// Insights
	fmt.Println()
	printInsights(entries, stats, totalMin)
}

func printInsights(entries []Entry, stats []catStat, totalMin float64) {
	// Meeting ratio
	meetingMin := 0.0
	for _, s := range stats {
		if s.name == "meeting" {
			meetingMin = s.min
			break
		}
	}
	meetingPct := (meetingMin / totalMin) * 100
	switch {
	case meetingPct > 50:
		fmt.Printf("  🚨 Meetings consume %.0f%% of your time — consider declining some.\n", meetingPct)
	case meetingPct > 35:
		fmt.Printf("  ⚠️  Meetings are %.0f%% of your time — that's high.\n", meetingPct)
	case meetingPct > 0:
		fmt.Printf("  ✅ Meetings at %.0f%% — reasonable.\n", meetingPct)
	}

	// Deep work (code + review)
	deepWorkMin := 0.0
	for _, s := range stats {
		if s.name == "code" || s.name == "review" || s.name == "plan" {
			deepWorkMin += s.min
		}
	}
	deepPct := (deepWorkMin / totalMin) * 100
	fmt.Printf("  💡 Deep work (code+review+plan): %.0f%%\n", deepPct)

	// Side project time
	sideMin := 0.0
	for _, s := range stats {
		if s.name == "side" {
			sideMin = s.min
			break
		}
	}
	if sideMin > 0 {
		fmt.Printf("  🚀 Side project: %s this period\n", fmtMinutes(sideMin))
	} else {
		fmt.Println("  🔴 No side project time tracked this period.")
	}

	// Break time
	breakMin := 0.0
	for _, s := range stats {
		if s.name == "break" {
			breakMin = s.min
			break
		}
	}
	if breakMin < 30 && totalMin > 240 {
		fmt.Println("  ☕ Break time is very low — take care of yourself.")
	}

	// Estimated wasted time (admin + other)
	wastedMin := 0.0
	for _, s := range stats {
		if s.name == "admin" || s.name == "other" {
			wastedMin += s.min
		}
	}
	if wastedMin > 0 {
		fmt.Printf("  🗂️  Admin+other: %s — review if any can be eliminated\n", fmtMinutes(wastedMin))
	}

	// Per-day breakdown
	dayTotals := make(map[string]float64)
	for _, e := range entries {
		day := time.Unix(e.Start, 0).Format("Mon 01/02")
		dayTotals[day] += e.DurationMin()
	}

	var days []string
	for day := range dayTotals {
		days = append(days, day)
	}
	sort.Strings(days)

	if len(days) > 1 {
		fmt.Println()
		fmt.Println("  📅 Daily breakdown:")
		maxMin := 0.0
		for _, day := range days {
			if dayTotals[day] > maxMin {
				maxMin = dayTotals[day]
			}
		}
		dayBarWidth := 15
		for _, day := range days {
			min := dayTotals[day]
			filled := int((min / maxMin) * float64(dayBarWidth))
			bar := strings.Repeat("█", filled) + strings.Repeat("░", dayBarWidth-filled)
			fmt.Printf("     %s  %s  %s\n", day, bar, fmtMinutes(min))
		}
	}
}

func printJSONReport(entries []Entry, period string) {
	type CatTotal struct {
		Category string  `json:"category"`
		Minutes  float64 `json:"minutes"`
		Percent  float64 `json:"percent"`
	}
	type Report struct {
		Period    string     `json:"period"`
		TotalMin  float64    `json:"total_minutes"`
		Categories []CatTotal `json:"categories"`
		Entries   []Entry    `json:"entries"`
	}

	catTimes := make(map[string]float64)
	var totalMin float64
	for _, e := range entries {
		catTimes[e.Category] += e.DurationMin()
		totalMin += e.DurationMin()
	}

	var cats []CatTotal
	for name, minutes := range catTimes {
		cats = append(cats, CatTotal{name, minutes, (minutes / totalMin) * 100})
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Minutes > cats[j].Minutes })

	report := Report{Period: period, TotalMin: totalMin, Categories: cats, Entries: entries}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(report)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func todayStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func weekStart() time.Time {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
}

func monthStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func entriesSince(entries []Entry, cutoff time.Time) []Entry {
	var result []Entry
	for _, e := range entries {
		if time.Unix(e.Start, 0).After(cutoff) || time.Unix(e.Start, 0).Equal(cutoff) {
			result = append(result, e)
		}
	}
	return result
}

func filterEntries(entries []Entry, cutoff time.Time) []Entry {
	if cutoff.IsZero() {
		return entries
	}
	return entriesSince(entries, cutoff)
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func fmtMinutes(min float64) string {
	h := int(min / 60)
	m := int(min) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func periodTitle(p string) string {
	switch p {
	case "today":
		return "Today"
	case "week":
		return "Weekly"
	case "month":
		return "Monthly"
	case "all":
		return "All Time"
	default:
		return strings.Title(p)
	}
}

func die(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
	os.Exit(1)
}

// ─── Main ───────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Println(`tt — minimal CLI time tracker

Usage:
  tt start <category> [note]    Start tracking time
  tt stop [note]                Stop current session
  tt status                     Show current tracking info
  tt report [period]            Time report: today, week (default), month, all
  tt report --json [period]     Machine-readable JSON output
  tt log [N]                    Show last N entries (default: 10)
  tt categories                 List available categories

Categories:
  📅 meeting    💻 code       👀 review     🎯 plan
  📋 admin      🚀 side      ☕ break      ❓ other

Data is stored in .tt.json in the current directory.`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "start":
		if len(os.Args) < 3 {
			fmt.Println("Usage: tt start <category> [note]")
			fmt.Println("Run `tt categories` to see available categories.")
			os.Exit(1)
		}
		category := os.Args[2]
		note := strings.Join(os.Args[3:], " ")
		cmdStart(category, note)

	case "stop":
		note := strings.Join(os.Args[2:], " ")
		cmdStop(note)

	case "status", "s":
		cmdStatus()

	case "report", "r":
		period := "week"
		jsonOutput := false
		args := os.Args[2:]
		for _, a := range args {
			switch a {
			case "--json", "-j":
				jsonOutput = true
			case "today", "week", "month", "all":
				period = a
			}
		}
		cmdReport(period, jsonOutput)

	case "log", "l":
		n := 10
		if len(os.Args) >= 3 {
			if parsed, err := strconv.Atoi(os.Args[2]); err == nil {
				n = parsed
			}
		}
		cmdLog(n)

	case "categories", "cat", "c":
		cmdCategories()

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("❌ Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}
