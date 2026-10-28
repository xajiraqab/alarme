//go:build windows

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows/registry"
	_ "modernc.org/sqlite"
)

type ReminderInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Date     string `json:"date,omitempty"`
	Time     string `json:"time"`
	Weekdays []int  `json:"weekdays,omitempty"`
	Sound    string `json:"sound,omitempty"`
}

type Reminder struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Date     string `json:"date,omitempty"`
	Time     string `json:"time"`
	Weekdays []int  `json:"weekdays,omitempty"`
	Sound    string `json:"sound"`
}

const (
	reminderSoundNone           = "none"
	reminderSoundSoftChime      = "soft-chime"
	reminderSoundGlassBells     = "glass-bells"
	reminderSoundWarmTone       = "warm-tone"
	reminderSoundDewdrop        = "dewdrop"
	reminderSoundAurora         = "aurora"
	reminderSoundPulse          = "pulse"
	reminderSoundTwinkle        = "twinkle"
	reminderSoundMoonSun        = "moon-sun"
	reminderSoundRainGlass      = "rain-glass"
	reminderSoundHearth         = "hearth"
	reminderSoundOrbit          = "orbit"
	reminderSoundOdeToJoy       = "ode-to-joy"
	reminderSoundHungarianMarch = "hungarian-march"
	reminderSoundHappyBirthday  = "happy-birthday"
)

type storedReminder struct {
	Reminder
	LastFiredKey string `json:"lastFiredKey,omitempty"`
}

type ReminderService struct {
	mu        sync.Mutex
	path      string
	db        *sql.DB
	reminders []storedReminder
	pending   []Reminder
	onDue     func(Reminder)
	onDismiss func()
	cancel    context.CancelFunc
	done      chan struct{}
}

const startupRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const startupApprovedRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
const startupLaunchArgument = "--startup"

func startupCommandTargets(command, executable string) (bool, bool) {
	command = strings.TrimSpace(command)
	var commandPath string
	var args []string
	if strings.HasPrefix(command, `"`) {
		closingQuote := strings.Index(command[1:], `"`)
		if closingQuote < 0 {
			return false, false
		}
		commandPath = command[1 : closingQuote+1]
		args = strings.Fields(command[closingQuote+2:])
	} else {
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return false, false
		}
		commandPath = parts[0]
		args = parts[1:]
	}

	for _, arg := range args {
		if arg == startupLaunchArgument {
			return strings.EqualFold(filepath.Clean(commandPath), filepath.Clean(executable)), true
		}
	}
	return strings.EqualFold(filepath.Clean(commandPath), filepath.Clean(executable)), false
}

// StartupEnabled reports whether Alarme is registered to run for this user at sign-in.
func (s *ReminderService) StartupEnabled() (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, startupRegistryPath, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()

	command, _, err := key.GetStringValue("Alarme")
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	commandMatches, launchesHidden := startupCommandTargets(command, executable)
	if !commandMatches || !launchesHidden {
		return false, nil
	}

	approvedKey, err := registry.OpenKey(registry.CURRENT_USER, startupApprovedRegistryPath, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer approvedKey.Close()
	approval, _, err := approvedKey.GetBinaryValue("Alarme")
	if err == registry.ErrNotExist {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if len(approval) == 0 {
		return false, nil
	}
	// Task Manager marks disabled Run entries with bit 0 set in the first byte.
	return approval[0]&1 == 0, nil
}

// EnableStartup registers Alarme in the current user's Run key; no administrator access is needed.
func (s *ReminderService) EnableStartup() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, startupRegistryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue("Alarme", `"`+executable+`" `+startupLaunchArgument); err != nil {
		return err
	}

	approvedKey, _, err := registry.CreateKey(registry.CURRENT_USER, startupApprovedRegistryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer approvedKey.Close()
	return approvedKey.SetBinaryValue("Alarme", []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
}

func NewReminderService() (*ReminderService, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find user config directory: %w", err)
	}

	service := &ReminderService{
		path: filepath.Join(configDir, "alarme", "reminders.db"),
	}
	if err := os.MkdirAll(filepath.Dir(service.path), 0o700); err != nil {
		return nil, fmt.Errorf("create reminders directory: %w", err)
	}
	if err := service.open(); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *ReminderService) List() ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Reminder, len(s.reminders))
	for i, entry := range s.reminders {
		result[i] = cloneReminder(entry.Reminder)
	}
	return result, nil
}

func (s *ReminderService) PendingAlerts() ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Reminder, len(s.pending))
	for i, reminder := range s.pending {
		result[i] = reminder
		result[i].Weekdays = append([]int(nil), reminder.Weekdays...)
	}
	s.pending = nil
	return result, nil
}

