//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestNormalizeReminder(t *testing.T) {
	now := time.Date(2026, time.June, 10, 10, 0, 0, 0, time.Local)
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")

	got, err := normalizeReminder(ReminderInput{
		Name: "  წამალი  ",
		Type: "date",
		Date: tomorrow,
		Time: "09:30",
	}, now)
	if err != nil {
		t.Fatalf("normalize date reminder: %v", err)
	}
	if got.Name != "წამალი" || got.Date != tomorrow || got.Time != "09:30" {
		t.Fatalf("unexpected normalized reminder: %#v", got)
	}

	got, err = normalizeReminder(ReminderInput{
		Type: "date",
		Date: tomorrow,
		Time: "10:00",
	}, now)
	if err != nil {
		t.Fatalf("normalize unnamed reminder: %v", err)
	}
	if got.Name != "-" {
		t.Fatalf("unnamed reminder got name %q, want %q", got.Name, "-")
	}

	got, err = normalizeReminder(ReminderInput{
		Name:     "ყოველკვირა",
		Type:     "weekly",
		Time:     "08:00",
		Weekdays: []int{1, 3, 1},
	}, now)
	if err != nil {
		t.Fatalf("normalize weekly reminder: %v", err)
	}
	if !reflect.DeepEqual(got.Weekdays, []int{1, 3}) {
		t.Fatalf("weekdays = %v, want [1 3]", got.Weekdays)
	}
}

func TestNormalizeReminderRejectsInvalidSchedule(t *testing.T) {
	now := time.Date(2026, time.June, 10, 10, 0, 0, 0, time.Local)
	tests := []ReminderInput{
		{Name: "წარსული", Type: "date", Date: "2026-06-09", Time: "11:00"},
		{Name: "დღეს უკვე გასული", Type: "date", Date: "2026-06-10", Time: "09:00"},
		{Name: "დღეების გარეშე", Type: "weekly", Time: "09:00"},
		{Name: "არასწორი დღე", Type: "weekly", Time: "09:00", Weekdays: []int{7}},
	}

	for _, input := range tests {
		if _, err := normalizeReminder(input, now); err == nil {
			t.Errorf("normalizeReminder(%#v) succeeded, want validation error", input)
		}
	}
}

func TestReminderMatchesSchedule(t *testing.T) {
	now := time.Date(2026, time.June, 10, 9, 15, 0, 0, time.Local)
	dateReminder := storedReminder{Reminder: Reminder{
		Type: "date",
		Date: "2026-06-10",
		Time: "09:15",
	}}
	if !dateReminder.matches(now) {
		t.Fatal("date reminder did not match its scheduled minute")
	}
	dateReminder.Time = "09:16"
	if dateReminder.matches(now) {
		t.Fatal("date reminder matched a different time")
	}

	weeklyReminder := storedReminder{Reminder: Reminder{
		Type:     "weekly",
		Time:     "09:15",
		Weekdays: []int{int(now.Weekday())},
	}}
	if !weeklyReminder.matches(now) {
		t.Fatal("weekly reminder did not match its selected weekday")
	}
}

