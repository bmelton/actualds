# actualds

Free space on the drives you can actually run out of.

## Why

`df -h` on a Mac answers a question nobody asked. It lists every mount the
kernel knows about, so a real answer about your boot drive arrives buried under
`devfs`, `map auto_home`, every mounted ISO, every read-only disk image, and the
handful of APFS system volumes that macOS keeps for its own use.

Two macOS details make the raw output worse:

- Modern macOS mounts `/` as a sealed read-only snapshot. The volume you fill
  up is `/System/Volumes/Data`, which is not the line most people read.
- `df` reports the Data volume's own blocks as Used. On a shared APFS
  container that undercounts what is really consuming the disk.

`actualds` keeps only the volumes you can fill up: writable, backed by a real
device, and mounted either at `/System/Volumes/Data` or under `/Volumes`. The
sealed `/` is read-only, so it drops out with the ISOs. Nothing else is
printed.

## What it does

```
$ actualds
VOLUME                         SIZE       USED       FREE  FREE%
Macintosh HD               494.4 GB   389.4 GB   105.0 GB  21.2%
Dock (non-Backups)           2.0 TB   254.3 GB     1.7 TB  87.3%
Dock                       257.0 GB   246.3 GB    10.7 GB   4.2%
```

Free space matches `df` exactly. The Used column deliberately does not: it
counts everything in the shared APFS container that is not free, including the
sealed system volume, swap, and snapshots. For "what is eating my disk," that
is the number you want.

The whole thing reads mounts through `syscall.Getfsstat`. There are no
third-party dependencies.

## Reclaiming space

`actualds clean` finds regenerable data that is safe to delete and opens a
checklist:

```
Select what to clean:
  up/down move   space toggle   a all   n none   enter run   q quit

  [x] Docker build cache        21.2 GB  docker builder prune -af
  [x] Docker unused images       2.8 GB  docker image prune -af
> [ ] Go module cache            4.5 GB  go clean -modcache
  [x] Go build cache             2.9 GB  go clean -cache
  [x] Yarn cache                 4.4 GB  yarn cache clean
  [x] Homebrew downloads         2.9 GB  brew cleanup --prune=all
  [x] npm cache                  2.2 GB  npm cache clean --force
  [x] pip cache                212.0 MB  pip3 cache purge
  [x] Xcode DerivedData          1.7 GB  rm -rf ~/Library/Developer/Xcode/DerivedData

8 of 9 selected (38.3 GB)
```

Everything starts selected. Arrow keys or `j`/`k` move, space toggles, `a` and
`n` select all or none, Enter runs only the checked rows, and `q`, Esc, or
Ctrl-C cancels without deleting anything. The running total updates as you
toggle, so you can see what a given selection buys you before you commit.

Categories with no installed tool, or with nothing to reclaim, are skipped. The
Docker rows disappear when the daemon is not running.

After a run it reports space actually freed, measured from a `statfs` delta
rather than from the sum of the estimates.

### Design rules

Two rules keep this honest:

1. **Each tool prunes its own files.** `actualds` calls `docker builder prune`,
   `go clean -cache`, and `brew cleanup` instead of deleting paths it guessed
   at. Those tools know which of their files are live. A hand-written `rm -rf`
   does not, and that is where cleanup scripts cause damage.
2. **The command shown is the command that runs.** The text in the checklist is
   the exact argument list passed to `exec`, so the preview cannot drift from
   the behavior.

### What it will not touch

Deliberate omissions, because the usual advice about them is wrong:

- **All of `~/Library/Caches`.** Real user data hides there. Spotify keeps
  offline music in it and Chrome keeps profile data. Deleting a cache while its
  app runs can also corrupt state. Named subdirectories are fine; the parent is
  not.
- **APFS local snapshots.** macOS purges these under pressure on its own.
- **`/private/var/folders`.** System-managed temporary files with live
  processes holding them open.
- **`~/Library/Application Support` and MobileSync backups.** That is data, not
  cache.

The Trash is not included either, because `actualds` reports what it can
reclaim and the Trash is better emptied from Finder.

## Usage

```
actualds                    Show free space
actualds clean              Pick categories interactively, then run them
actualds clean --dry-run    Print the table and exit, changing nothing
actualds clean --yes        Run every category without prompting
```

When stdin is not a terminal the picker is skipped and the table is printed
with a pointer to `--yes`, so nothing can be deleted without an explicit
decision.

## Install

```
task install        # go install ./cmd/actualds
task build          # build ./actualds in place
task test
task clean          # run the picker from source
```

## Caveat

Docker Desktop stores its VM in a sparse disk image. Pruning frees space inside
the VM, but the host file does not always shrink right away, so the reclaimed
space can take a Docker Desktop restart to appear in `actualds`.

## License

MIT. See [LICENSE](LICENSE).