func (s *ReminderService) AcknowledgeAlert(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, reminder := range s.pending {
		if reminder.ID == id {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			return nil
		}
	}
	return nil
}

func (s *ReminderService) DismissAlert() error {
	if s.onDismiss == nil {
		return errors.New("Reminder window is unavailable.")
	}
	s.onDismiss()
	return nil
}

func (s *ReminderService) Create(input ReminderInput) (Reminder, error) {
	reminder, err := normalizeReminder(input, time.Now())
	if err != nil {
		return Reminder{}, err
	}
	startupEnabled, err := s.StartupEnabled()
	if err != nil {
		return Reminder{}, fmt.Errorf("check Windows startup setting: %w", err)
	}
	if !startupEnabled {
		if err := s.EnableStartup(); err != nil {
			return Reminder{}, fmt.Errorf("enable Alarme at Windows sign-in: %w", err)
		}
	}
	reminder.ID, err = newReminderID()
	if err != nil {
		return Reminder{}, fmt.Errorf("create reminder ID: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	weekdays, err := json.Marshal(reminder.Weekdays)
	if err != nil {
		return Reminder{}, fmt.Errorf("encode reminder weekdays: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Reminder{}, fmt.Errorf("begin reminder save: %w", err)
	}
	_, err = tx.Exec(
		`INSERT INTO reminders (id, name, type, date, time, weekdays, last_fired_key)
		 VALUES (?, ?, ?, ?, ?, ?, '')`,
		reminder.ID, reminder.Name, reminder.Type, reminder.Date, reminder.Time, string(weekdays),
	)
	if err != nil {
		tx.Rollback()
		return Reminder{}, fmt.Errorf("save reminder: %w", err)
	}
	if err := saveReminderSound(tx, reminder.ID, reminder.Sound); err != nil {
		tx.Rollback()
		return Reminder{}, fmt.Errorf("save reminder sound: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Reminder{}, fmt.Errorf("commit reminder save: %w", err)
	}
	s.reminders = append(s.reminders, storedReminder{Reminder: cloneReminder(reminder)})
	return reminder, nil
}

func (s *ReminderService) Update(id string, input ReminderInput) (Reminder, error) {
	reminder, err := normalizeReminder(input, time.Now())
	if err != nil {
		return Reminder{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	index := -1
	for i, entry := range s.reminders {
		if entry.ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return Reminder{}, errors.New("reminder not found")
	}
	reminder.ID = id
	weekdays, err := json.Marshal(reminder.Weekdays)
	if err != nil {
		return Reminder{}, fmt.Errorf("encode reminder weekdays: %w", err)
	}
	previous := s.reminders[index]
	lastFiredKey := previous.LastFiredKey
	if previous.Type != reminder.Type || previous.Date != reminder.Date ||
		previous.Time != reminder.Time || !equalWeekdays(previous.Weekdays, reminder.Weekdays) {
		lastFiredKey = ""
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Reminder{}, fmt.Errorf("begin reminder update: %w", err)
	}
	result, err := tx.Exec(
		`UPDATE reminders
		 SET name = ?, type = ?, date = ?, time = ?, weekdays = ?, last_fired_key = ?
		 WHERE id = ?`,
		reminder.Name, reminder.Type, reminder.Date, reminder.Time, string(weekdays), lastFiredKey, id,
	)
	if err != nil {
		tx.Rollback()
		return Reminder{}, fmt.Errorf("update reminder: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return Reminder{}, fmt.Errorf("check updated reminder: %w", err)
	}
	if affected == 0 {
		tx.Rollback()
		return Reminder{}, errors.New("reminder not found")
	}
	if err := saveReminderSound(tx, id, reminder.Sound); err != nil {
		tx.Rollback()
		return Reminder{}, fmt.Errorf("update reminder sound: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Reminder{}, fmt.Errorf("commit reminder update: %w", err)
	}
	s.reminders[index] = storedReminder{Reminder: cloneReminder(reminder), LastFiredKey: lastFiredKey}
	return reminder, nil
}

func (s *ReminderService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reminder delete: %w", err)
	}
	result, err := tx.Exec(`DELETE FROM reminders WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("delete reminder: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("check deleted reminder: %w", err)
	}
	if affected == 0 {
		tx.Rollback()
		return errors.New("reminder not found")
	}
	if _, err := tx.Exec(`DELETE FROM reminder_sounds WHERE reminder_id = ?`, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("delete reminder sound: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reminder delete: %w", err)
	}
	for i, reminder := range s.reminders {
		if reminder.ID == id {
			s.reminders = append(s.reminders[:i], s.reminders[i+1:]...)
			break
		}
	}
	for i := 0; i < len(s.pending); {
		if s.pending[i].ID == id {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			continue
		}
		i++
	}
	return nil
}

func (s *ReminderService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	go s.runScheduler(runCtx, s.done)
	return nil
}

func (s *ReminderService) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
		<-s.done
		s.cancel = nil
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("close reminders database: %w", err)
		}
		s.db = nil
	}
	return nil
}

func (s *ReminderService) open() error {
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return fmt.Errorf("open reminders database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("connect to reminders database: %w", err)
	}
	s.db = db

	const schema = `CREATE TABLE IF NOT EXISTS reminders (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL CHECK (type IN ('date', 'weekly')),
		date TEXT NOT NULL DEFAULT '',
		time TEXT NOT NULL,
		weekdays TEXT NOT NULL DEFAULT '[]',
		last_fired_key TEXT NOT NULL DEFAULT ''
	)`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		s.db = nil
		return fmt.Errorf("create reminders table: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS reminder_sounds (
		reminder_id TEXT PRIMARY KEY,
		sound_id TEXT NOT NULL
	)`); err != nil {
		db.Close()
		s.db = nil
		return fmt.Errorf("create reminder sounds table: %w", err)
	}
	if err := s.migrateLegacyJSON(); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	s.reminders, err = s.loadStoredReminders()
	if err != nil {
		db.Close()
		s.db = nil
		return err
	}
	return nil
}

func (s *ReminderService) migrateLegacyJSON() error {
	legacyPath := filepath.Join(filepath.Dir(s.path), "reminders.json")
	data, err := os.ReadFile(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy reminders: %w", err)
	}

	var file struct {
		Reminders []storedReminder `json:"reminders"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse legacy reminders: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reminders migration: %w", err)
	}
	for _, entry := range file.Reminders {
		weekdays, err := json.Marshal(entry.Weekdays)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("encode legacy reminder weekdays: %w", err)
		}
		_, err = tx.Exec(
			`INSERT OR IGNORE INTO reminders (id, name, type, date, time, weekdays, last_fired_key)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			entry.ID, entry.Name, entry.Type, entry.Date, entry.Time, string(weekdays), entry.LastFiredKey,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate legacy reminder %q: %w", entry.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reminders migration: %w", err)
	}

	if err := os.Rename(legacyPath, legacyPath+".migrated"); err != nil {
		return fmt.Errorf("mark legacy reminders as migrated: %w", err)
	}
	return nil
}

func (s *ReminderService) runScheduler(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	s.checkDueReminders(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.checkDueReminders(now)
		}
	}
}

func (s *ReminderService) checkDueReminders(now time.Time) {
	key := now.Format("2006-01-02 15:04")

	s.mu.Lock()
	due := make([]storedReminder, 0)
	for _, entry := range s.reminders {
		if entry.LastFiredKey != key && entry.matches(now) {
			due = append(due, storedReminder{Reminder: cloneReminder(entry.Reminder), LastFiredKey: entry.LastFiredKey})
		}
	}
	s.mu.Unlock()

	for _, entry := range due {
		if !s.markFired(entry.ID, key) {
			continue
		}
		if s.onDue != nil {
			s.onDue(entry.Reminder)
		}
	}
}

func (r storedReminder) matches(now time.Time) bool {
	if r.Time != now.Format("15:04") {
		return false
	}
	switch r.Type {
	case "date":
		return r.Date == now.Format("2006-01-02")
	case "weekly":
		for _, weekday := range r.Weekdays {
			if weekday == int(now.Weekday()) {
				return true
			}
		}
	}
	return false
}

func (s *ReminderService) markFired(id, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := -1
	for i, entry := range s.reminders {
		if entry.ID == id {
			if entry.LastFiredKey == key {
				return false
			}
			index = i
			break
		}
	}
	if index == -1 {
		return false
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("begin reminder delivery: %v", err)
		return false
	}
	var result sql.Result
	if s.reminders[index].Type == "date" {
		result, err = tx.Exec(`DELETE FROM reminders WHERE id = ?`, id)
		if err == nil {
			_, err = tx.Exec(`DELETE FROM reminder_sounds WHERE reminder_id = ?`, id)
		}
	} else {
		result, err = tx.Exec(`UPDATE reminders SET last_fired_key = ? WHERE id = ? AND last_fired_key != ?`, key, id, key)
	}
	if err != nil {
		tx.Rollback()
		log.Printf("complete reminder delivery: %v", err)
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		log.Printf("check completed reminder update: %v", err)
		return false
	}
	if affected == 0 {
		tx.Rollback()
		return false
	}

	if err := tx.Commit(); err != nil {
		log.Printf("commit reminder delivery: %v", err)
		return false
	}
	reminder := cloneReminder(s.reminders[index].Reminder)
	s.pending = append(s.pending, reminder)
	if reminder.Type == "date" {
		s.reminders = append(s.reminders[:index], s.reminders[index+1:]...)
	} else {
		s.reminders[index].LastFiredKey = key
	}
	return true
}

func (s *ReminderService) loadStoredReminders() ([]storedReminder, error) {
	rows, err := s.db.Query(`SELECT r.id, r.name, r.type, r.date, r.time, r.weekdays,
		COALESCE(rs.sound_id, 'none'), r.last_fired_key
		FROM reminders r LEFT JOIN reminder_sounds rs ON rs.reminder_id = r.id
		ORDER BY r.rowid`)
	if err != nil {
		return nil, fmt.Errorf("load reminders: %w", err)
	}
	defer rows.Close()

	result := make([]storedReminder, 0)
	for rows.Next() {
		var entry storedReminder
		var weekdays string
		if err := rows.Scan(
			&entry.ID, &entry.Name, &entry.Type, &entry.Date, &entry.Time, &weekdays, &entry.Sound, &entry.LastFiredKey,
		); err != nil {
			return nil, fmt.Errorf("read reminder: %w", err)
		}
		if err := json.Unmarshal([]byte(weekdays), &entry.Weekdays); err != nil {
			return nil, fmt.Errorf("decode reminder weekdays: %w", err)
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders: %w", err)
	}
	return result, nil
}

func saveReminderSound(tx *sql.Tx, reminderID, soundID string) error {
	if soundID == reminderSoundNone {
		_, err := tx.Exec(`DELETE FROM reminder_sounds WHERE reminder_id = ?`, reminderID)
		return err
	}
	_, err := tx.Exec(
		`INSERT INTO reminder_sounds (reminder_id, sound_id) VALUES (?, ?)
		 ON CONFLICT(reminder_id) DO UPDATE SET sound_id = excluded.sound_id`,
		reminderID, soundID,
	)
	return err
}

func cloneReminder(reminder Reminder) Reminder {
	reminder.Weekdays = append([]int(nil), reminder.Weekdays...)
	return reminder
}

func normalizeReminder(input ReminderInput, now time.Time) (Reminder, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "-"
	}
	if len([]rune(name)) > 120 {
		return Reminder{}, errors.New("Name must be 120 characters or fewer.")
	}
	if _, err := time.Parse("15:04", input.Time); err != nil {
		return Reminder{}, errors.New("Time must use the HH:MM format.")
	}
	sound := input.Sound
	if sound == "" {
		sound = reminderSoundNone
	}
	switch sound {
	case reminderSoundNone, reminderSoundSoftChime, reminderSoundGlassBells, reminderSoundWarmTone,
		reminderSoundDewdrop, reminderSoundAurora, reminderSoundPulse, reminderSoundTwinkle,
		reminderSoundMoonSun, reminderSoundRainGlass, reminderSoundHearth, reminderSoundOrbit,
		reminderSoundOdeToJoy, reminderSoundHungarianMarch, reminderSoundHappyBirthday:
	default:
		return Reminder{}, errors.New("Choose a valid notification sound.")
	}

	reminder := Reminder{
		Name:  name,
		Type:  input.Type,
		Time:  input.Time,
		Sound: sound,
	}
	switch input.Type {
	case "date":
		date, err := time.ParseInLocation("2006-01-02", input.Date, now.Location())
		if err != nil {
			return Reminder{}, errors.New("Date must use the YYYY-MM-DD format.")
		}
		scheduled := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, now.Location())
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if scheduled.Before(today) {
			return Reminder{}, errors.New("A past date cannot be selected.")
		}
		if scheduled.Year() == now.Year() && scheduled.YearDay() == now.YearDay() {
			clock, _ := time.Parse("15:04", input.Time)
			scheduled = time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), 0, 0, now.Location())
			if scheduled.Before(now.Truncate(time.Minute)) {
				return Reminder{}, errors.New("This time has already passed today.")
			}
		}
		reminder.Date = input.Date
	case "weekly":
		if len(input.Weekdays) == 0 {
			return Reminder{}, errors.New("Select at least one weekday.")
		}
		seen := make(map[int]struct{}, len(input.Weekdays))
		for _, day := range input.Weekdays {
			if day < 0 || day > 6 {
				return Reminder{}, errors.New("Invalid weekday.")
			}
			seen[day] = struct{}{}
		}
		for day := range seen {
			reminder.Weekdays = append(reminder.Weekdays, day)
		}
		sort.Ints(reminder.Weekdays)
	default:
		return Reminder{}, errors.New("Choose a specific date or selected weekdays.")
	}
	return reminder, nil
}

func equalWeekdays(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func newReminderID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
