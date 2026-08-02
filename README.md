# actualds

A command line tool that reports free space on the drives you can run out of
space on. It also deletes regenerable cache files to reclaim space.

## Why

Mac drives are small and they fill up. `df -h` reports every mounted
filesystem, which includes `devfs`, `map auto_home`, mounted ISOs, read-only
disk images, and several APFS system volumes. That output is hard to read when
you only want to know how much room is left on your boot drive.

`actualds` reports the volumes you can fill up. It keeps a mount when the mount
is writable, is backed by a device under `/dev/`, and is mounted at
`/System/Volumes/Data` or under `/Volumes`. It drops everything else.

On current macOS versions, `/` is a sealed read-only snapshot. The volume that
fills up is `/System/Volumes/Data`. `actualds` labels that volume
`Macintosh HD`.

## Install

```
task install        # go install ./cmd/actualds
```

Other tasks:

```
task build          # build ./actualds in place
task test           # run the unit tests
task run            # run the report from source
task clean          # run the picker from source
```

## Usage

```
actualds                    Report free space
actualds clean              Select categories to delete, then delete them
actualds clean --dry-run    Print the table and exit
actualds clean --yes        Delete every category without a prompt
```

## Report

```
$ actualds
VOLUME                         SIZE       USED       FREE  FREE%
Macintosh HD               494.4 GB   389.4 GB   105.0 GB  21.2%
Dock (non-Backups)           2.0 TB   254.3 GB     1.7 TB  87.3%
Dock                       257.0 GB   246.3 GB    10.7 GB   4.2%
```

The FREE column matches `df`. The USED column does not. `df` counts only the
blocks of the Data volume. `actualds` counts everything in the shared APFS
container that is not free, which includes the sealed system volume, swap, and
snapshots.

The report reads mounts with `syscall.Getfsstat`. The project has no
third-party dependencies.

## Cleaning up space

`actualds clean` measures each category and opens a list:

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

Every category starts selected. Arrow keys or `j` and `k` move the cursor.
Space toggles a row. `a` selects all rows and `n` clears all rows. Enter runs
the selected rows. `q`, Esc, and Ctrl-C exit without deleting anything.

`actualds` skips a category when its tool is not installed or when the category
has nothing to reclaim. The Docker rows do not appear when the Docker daemon is
not running.

After a run, `actualds` reports the space it freed. It measures that number
from a `statfs` delta, not from the sum of the estimates.

When stdin is not a terminal, `actualds` skips the list, prints the table, and
exits without deleting anything.

### How the deletion works

Each category runs the prune command of its own tool. `actualds` does not
delete paths itself, except for the Xcode `DerivedData` directory. Tools such
as `docker`, `go`, and `brew` know which of their files are still in use.

The command in the list is the argument list that `actualds` passes to `exec`.
The preview and the executed command cannot differ.

### What it does not delete

- All of `~/Library/Caches`. Some applications store real data there. Spotify
  stores offline music and Chrome stores profile data. Deleting a cache while
  its application runs can also corrupt state. `actualds` deletes named
  subdirectories instead.
- APFS local snapshots. macOS deletes these when disk space runs low.
- `/private/var/folders`. macOS manages this directory and running processes
  hold files open in it.
- `~/Library/Application Support` and MobileSync backups. These directories
  hold data, not cache.
- The Trash. Empty it from Finder.

## Docker note

Docker Desktop stores its virtual machine in a sparse disk image. A prune frees
space inside the virtual machine, but the image file on the host does not
always shrink right away. The reclaimed space can require a Docker Desktop
restart before `actualds` reports it.

## License

MIT. See [LICENSE](LICENSE).
