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
	Entries          []Entry          `json:"entries"`
	Current          *Entry           `json:"current,omitempty"`
	CustomCategories map[string]string `json:"custom_categories,omitempty"`
}

// Built-in categories
var builtinCategories = []string{
	"meeting", "code", "review", "plan", "admin", "side", "break", "other",
}

var builtinEmoji = map[string]string{
	"meeting": "📅", "code": "💻", "review": "👀", "plan": "🎯",
	"admin": "📋", "side": "🚀", "break": "☕", "other": "❓",
}

// Emoji pool for auto-assigning to new custom categories
var emojiPool = []string{
	"🔧", "📝", "🎮", "📚", "🏋️", "🎵", "🎨", "🔬",
	"🧪", "📊", "🔔", "💬", "🤖", "🏥", "🏠", "🚗",
	"🌍", "⚡", "🔥", "💰", "🎯", "🧠", "⚙️", "📱",
	"🖥️", "🔒", "🌐", "📦", "🎭", "🧩", "📈", "🌻",
}

var emojiPoolUsed int

// ─── Storage ─────────────────────────────────────────────────────────────────

func dataFilePath() string {
	local := filepath.Join(".", ".tt.json")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return local
	}
	return filepath.Join(home, ".tt.json")
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
	if d.CustomCategories == nil {
		d.CustomCategories = make(map[string]string)
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

// ─── Category Helpers ───────────────────────────────────────────────────────

func getEmoji(d *Data, category string) string {
	if e, ok := builtinEmoji[category]; ok {
		return e
	}
	if e, ok := d.CustomCategories[category]; ok {
		return e
	}
	return "📌"
}

func getAllCategories(d *Data) []string {
	all := make([]string, 0, len(builtinCategories)+len(d.CustomCategories))
	all = append(all, builtinCategories...)
	for c := range d.CustomCategories {
		all = append(all, c)
	}
	return all
}

func resolveOrCreateCategory(d *Data, category string) string {
	if d.CustomCategories == nil {
		d.CustomCategories = make(map[string]string)
	}

	// Already known (builtin or custom)?
	for _, c := range builtinCategories {
		if c == category {
			return category
		}
	}
	if _, ok := d.CustomCategories[category]; ok {
		return category
	}

	// New category — assign emoji from pool
	used := make(map[string]bool)
	for _, e := range builtinEmoji {
		used[e] = true
	}
	for _, e := range d.CustomCategories {
		used[e] = true
	}

	var emoji string
	for _, e := range emojiPool {
		if !used[e] {
			emoji = e
			break
		}
	}
	if emoji == "" {
		// Pool exhausted — use first char of category as fallback
		emoji = "📌"
	}

	d.CustomCategories[category] = emoji
	fmt.Printf("📌 New category created: %s %s\n", emoji, category)
	return category
}

// ─── Commands ────────────────────────────────────────────────────────────────

func cmdStart(category, note string) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		fmt.Println("Usage: tt start <category> [note]")
		fmt.Println("Run `tt categories` to see available categories.")
		os.Exit(1)
	}

	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	// Resolve or create the category
	resolveOrCreateCategory(d, category)

	if d.Current != nil {
		elapsed := d.Current.Duration()
		fmt.Printf("⚠️  Already tracking: %s %s for %s\n", getEmoji(d, d.Current.Category), d.Current.Category, fmtDuration(elapsed))
		fmt.Println("   Run `tt stop` first, or `tt stop` + `tt start` to switch.")
		// Don't save the custom category in this case — but we already did above
		// Actually let's still save it so it's registered
		if err := saveData(d); err != nil {
			die("save: %v", err)
		}
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
	fmt.Printf("%s %s started at %s", getEmoji(d, category), category, now)
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
		getEmoji(d, entry.Category),
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
		getEmoji(d, e.Category),
		e.Category,
		time.Unix(e.Start, 0).Format("15:04"),
		fmtDuration(elapsed),
	)

	if elapsed >= 2*time.Hour {
		fmt.Printf("⚠️  You've been at this for %s. Time for a break?\n", fmtDuration(elapsed))
	}

	if e.Note != "" {
		fmt.Printf("   📝 %s\n", e.Note)
	}

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
		printJSONReport(filtered, d, period)
		return
	}

	printHumanReport(filtered, d, period)
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

	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Start > allEntries[j].Start
	})

	if n <= 0 || n > len(allEntries) {
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
		fmt.Printf("  %s %-10s %s%s  %5s", getEmoji(d, e.Category), e.Category, start.Format("Mon 01/02 15:04"), end, fmtDuration(dur))
		if e.Note != "" {
			fmt.Printf("  — %s", e.Note)
		}
		fmt.Println()
	}
}

