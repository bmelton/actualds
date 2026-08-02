package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// A cleaner deletes one category of regenerable data. argv is both the
// displayed command and the command that runs, so a dry run cannot differ
// from what --yes executes.
type cleaner struct {
	name string
	tool string // binary that must be installed; empty means no requirement
	size func() int64
	argv []string
}

func home(p string) string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(h, p)
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func duSize(path string) func() int64 {
	return func() int64 {
		if path == "" {
			return 0
		}
		out, err := exec.Command("du", "-sk", path).Output()
		if err != nil {
			return 0
		}
		fields := strings.Fields(string(out))
		if len(fields) == 0 {
			return 0
		}
		kb, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
}

// parseSize reads the base-1000 sizes docker prints, e.g. "21.25GB".
func parseSize(s string) int64 {
	units := []struct {
		suffix string
		mult   float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}}
	for _, u := range units {
		num, ok := strings.CutSuffix(s, u.suffix)
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0
		}
		return int64(f * u.mult)
	}
	return 0
}

// dockerReclaim reports what docker says it can free for one resource type.
// It returns 0 when the daemon is not running, which drops the entry.
func dockerReclaim(resource string) func() int64 {
	return func() int64 {
		out, err := exec.Command("docker", "system", "df",
			"--format", "{{.Type}}|{{.Reclaimable}}").Output()
		if err != nil {
			return 0
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			kind, value, ok := strings.Cut(line, "|")
			if !ok || kind != resource {
				continue
			}
			fields := strings.Fields(value)
			if len(fields) == 0 {
				return 0
			}
			return parseSize(fields[0])
		}
		return 0
	}
}

func cleaners() []cleaner {
	derived := home("Library/Developer/Xcode/DerivedData")
	return []cleaner{
		{"Docker build cache", "docker", dockerReclaim("Build Cache"),
			[]string{"docker", "builder", "prune", "-af"}},
		{"Docker unused images", "docker", dockerReclaim("Images"),
			[]string{"docker", "image", "prune", "-af"}},
		{"Go module cache", "go", duSize(goEnv("GOMODCACHE")),
			[]string{"go", "clean", "-modcache"}},
		{"Go build cache", "go", duSize(goEnv("GOCACHE")),
			[]string{"go", "clean", "-cache"}},
		{"Yarn cache", "yarn", duSize(home("Library/Caches/Yarn")),
			[]string{"yarn", "cache", "clean"}},
		{"Homebrew downloads", "brew", duSize(home("Library/Caches/Homebrew")),
			[]string{"brew", "cleanup", "--prune=all"}},
		{"npm cache", "npm", duSize(home(".npm")),
			[]string{"npm", "cache", "clean", "--force"}},
		{"pip cache", "pip3", duSize(home("Library/Caches/pip")),
			[]string{"pip3", "cache", "purge"}},
		{"Xcode DerivedData", "", duSize(derived),
			[]string{"rm", "-rf", derived}},
	}
}

func freeBytes() int64 {
	var s syscall.Statfs_t
	h, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	if err := syscall.Statfs(h, &s); err != nil {
		return 0
	}
	return int64(s.Bavail) * int64(s.Bsize)
}

// gather keeps the categories whose tool is installed and which have
// something to reclaim. Each size is measured once and memoized, so the
// picker can repaint without re-running du on every keystroke.
func gather() []cleaner {
	var jobs []cleaner
	for _, c := range cleaners() {
		if c.tool != "" {
			if _, err := exec.LookPath(c.tool); err != nil {
				continue
			}
		}
		n := c.size()
		if n <= 0 {
			continue
		}
		c.size = func() int64 { return n }
		jobs = append(jobs, c)
	}
	return jobs
}

func table(jobs []cleaner) {
	var total int64
	fmt.Printf("%-22s %10s  %s\n", "CATEGORY", "RECLAIMS", "COMMAND")
	for _, c := range jobs {
		total += c.size()
		fmt.Printf("%-22s %10s  %s\n", c.name, humanize(uint64(c.size())),
			strings.Join(c.argv, " "))
	}
	fmt.Printf("%-22s %10s\n", "total", humanize(uint64(total)))
}

func runJobs(jobs []cleaner) {
	before := freeBytes()
	ranDocker := false
	for _, c := range jobs {
		fmt.Printf("\n==> %s\n", strings.Join(c.argv, " "))
		cmd := exec.Command(c.argv[0], c.argv[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "actualds: %s failed: %v\n", c.name, err)
			continue
		}
		if c.tool == "docker" {
			ranDocker = true
		}
	}

	if freed := freeBytes() - before; freed > 0 {
		fmt.Printf("\nFreed %s.\n", humanize(uint64(freed)))
	} else {
		fmt.Println("\nNo change in free space yet.")
	}
	if ranDocker {
		// Docker Desktop stores the VM in a sparse image that does not always
		// shrink on the host right after a prune.
		fmt.Println("Docker space may need a Docker Desktop restart to appear.")
	}
}

func clean(args []string) {
	jobs := gather()
	if len(jobs) == 0 {
		fmt.Println("Nothing to clean.")
		return
	}

	switch {
	case slices.Contains(args, "--yes"):
		table(jobs)
		runJobs(jobs)
	case slices.Contains(args, "--dry-run"):
		table(jobs)
		fmt.Println("\nDry run. Re-run without --dry-run to choose categories.")
	default:
		chosen, status := pick(jobs)
		switch {
		case status == pickNoTTY:
			// Not a terminal, so there is no safe way to confirm.
			table(jobs)
			fmt.Println("\nNot a terminal. Re-run with --yes to execute.")
		case status == pickCancelled:
			fmt.Println("Cancelled. Nothing was deleted.")
		case len(chosen) == 0:
			fmt.Println("Nothing selected.")
		default:
			runJobs(chosen)
		}
	}
}
