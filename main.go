package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// From <sys/mount.h>; darwin's syscall package does not export these.
const (
	mntRdonly = 0x1
	mntNowait = 2
)

func cstr(b []int8) string {
	var sb strings.Builder
	for _, c := range b {
		if c == 0 {
			break
		}
		sb.WriteByte(byte(c))
	}
	return sb.String()
}

func humanize(bytes uint64) string {
	const unit = 1000
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// keep reports whether a mount is a real drive the user can run out of
// space on: writable and backed by a device. The root "/" is a sealed
// read-only snapshot on modern macOS, so the internal drive is represented
// by /System/Volumes/Data, which reports the shared container's free space.
func keep(mntFrom, mntOn string, flags uint32) bool {
	if flags&mntRdonly != 0 {
		return false // ISOs, read-only disk images, and the sealed "/"
	}
	if !strings.HasPrefix(mntFrom, "/dev/") {
		return false // devfs, map auto_home, network mounts
	}
	return mntOn == "/System/Volumes/Data" || strings.HasPrefix(mntOn, "/Volumes/")
}

func main() {
	switch {
	case len(os.Args) == 1:
		report()
	case os.Args[1] == "clean":
		clean(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "usage: actualds [clean [--yes|--dry-run]]")
		os.Exit(2)
	}
}

func report() {
	n, err := syscall.Getfsstat(nil, mntNowait)
	if err != nil {
		fmt.Fprintln(os.Stderr, "actualds:", err)
		os.Exit(1)
	}
	fs := make([]syscall.Statfs_t, n)
	if _, err := syscall.Getfsstat(fs, mntNowait); err != nil {
		fmt.Fprintln(os.Stderr, "actualds:", err)
		os.Exit(1)
	}

	fmt.Printf("%-24s %10s %10s %10s  %s\n", "VOLUME", "SIZE", "USED", "FREE", "FREE%")
	for _, f := range fs {
		mntFrom := cstr(f.Mntfromname[:])
		mntOn := cstr(f.Mntonname[:])
		if !keep(mntFrom, mntOn, f.Flags) {
			continue
		}
		bs := uint64(f.Bsize)
		total := f.Blocks * bs
		free := f.Bavail * bs
		used := total - f.Bfree*bs
		name := mntOn
		if name == "/System/Volumes/Data" {
			name = "Macintosh HD"
		} else {
			name = strings.TrimPrefix(name, "/Volumes/")
		}
		fmt.Printf("%-24s %10s %10s %10s  %4.1f%%\n",
			name, humanize(total), humanize(used), humanize(free),
			float64(free)/float64(total)*100)
	}
}
