# Alarme

A Windows reminder app built with Wails 3. Reminders are stored in the user's
configuration directory, and continue running while the app is hidden in the
system tray. When a reminder is due, a small separate window displays its text.
No Windows system notification is sent.
The interface supports dark and light themes. Use the theme switch in the main
window to change appearance; the selected theme is saved between launches.
Each reminder can optionally play a notification chime; **No Sound** is the
default.

## გაშვება

საჭიროა Go, Node.js/npm და Windows-ზე WebView2 Runtime.

```powershell
wails3 dev
```

## Build

```powershell
wails3 build
```

The resulting Windows executable is available at `bin\alarme.exe`.
The window's `X` button hides the app in the system tray. To exit, right-click
the tray icon and select **Exit**.

Reminders are stored in a SQLite database at `%APPDATA%\alarme\reminders.db`.
On first launch, an old `reminders.json` file is migrated into the database and
archived as `reminders.json.migrated`.
