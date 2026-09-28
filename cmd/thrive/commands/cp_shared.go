package commands

import (
	"strings"
)

// parseCpArgs splits `thrive cp` SRC/DST into container ID, source path,
// destination path, and direction. A colon in SRC means copy-from-container;
// otherwise a colon in DST means copy-to-container. Shared by the Linux
// native implementation (cp.go) and the VM-daemon proxy (cp_proxy.go) so
// both parse identically on every platform.
func parseCpArgs(src, dst string) (containerID, srcPath, dstPath string, toContainer bool) {
	if i := strings.Index(src, ":"); i > 0 {
		return src[:i], src[i+1:], dst, false
	}
	if i := strings.Index(dst, ":"); i > 0 {
		return dst[:i], src, dst[i+1:], true
	}
	return "", src, dst, false
}
