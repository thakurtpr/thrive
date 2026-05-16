// desktop/tray.go

//go:build darwin || windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

var (
	menuInitialized bool
	itemStart, itemRestart, itemStop *systray.MenuItem
	statusText *systray.MenuItem
)

var currentState *vm.VMState

func init() {
	currentState, _ = vm.ReadVMState()
}

func RunTray() {
	systray.Run(onReady, onExit)
}

func onReady() {
	updateMenu()

	go func() {
		for {
			time.Sleep(5 * time.Second)
			newState, err := vm.ReadVMState()
			if err == nil && (currentState == nil || newState.Running != currentState.Running) {
				currentState = newState
				updateMenu()
			}
		}
	}()
}

func onExit() {}

func updateMenu() {
	if !menuInitialized {
		// First time: create all menu items
		statusText = systray.AddMenuItem("○ Thrive Stopped", "Thrive VM status")
		systray.AddSeparator()
		itemStart = systray.AddMenuItem("Start VM", "Start the Thrive VM")
		itemRestart = systray.AddMenuItem("Restart VM", "Restart the Thrive VM")
		itemStop = systray.AddMenuItem("Stop VM", "Stop the Thrive VM")
		itemStop.Hide()
		itemRestart.Hide()
		systray.AddSeparator()
		systray.AddMenuItem("Status", "Show VM status")
		systray.AddMenuItem("Quit Thrive", "Quit Thrive Desktop")
		menuInitialized = true

		// Set up click handlers
		go func() {
			for {
				select {
				case <-itemStart.ClickedCh:
					exec.Command("thrive", "desktop", "start").Run()
				case <-itemRestart.ClickedCh:
					exec.Command("thrive", "desktop", "restart").Run()
				case <-itemStop.ClickedCh:
					exec.Command("thrive", "desktop", "stop").Run()
				}
			}
		}()

		itemStatus := systray.AddMenuItem("Status", "Show VM status")
		go func() {
			for {
				select {
				case <-itemStatus.ClickedCh:
					out, err := exec.Command("thrive", "desktop", "status").Output()
					if err == nil {
						fmt.Print(string(out))
					}
				}
			}
		}()

		itemQuit := systray.AddMenuItem("Quit Thrive", "Quit Thrive Desktop")
		go func() {
			for {
				select {
				case <-itemQuit.ClickedCh:
					os.Exit(0)
				}
			}
		}()

		return
	}

	// Update visibility based on state
	if currentState != nil && currentState.Running {
		statusText.SetTitle("● Thrive Running")
		itemStart.Hide()
		itemStop.Show()
		itemRestart.Show()
	} else {
		statusText.SetTitle("○ Thrive Stopped")
		itemStart.Show()
		itemStop.Hide()
		itemRestart.Hide()
	}
}