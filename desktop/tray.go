// desktop/tray.go

//go:build darwin || windows

package desktop

import (
	"context"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

const pollInterval = 5 * time.Second

var (
	menuOnce       sync.Once
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
	menuOnce.Do(func() {
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

		go func() {
			for {
				select {
				case <-itemStart.ClickedCh:
					exec.CommandContext(context.Background(), "thrive", "desktop", "start").Run()
				case <-itemRestart.ClickedCh:
					exec.CommandContext(context.Background(), "thrive", "desktop", "restart").Run()
				case <-itemStop.ClickedCh:
					exec.CommandContext(context.Background(), "thrive", "desktop", "stop").Run()
				}
			}
		}()

		itemStatus := systray.AddMenuItem("Status", "Show VM status")
		go func() {
			for {
				select {
				case <-itemStatus.ClickedCh:
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					out, err := exec.CommandContext(ctx, "thrive", "desktop", "status").Output()
					if err == nil {
						log.Printf("%s", out)
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
	})

	go func() {
		for {
			time.Sleep(pollInterval)
			newState, err := vm.ReadVMState()
			if err == nil && (currentState == nil || newState.Running != currentState.Running) {
				currentState = newState
				updateVisibility()
			}
		}
	}()
}

func onExit() {}

func updateVisibility() {
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