func TestPendingAlertsAreDrained(t *testing.T) {
	service := &ReminderService{
		pending: []Reminder{{
			ID:       "reminder-id",
			Name:     "შეხსენება",
			Type:     "weekly",
			Time:     "09:00",
			Weekdays: []int{1, 3},
		}},
	}

	alerts, err := service.PendingAlerts()
	if err != nil {
		t.Fatalf("get pending alerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].ID != "reminder-id" {
		t.Fatalf("unexpected pending alerts: %#v", alerts)
	}

	alerts[0].Weekdays[0] = 6
	next, err := service.PendingAlerts()
	if err != nil {
		t.Fatalf("get pending alerts again: %v", err)
	}
	if len(next) != 0 {
		t.Fatalf("pending alerts were not drained: %#v", next)
	}
	if service.pending != nil {
		t.Fatal("returned alert shares service state")
	}
}

func TestDismissAlertCallsWindowHandler(t *testing.T) {
	dismissed := false
	service := &ReminderService{
		onDismiss: func() {
			dismissed = true
		},
	}

	if err := service.DismissAlert(); err != nil {
		t.Fatalf("dismiss alert: %v", err)
	}
	if !dismissed {
		t.Fatal("dismiss handler was not called")
	}
}

func TestReminderPersistenceAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "reminders.db")
	service := &ReminderService{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := service.open(); err != nil {
		t.Fatalf("open reminders database: %v", err)
	}
	date := time.Now().AddDate(0, 0, 2).Format("2006-01-02")

	first, err := service.Create(ReminderInput{
		Name:     "პირველი",
		Type:     "weekly",
		Time:     "10:00",
		Weekdays: []int{int(time.Now().Weekday())},
	})
	if err != nil {
		t.Fatalf("create first reminder: %v", err)
	}
	if _, err := service.Create(ReminderInput{
		Name: "მეორე",
		Type: "date",
		Date: date,
		Time: "11:00",
	}); err != nil {
		t.Fatalf("create second reminder: %v", err)
	}
	if !service.markFired(first.ID, "2026-10-28 10:00") {
		t.Fatal("mark reminder as fired")
	}
	if service.markFired(first.ID, "2026-10-28 10:00") {
		t.Fatal("same reminder fired twice in one minute")
	}
	pending, err := service.PendingAlerts()
	if err != nil {
		t.Fatalf("get pending reminder: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != first.ID {
		t.Fatalf("unexpected pending reminders: %#v", pending)
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatalf("close reminders database: %v", err)
	}

	reloaded := &ReminderService{path: path}
	if err := reloaded.open(); err != nil {
		t.Fatalf("reopen reminders database: %v", err)
	}
	reminders, err := reloaded.List()
	if err != nil {
		t.Fatalf("list reminders: %v", err)
	}
	if len(reminders) != 2 {
		t.Fatalf("loaded %d reminders, want 2", len(reminders))
	}

	if err := reloaded.Delete(first.ID); err != nil {
		t.Fatalf("delete reminder: %v", err)
	}
	reminders, err = reloaded.List()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(reminders) != 1 || reminders[0].Name != "მეორე" {
		t.Fatalf("unexpected reminders after delete: %#v", reminders)
	}
	if err := reloaded.ServiceShutdown(); err != nil {
		t.Fatalf("close reopened reminders database: %v", err)
	}
}

func TestSchedulerUsesCachedRemindersAndPersistsWeeklyDelivery(t *testing.T) {
	service := &ReminderService{path: filepath.Join(t.TempDir(), "reminders.db")}
	if err := service.open(); err != nil {
		t.Fatalf("open reminders database: %v", err)
	}
	defer service.ServiceShutdown()

	now := time.Now().Truncate(time.Minute)
	reminder, err := service.Create(ReminderInput{
		Name:     "შემოწმება",
		Type:     "weekly",
		Time:     now.Format("15:04"),
		Weekdays: []int{int(now.Weekday())},
	})
	if err != nil {
		t.Fatalf("create due reminder: %v", err)
	}

	fired := 0
	service.onDue = func(got Reminder) {
		if got.ID != reminder.ID {
			t.Errorf("fired reminder ID = %q, want %q", got.ID, reminder.ID)
		}
		fired++
	}

	service.checkDueReminders(now)
	service.checkDueReminders(now)
	if fired != 1 {
		t.Fatalf("onDue called %d times, want once", fired)
	}

	wantKey := now.Format("2006-01-02 15:04")
	service.mu.Lock()
	cachedKey := service.reminders[0].LastFiredKey
	service.mu.Unlock()
	if cachedKey != wantKey {
		t.Fatalf("cached fired key = %q, want %q", cachedKey, wantKey)
	}

	var storedKey string
	if err := service.db.QueryRow(
		`SELECT last_fired_key FROM reminders WHERE id = ?`, reminder.ID,
	).Scan(&storedKey); err != nil {
		t.Fatalf("read persisted fired key: %v", err)
	}
	if storedKey != wantKey {
		t.Fatalf("stored fired key = %q, want %q", storedKey, wantKey)
	}
}

func TestOneTimeReminderIsDeletedAfterFiring(t *testing.T) {
	service := &ReminderService{path: filepath.Join(t.TempDir(), "reminders.db")}
	if err := service.open(); err != nil {
		t.Fatalf("open reminders database: %v", err)
	}
	defer service.ServiceShutdown()

	now := time.Now().Truncate(time.Minute)
	reminder, err := service.Create(ReminderInput{
		Name: "ერთჯერადი",
		Type: "date",
		Date: now.Format("2006-01-02"),
		Time: now.Format("15:04"),
	})
	if err != nil {
		t.Fatalf("create one-time reminder: %v", err)
	}

	fired := 0
	service.onDue = func(got Reminder) {
		if got.ID != reminder.ID {
			t.Errorf("fired reminder ID = %q, want %q", got.ID, reminder.ID)
		}
		fired++
	}
	service.checkDueReminders(now)
	service.checkDueReminders(now)
	if fired != 1 {
		t.Fatalf("onDue called %d times, want once", fired)
	}

	reminders, err := service.List()
	if err != nil {
		t.Fatalf("list reminders after firing: %v", err)
	}
	if len(reminders) != 0 {
		t.Fatalf("one-time reminder remained in cache: %#v", reminders)
	}

	var count int
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM reminders WHERE id = ?`, reminder.ID).Scan(&count); err != nil {
		t.Fatalf("check one-time reminder in database: %v", err)
	}
	if count != 0 {
		t.Fatalf("one-time reminder remains in database, count = %d", count)
	}

	pending, err := service.PendingAlerts()
	if err != nil {
		t.Fatalf("get fired reminder alert: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != reminder.ID {
		t.Fatalf("fired reminder alert was not preserved: %#v", pending)
	}
}

func TestUpdateReminderRefreshesCacheAndStorage(t *testing.T) {
	service := &ReminderService{path: filepath.Join(t.TempDir(), "reminders.db")}
	if err := service.open(); err != nil {
		t.Fatalf("open reminders database: %v", err)
	}
	defer service.ServiceShutdown()

	reminder, err := service.Create(ReminderInput{
		Name:     "Original",
		Type:     "weekly",
		Time:     "09:00",
		Weekdays: []int{1},
	})
	if err != nil {
		t.Fatalf("create reminder: %v", err)
	}
	updated, err := service.Update(reminder.ID, ReminderInput{
		Name:     "Updated",
		Type:     "weekly",
		Time:     "10:30",
		Weekdays: []int{2, 4},
	})
	if err != nil {
		t.Fatalf("update reminder: %v", err)
	}
	if updated.Name != "Updated" || updated.Time != "10:30" ||
		!reflect.DeepEqual(updated.Weekdays, []int{2, 4}) {
		t.Fatalf("unexpected updated reminder: %#v", updated)
	}

	cached, err := service.List()
	if err != nil {
		t.Fatalf("list cached reminders: %v", err)
	}
	if len(cached) != 1 || cached[0].Name != updated.Name ||
		!reflect.DeepEqual(cached[0].Weekdays, updated.Weekdays) {
		t.Fatalf("cache not refreshed after update: %#v", cached)
	}
	stored, err := service.loadStoredReminders()
	if err != nil {
		t.Fatalf("load stored reminders: %v", err)
	}
	if len(stored) != 1 || stored[0].Name != updated.Name ||
		!reflect.DeepEqual(stored[0].Weekdays, updated.Weekdays) {
		t.Fatalf("database not updated: %#v", stored)
	}
}

func TestLegacyJSONIsMigratedToSQLite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reminders.db")
	legacyPath := filepath.Join(dir, "reminders.json")
	legacy := struct {
		Reminders []storedReminder `json:"reminders"`
	}{
		Reminders: []storedReminder{{
			Reminder: Reminder{
				ID:       "legacy-id",
				Name:     "ძველი შეხსენება",
				Type:     "weekly",
				Time:     "08:30",
				Weekdays: []int{1, 4},
			},
			LastFiredKey: "2026-10-27 08:30",
		}},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("encode legacy reminders: %v", err)
	}
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil {
		t.Fatalf("write legacy reminders: %v", err)
	}

	service := &ReminderService{path: path}
	if err := service.open(); err != nil {
		t.Fatalf("open database and migrate legacy reminders: %v", err)
	}
	defer service.ServiceShutdown()

	reminders, err := service.List()
	if err != nil {
		t.Fatalf("list migrated reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].ID != "legacy-id" ||
		!reflect.DeepEqual(reminders[0].Weekdays, []int{1, 4}) {
		t.Fatalf("unexpected migrated reminders: %#v", reminders)
	}

	var lastFiredKey string
	if err := service.db.QueryRow(
		`SELECT last_fired_key FROM reminders WHERE id = ?`, "legacy-id",
	).Scan(&lastFiredKey); err != nil {
		t.Fatalf("read migrated delivery time: %v", err)
	}
	if lastFiredKey != "2026-10-27 08:30" {
		t.Fatalf("last fired key = %q, want migrated value", lastFiredKey)
	}
	if _, err := os.Stat(legacyPath + ".migrated"); err != nil {
		t.Fatalf("legacy data was not archived after migration: %v", err)
	}
}