func cmdCategories() {
	d, err := loadData()
	if err != nil {
		die("load data: %v", err)
	}

	fmt.Println("📦 Available categories:\n")
	fmt.Println("  Built-in:")
	for _, c := range builtinCategories {
		fmt.Printf("    %s %s\n", builtinEmoji[c], c)
	}

	if len(d.CustomCategories) > 0 {
		fmt.Println("\n  Custom:")
		// Sort custom categories for stable output
		var customs []string
		for c := range d.CustomCategories {
			customs = append(customs, c)
		}
		sort.Strings(customs)
		for _, c := range customs {
			fmt.Printf("    %s %s\n", d.CustomCategories[c], c)
		}
	}

	fmt.Println("\n  💡 Use any new name to auto-create a custom category.")
}

// ─── Report Formatters ──────────────────────────────────────────────────────

type catStat struct {
	name string
	min  float64
	pct  float64
}

func printHumanReport(entries []Entry, d *Data, period string) {
	catTimes := make(map[string]float64)
	for _, e := range entries {
		catTimes[e.Category] += e.DurationMin()
	}

	var totalMin float64
	for _, m := range catTimes {
		totalMin += m
	}

	var stats []catStat
	for name, minutes := range catTimes {
		stats = append(stats, catStat{name, minutes, (minutes / totalMin) * 100})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].min > stats[j].min })

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
		fmt.Printf("  %s %-10s %s  %2dh%02dm  (%5.1f%%)\n",
			getEmoji(d, s.name), s.name, bar, h, m, s.pct)
	}

	fmt.Printf("  %s\n", strings.Repeat("─", 60))

	th := int(totalMin / 60)
	tm := int(totalMin) % 60
	fmt.Printf("  📊 %-10s            %2dh%02dm\n", "TOTAL", th, tm)

	fmt.Println()
	printInsights(entries, d, stats, totalMin)
}

func printInsights(entries []Entry, d *Data, stats []catStat, totalMin float64) {
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

	deepWorkMin := 0.0
	for _, s := range stats {
		if s.name == "code" || s.name == "review" || s.name == "plan" {
			deepWorkMin += s.min
		}
	}
	deepPct := (deepWorkMin / totalMin) * 100
	fmt.Printf("  💡 Deep work (code+review+plan): %.0f%%\n", deepPct)

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

func printJSONReport(entries []Entry, d *Data, period string) {
	type CatTotal struct {
		Category string  `json:"category"`
		Emoji    string  `json:"emoji"`
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
		cats = append(cats, CatTotal{name, getEmoji(d, name), minutes, (minutes / totalMin) * 100})
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
  tt start <category> [note]    Start tracking time (any new name auto-creates)
  tt stop [note]                Stop current session
  tt status                     Show current tracking info
  tt report [period]            Time report: today, week (default), month, all
  tt report --json [period]     Machine-readable JSON output
  tt log [N]                    Show last N entries (default: 10)
  tt categories                 List built-in + custom categories

Categories:
  📅 meeting    💻 code       👀 review     🎯 plan
  📋 admin      🚀 side      ☕ break      ❓ other

  💡 Use any name to auto-create a custom category with emoji.

Data is stored in ~/.tt.json by default (or .tt.json in current directory if it exists).`)
}

func main() {
	if len(os.Args) < 2 {
		cmdCategories()
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
