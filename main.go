package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	timerRunning   bool
	timerPaused    bool
	remainingTime  int
	mu             sync.Mutex
	pauseResumeBtn *widget.Button
	progressBar    *widget.ProgressBar
	totalTime      int
)

func main() {
	a := app.New()
	w := a.NewWindow("AppTimer")
	w.Resize(fyne.NewSize(400, 400))

	// Input fields
	countdownEntry := widget.NewEntry()
	countdownEntry.SetPlaceHolder("Enter time duration")

	appNameEntry := widget.NewEntry()
	appNameEntry.SetPlaceHolder("Enter app name (e.g., 'firefox')")

	timeUnitSelect := widget.NewSelect([]string{"Seconds", "Minutes", "Hours"}, func(value string) {})

	timeRemainingLabel := widget.NewLabel("Time remaining: --")

	// Progress bar
	progressBar = widget.NewProgressBar()
	progressBar.Hide()

	// Start/Stop and Pause/Resume buttons
	startStopBtn := widget.NewButton("Start Timer", nil)
	pauseResumeBtn = widget.NewButton("Pause Timer", nil)
	pauseResumeBtn.Disable()
	var stopTimer func()

	// Notification checkbox
	notifyCheckbox := widget.NewCheck("Enable notifications", nil)

	startStopBtn.OnTapped = func() {
		if timerRunning {
			stopTimer()
		} else {
			duration, err := strconv.Atoi(countdownEntry.Text)
			if err != nil || duration <= 0 {
				dialog.ShowError(fmt.Errorf("invalid time input"), w)
				return
			}

			appName := appNameEntry.Text
			if appName == "" {
				dialog.ShowError(fmt.Errorf("app name cannot be empty"), w)
				return
			}

			unitMultiplier := 1
			switch timeUnitSelect.Selected {
			case "Seconds":
				unitMultiplier = 1
			case "Minutes":
				unitMultiplier = 60
			case "Hours":
				unitMultiplier = 3600
			default:
				dialog.ShowError(fmt.Errorf("please select a time unit"), w)
				return
			}
			remainingTime = duration * unitMultiplier
			totalTime = remainingTime

			pid, err := launchApp(appName)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to launch %s: %v", appName, err), w)
				return
			}

			startStopBtn.SetText("Stop Timer And Kill App")
			pauseResumeBtn.Enable()
			pauseResumeBtn.SetText("Pause Timer")
			progressBar.Show()
			mu.Lock()
			timerRunning = true
			timerPaused = false
			mu.Unlock()

			stopTimer = func() {
				mu.Lock()
				timerRunning = false
				timerPaused = false
				mu.Unlock()
				err := killAppByPID(pid)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to kill %s: %v", appName, err), w)
				} else {
					dialog.ShowInformation("Timer Stopped", fmt.Sprintf("%s has been closed.", appName), w)
				}
				startStopBtn.SetText("Start Timer")
				pauseResumeBtn.Disable()
				timeRemainingLabel.SetText("Time remaining: --")
				progressBar.Hide()
			}

			pauseResumeBtn.OnTapped = func() {
				mu.Lock()
				if timerPaused {
					timerPaused = false
					pauseResumeBtn.SetText("Pause Timer")
					go countdownTimer(pid, timeRemainingLabel, stopTimer, notifyCheckbox.Checked)
				} else {
					timerPaused = true
					pauseResumeBtn.SetText("Resume Timer")
				}
				mu.Unlock()
			}

			go countdownTimer(pid, timeRemainingLabel, stopTimer, notifyCheckbox.Checked)
		}
	}

	// Theme toggle
	themeToggle := widget.NewCheck("Dark Theme", func(value bool) {
		if value {
			a.Settings().SetTheme(theme.DarkTheme())
		} else {
			a.Settings().SetTheme(theme.LightTheme())
		}
	})

	// Layout
	form := container.NewVBox(
		widget.NewLabel("Enter the time you want to spend:"),
		countdownEntry,
		timeUnitSelect,
		widget.NewLabel("Enter the app you want to limit time on:"),
		appNameEntry,
		timeRemainingLabel,
		progressBar,
		startStopBtn,
		pauseResumeBtn,
		notifyCheckbox,
		themeToggle,
	)

	w.SetContent(form)
	w.ShowAndRun()
}

func countdownTimer(pid int, timeRemainingLabel *widget.Label, stopTimer func(), notify bool) {
	for remainingTime > 0 {
		mu.Lock()
		if !timerRunning || timerPaused {
			mu.Unlock()
			break
		}
		timeRemainingLabel.SetText(fmt.Sprintf("Time remaining: %d seconds", remainingTime))
		progressBar.SetValue(float64(totalTime-remainingTime) / float64(totalTime))
		mu.Unlock()

		if notify && remainingTime == 60 { // Notify when 1 minute is left
			notifyUser("1 minute remaining!")
		}

		time.Sleep(1 * time.Second)
		remainingTime--
	}

	mu.Lock()
	if timerRunning && !timerPaused {
		timerRunning = false
		mu.Unlock()
		err := killAppByPID(pid)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to kill app: %v", err), nil)
		} else {
			dialog.ShowInformation("Time's Up", fmt.Sprintf("Time's up! App has been closed."), nil)
			if notify {
				notifyUser("Time's up! App has been closed.")
			}
		}
		stopTimer()
	} else {
		mu.Unlock()
	}
}

func launchApp(appName string) (int, error) {
	cmd := exec.Command(appName)
	err := cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start the application: %w", err)
	}

	pid := cmd.Process.Pid
	fmt.Printf("%s launched with PID %d\n", appName, pid)
	return pid, nil
}

func killAppByPID(pid int) error {
	cmd := exec.Command("kill", fmt.Sprintf("%d", pid))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to kill process with PID %d: %w", pid, err)
	}

	fmt.Printf("Process with PID %d has been killed.\n", pid)
	return nil
}

func notifyUser(message string) {
	cmd := exec.Command("notify-send", "AppTimer", message)
	cmd.Run()
}
