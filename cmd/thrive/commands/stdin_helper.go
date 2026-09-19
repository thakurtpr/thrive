package commands

import (
	"io"
	"os"
)

// readAllStdin reads all of stdin (shared by proxy commands).
func readAllStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
