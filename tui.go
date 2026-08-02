package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func ioctl(fd, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

// rawMode turns off echo and line buffering so the picker reads single
// keystrokes. ISIG is cleared as well, so Ctrl-C arrives as a 0x03 byte
// instead of a signal and the deferred restore is guaranteed to run.
func rawMode(fd uintptr) (*syscall.Termios, error) {
	var old syscall.Termios
	if err := ioctl(fd, syscall.TIOCGETA, unsafe.Pointer(&old)); err != nil {
		return nil, err
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctl(fd, syscall.TIOCSETA, unsafe.Pointer(&raw)); err != nil {
		return nil, err
	}
	return &old, nil
}

func termWidth(fd uintptr) int {
	var ws struct{ row, col, xpixel, ypixel uint16 }
	if err := ioctl(fd, syscall.TIOCGWINSZ, unsafe.Pointer(&ws)); err != nil || ws.col == 0 {
		return 80
	}
	return int(ws.col)
}

type action int

const (
	actNone action = iota
	actQuit
	actConfirm
)

// handleKey applies the first keypress in key to the picker state. It
// toggles on in place and reports how many bytes the keypress consumed, so
// the caller can advance through a buffer holding several keystrokes.
func handleKey(key []byte, cur int, on []bool) (used, newCur int, act action) {
	n := len(on)
	if len(key) == 0 || n == 0 {
		return 0, cur, actQuit
	}
	if key[0] == 0x1b {
		if len(key) >= 3 && key[1] == '[' {
			switch key[2] {
			case 'A':
				return 3, (cur - 1 + n) % n, actNone
			case 'B':
				return 3, (cur + 1) % n, actNone
			}
			return 3, cur, actNone // some other CSI sequence, ignored
		}
		return 1, cur, actQuit // bare Esc
	}
	switch key[0] {
	case 'k':
		return 1, (cur - 1 + n) % n, actNone
	case 'j':
		return 1, (cur + 1) % n, actNone
	case ' ':
		on[cur] = !on[cur]
	case 'a':
		for i := range on {
			on[i] = true
		}
	case 'n':
		for i := range on {
			on[i] = false
		}
	case '\r', '\n':
		return 1, cur, actConfirm
	case 'q', 0x03: // q, Ctrl-C
		return 1, cur, actQuit
	}
	return 1, cur, actNone
}

// handleKeys drains one read buffer. A single read can carry several
// keystrokes from key repeat, a paste, or input typed before the terminal
// switched to raw mode, so every one has to be applied or the extras are
// lost. It stops at the first key that quits or confirms.
func handleKeys(buf []byte, cur int, on []bool) (int, action) {
	for i := 0; i < len(buf); {
		used, next, act := handleKey(buf[i:], cur, on)
		cur = next
		if used <= 0 {
			return cur, actQuit
		}
		i += used
		if act != actNone {
			return cur, act
		}
	}
	return cur, actNone
}

func clip(s string, w int) string {
	if w > 1 && len(s) > w-1 {
		return s[:w-1]
	}
	return s
}

// draw repaints the list in place. prev is the line count of the previous
// frame so the cursor can move back up over it.
func draw(jobs []cleaner, on []bool, cur, prev, width int) int {
	var b strings.Builder
	if prev > 0 {
		fmt.Fprintf(&b, "\x1b[%dA", prev)
	}
	b.WriteString("\x1b[J")

	write := func(s string) {
		b.WriteString(clip(s, width))
		b.WriteByte('\n')
	}
	write("Select what to clean:")
	write("  up/down move   space toggle   a all   n none   enter run   q quit")
	write("")

	var total int64
	selected := 0
	for i, c := range jobs {
		mark := " "
		if on[i] {
			mark = "x"
			total += c.size()
			selected++
		}
		point := "  "
		if i == cur {
			point = "> "
		}
		write(fmt.Sprintf("%s[%s] %-22s %10s  %s", point, mark, c.name,
			humanize(uint64(c.size())), strings.Join(c.argv, " ")))
	}
	write("")
	write(fmt.Sprintf("%d of %d selected (%s)", selected, len(jobs), humanize(uint64(total))))

	os.Stdout.WriteString(b.String())
	return len(jobs) + 5
}

type pickStatus int

const (
	pickNoTTY pickStatus = iota
	pickCancelled
	pickConfirmed
)

// pick runs the interactive list. It reports pickNoTTY when stdin is not a
// terminal, which lets the caller fall back to non-interactive output.
func pick(jobs []cleaner) (chosen []cleaner, status pickStatus) {
	fd := os.Stdin.Fd()
	old, err := rawMode(fd)
	if err != nil {
		return nil, pickNoTTY
	}
	defer ioctl(fd, syscall.TIOCSETA, unsafe.Pointer(old))

	os.Stdout.WriteString("\x1b[?25l")
	defer os.Stdout.WriteString("\x1b[?25h")

	on := make([]bool, len(jobs))
	for i := range on {
		on[i] = true
	}

	cur, prev := 0, 0
	buf := make([]byte, 64)
	for {
		prev = draw(jobs, on, cur, prev, termWidth(fd))
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return nil, pickCancelled
		}
		next, act := handleKeys(buf[:n], cur, on)
		cur = next
		switch act {
		case actQuit:
			return nil, pickCancelled
		case actConfirm:
			for i, c := range jobs {
				if on[i] {
					chosen = append(chosen, c)
				}
			}
			return chosen, pickConfirmed
		}
	}
}
