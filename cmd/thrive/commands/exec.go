//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

// shellQuoteArg quotes one shell word.
func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'\\$`!*?[]{}()|&;<>") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func ExecCmd() *cobra.Command {
	var envVars []string
	var workdir string
	var interactive, tty bool
	cmd := &cobra.Command{
		Use:   "exec [container] [command...]",
		Short: "Execute a command in a running container",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]
			command := args[1:]

			state, err := runtime.State(ctx, containerID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: container not found: %v\n", err)
				os.Exit(1)
			}
			if state.Status != "running" || state.PID == 0 {
				fmt.Fprintf(os.Stderr, "Error: container %s is not running\n", containerID)
				os.Exit(1)
			}

			// Env via the env(1) prefix (no nsenter version dependency);
			// workdir via sh cd+exec wrapper for the same reason.
			if len(envVars) > 0 {
				command = append(append([]string{"env"}, envVars...), command...)
			}
			if workdir != "" {
				script := "cd " + shellQuoteArg(workdir) + " && exec \"$@\""
				command = append([]string{"/bin/sh", "-c", script, "thrive-exec"}, command...)
			}
			_ = interactive
			_ = tty

			nsenterArgs := buildNsenterArgs(state.PID)
			nsenterArgs = append(nsenterArgs, command...)

			os.Exit(execInContainerNS(ctx, nsenterArgs))
		},
	}
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Set environment variables")
	cmd.Flags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep stdin open (implied: stdin is attached)")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY (informational: output is not raw-PTY)")
	// Stop flag parsing after the container name so command flags (e.g. uname -a)
	// are not mistaken for thrive exec flags.
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// buildNsenterArgs returns the nsenter prefix (through "--") for entering
// a container's namespaces. --root pins the process to the container's
// root: nsenter changes namespaces but NOT the caller's chroot, so without
// it filesystem writes land on the host (found via e2e diff: touch missed
// the container). /proc/<pid>/root always tracks the target's root,
// including chroot-only containers.
func buildNsenterArgs(pid int) []string {
	return []string{
		"--target", strconv.Itoa(pid),
		"--mount", "--pid", "--ipc", "--uts", "--net",
		"--root=/proc/" + strconv.Itoa(pid) + "/root",
		"--",
	}
}

// execInContainerNS runs a prebuilt nsenter argv, returning the exit code.
func execInContainerNS(ctx context.Context, nsenterArgs []string) int {
	execCmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return ws.ExitStatus()
			}
		}
		fmt.Fprintf(os.Stderr, "exec error: %v\n", err)
		return 1
	}
	return 0
}
