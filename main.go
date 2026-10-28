//go:build windows

package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func launchedFromStartup() bool {
	for _, arg := range os.Args[1:] {
		if arg == startupLaunchArgument {
			return true
		}
	}
	return false
}

func activateMainWindow(window *application.WebviewWindow) {
	if window == nil {
		return
	}
	// Temporarily use the topmost z-order so Windows raises the window even
	// when the second-instance callback runs outside the foreground process.
	window.SetAlwaysOnTop(true)
	window.Show()
	window.Restore()
	window.Focus()
	window.SetAlwaysOnTop(false)
	window.Focus()
}

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	application.RegisterEvent[Reminder]("reminder:due")
	application.RegisterEvent[Reminder]("reminder:removed")
}

func main() {
	reminderService, err := NewReminderService()
	if err != nil {
		log.Fatal(err)
	}

	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "Alarme",
		Description: "Alarme",
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.jax.alarme",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				activateMainWindow(window)
			},
		},
		Services: []application.Service{
			application.NewService(reminderService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Alarme",
		Hidden:           launchedFromStartup(),
		Width:            520,
		Height:           560,
		MinWidth:         420,
		MinHeight:        500,
		DisableResize:    true,
		BackgroundColour: application.NewRGB(15, 17, 24),
		URL:              "/",
	})

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
	})

	alertWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "reminder-alert",
		Title:            "Reminder",
		Width:            300,
		Height:           176,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		Hidden:           true,
		InitialPosition:  application.WindowCentered,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Windows: application.WindowsWindow{
			DisableFramelessWindowDecorations: true,
		},
		URL: "/?popup=1",
	})
	alertWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		alertWindow.Hide()
		event.Cancel()
	})

	reminderService.onDue = func(reminder Reminder) {
		alertWindow.Show()
		alertWindow.Focus()
		alertWindow.EmitEvent("reminder:due", reminder)
		window.EmitEvent("reminder:due", reminder)
		if reminder.Type == "date" {
			window.EmitEvent("reminder:removed", reminder)
		}
	}
	reminderService.onDismiss = func() {
		alertWindow.Hide()
	}

	tray := app.SystemTray.New().SetIcon(appIcon)
	tray.SetTooltip("Alarme")

	trayMenu := app.NewMenu()
	trayMenu.Add("Open Alarme").OnClick(func(_ *application.Context) {
		window.Show()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Exit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(trayMenu)
	tray.OnClick(func() {
		window.Show()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
