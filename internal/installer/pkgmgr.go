package installer

import (
	"fmt"
	"os"
	"sync"
)

// pkgMgrMu serializes every system-package-manager invocation.
//
// pacman holds /var/lib/pacman/db.lck exclusively for the duration of
// any transaction, so a second concurrent `pacman -S` aborts with
// "unable to lock database". apt/dpkg behave identically with
// /var/lib/dpkg/lock-frontend. Without this serialization, parallel
// installs (config.parallel: true) collide whenever any tool uses the
// `system` installer or declares `depends:`.
//
// A secondary win: only one sudo prompt can read stdin at a time, so
// holding this mutex during the subprocess prevents two concurrent
// sudos from deadlocking on the password prompt.
var pkgMgrMu sync.Mutex

// runPkgMgrCmd runs a `sudo <args...>` invocation under the package
// manager mutex. assumeYes is consulted purely for the TTY guard: it
// doesn't add `--noconfirm`/`-y` to args (callers do that themselves
// since the right flag varies by pkg mgr). We refuse to launch into
// pacman/apt when stdin isn't a terminal AND assumeYes is false —
// otherwise the subprocess would block forever waiting for the
// confirmation prompt that no human is around to answer.
func runPkgMgrCmd(assumeYes bool, args ...string) error {
	if !assumeYes && !stdinIsTerminal() {
		return fmt.Errorf("non-interactive shell detected and -y/--yes not set; rerun with -y to bypass system pkg mgr prompts")
	}
	pkgMgrMu.Lock()
	defer pkgMgrMu.Unlock()
	return runCmd("sudo", args...)
}

// stdinIsTerminal reports whether the process's stdin is attached to a
// TTY. Uses ModeCharDevice rather than pulling in golang.org/x/term —
// good enough for Linux, which is the only platform berserk targets.
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// aptUpdateOnce ensures `sudo apt update` runs at most once per process.
// SystemUpdate() previously called it per tool — under a parallel batch
// of N system tools, that meant N apt-update invocations each fighting
// for /var/lib/apt/lists/lock.
var (
	aptUpdateOnce sync.Once
	aptUpdateErr  error
)

// ensureAptIndex refreshes the apt package index at most once per process.
// Subsequent calls return the cached error from the first attempt. Cheap
// to call, safe to call from any goroutine.
func ensureAptIndex(assumeYes bool) error {
	aptUpdateOnce.Do(func() {
		// `apt update` itself doesn't prompt, but it does need sudo
		// and we still want it serialized against any concurrent
		// pacman/apt activity.
		aptUpdateErr = runPkgMgrCmd(assumeYes, "apt", "update")
	})
	return aptUpdateErr
}
