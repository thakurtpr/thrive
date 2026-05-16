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
	// Note: systray does not support ClearMenu - menu items are rebuilt by re-running systray
	// For simplicity, we add items that reflect current state
	// In production, consider using menu item checkboxes or a state machine approach

	statusText := "○ Thrive Stopped"
	if currentState != nil && currentState.Running {
		statusText = "● Thrive Running"
	}
	systray.AddMenuItem(statusText, "Thrive VM status")

	systray.AddSeparator()

	if currentState != nil && currentState.Running {
		itemRestart := systray.AddMenuItem("Restart VM", "Restart the Thrive VM")
		go func() {
			<-itemRestart.ClickedCh
			exec.Command("thrive", "desktop", "restart").Run()
			updateMenu()
		}()

		itemStop := systray.AddMenuItem("Stop VM", "Stop the Thrive VM")
		go func() {
			<-itemStop.ClickedCh
			exec.Command("thrive", "desktop", "stop").Run()
			updateMenu()
		}()
	} else {
		itemStart := systray.AddMenuItem("Start VM", "Start the Thrive VM")
		go func() {
			<-itemStart.ClickedCh
			exec.Command("thrive", "desktop", "start").Run()
			updateMenu()
		}()
	}

	systray.AddSeparator()

	itemStatus := systray.AddMenuItem("Status", "Show VM status")
	go func() {
		<-itemStatus.ClickedCh
		out, err := exec.Command("thrive", "desktop", "status").Output()
		if err == nil {
			fmt.Print(string(out))
		}
	}()

	itemQuit := systray.AddMenuItem("Quit Thrive", "Quit Thrive Desktop")
	go func() {
		<-itemQuit.ClickedCh
		os.Exit(0)
	}()
}