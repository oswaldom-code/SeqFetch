<p>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-wordmark-on-dark.svg">
    <img src="assets/logo-wordmark.svg" alt="SeqFetch" width="320">
  </picture>
</p>

# SeqFetch

Command-line tool to download files whose names follow a numeric sequence
(`1.pdf, 2.pdf, ...` or `IMG_0001.jpg, IMG_0002.jpg, ...`). Give it one URL
with a placeholder for the number and it fetches the whole series, in parallel,
until the files run out.

```bash
seqfetch fetch "https://host.com/img/IMG_{n:04}.jpg"
```

## Installation

Prebuilt, dependency-free binaries are published for Linux, macOS and Windows
on amd64 and arm64 with every [release](https://github.com/oswaldom-code/SeqFetch/releases).

### Install script (Linux and macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/oswaldom-code/SeqFetch/main/install.sh | sh
```

The script detects your OS and architecture, downloads the latest release,
verifies its SHA-256 checksum and installs `seqfetch` into `/usr/local/bin`
(or `~/.local/bin` when `/usr/local/bin` is not writable). It accepts a few
environment variables:

| Variable               | Purpose                                   | Example                                 |
|------------------------|-------------------------------------------|-----------------------------------------|
| `SEQFETCH_VERSION`     | install a specific tag instead of latest  | `SEQFETCH_VERSION=v1.2.0 sh install.sh` |
| `SEQFETCH_INSTALL_DIR` | install somewhere else                    | `SEQFETCH_INSTALL_DIR=~/bin sh install.sh` |

### Manual download

1. Open the [releases page](https://github.com/oswaldom-code/SeqFetch/releases/latest)
   and download the archive for your platform:

   | Platform                | File                                  |
   |-------------------------|---------------------------------------|
   | Linux x86_64            | `seqfetch_<version>_linux_amd64.tar.gz`   |
   | Linux ARM64             | `seqfetch_<version>_linux_arm64.tar.gz`   |
   | macOS Intel             | `seqfetch_<version>_darwin_amd64.tar.gz`  |
   | macOS Apple Silicon     | `seqfetch_<version>_darwin_arm64.tar.gz`  |
   | Windows x86_64          | `seqfetch_<version>_windows_amd64.zip`    |
   | Windows ARM64           | `seqfetch_<version>_windows_arm64.zip`    |

2. Optionally verify the download against `checksums.txt` from the same release:

   ```bash
   sha256sum -c --ignore-missing checksums.txt
   ```

3. Extract the archive and put the `seqfetch` binary (`seqfetch.exe` on
   Windows) somewhere on your `PATH`. On macOS, if Gatekeeper blocks it, run
   `xattr -d com.apple.quarantine seqfetch` once.

4. Check it works:

   ```bash
   seqfetch --version
   ```

## Usage

The URL is a template with exactly one placeholder for the sequential part.
Everything else is literal, so purely numeric names and "static prefix +
sequence" names are handled the same way:

| Placeholder | Meaning                      | Template                              | Index 7 renders as |
|-------------|------------------------------|---------------------------------------|--------------------|
| `{n}`       | index as-is                  | `https://host.com/docs/{n}.pdf`       | `7.pdf`            |
| `{n:04}`    | index zero-padded to 4 digits| `https://host.com/img/IMG_{n:04}.jpg` | `IMG_0007.jpg`     |

```bash
seqfetch fetch "https://host.com/docs/{n}.pdf"
seqfetch fetch "https://host.com/img/IMG_{n:04}.jpg" --start 100 --workers 8 --out ~/Pictures
seqfetch fetch "https://host.com/docs/{n}.pdf" --start 123 --backward   # finds 122, 121, ... then 124, 125, ...
seqfetch fetch "https://host.com/docs/{n}.pdf" --limit 2                 # try the template on two files only
```

Flags for `fetch`:

| Flag        | Default | Description                                         |
|-------------|---------|-----------------------------------------------------|
| `--start`   | `1`     | first index of the sequence                         |
| `--workers` | `4`     | concurrent downloads                                |
| `--out`     |         | download directory, overrides the configured one    |
| `--backward`| `false` | also walk down from `--start` to find the first file |
| `--limit`   | `0`     | process at most N indices in total, 0 = unlimited   |

Behaviour:

- Stops at the first index that returns HTTP 404 or 403. With several workers
  a few extra `MISS` lines may appear for requests that were already in flight.
- `--limit N` stops after N indices have been processed (misses and skipped
  files count too), sharing the budget between the backward walk and the
  forward run. Handy to check a template before a long download.
- With `--backward`, before going forward it walks down from `--start` one
  index at a time (`start-1`, `start-2`, ...) downloading what it finds, until
  an index is missing. Use it when you know one file in the middle of the
  sequence but not where it begins.
- Files already present in the download directory are skipped without a
  request, so re-running the same command resumes an interrupted download.
- Non-404 errors (5xx, network) are reported, do not stop the run, and make
  the command exit with status 1.
- `Ctrl+C` cancels the run and removes the partially written file.

## Configuration

The download directory is resolved with this precedence:
`--out` flag > persisted config > current working directory.

```bash
seqfetch config set download-dir ~/Downloads/seq
seqfetch config show
```

The config file lives in the OS user config directory, e.g.
`~/.config/seqfetch/config.yaml` on Linux.

## For developers

Requirements: Go 1.26+, [golangci-lint](https://golangci-lint.run) v2 and
[goreleaser](https://goreleaser.com) v2 for the lint and snapshot targets.

```bash
git clone git@github.com:oswaldom-code/SeqFetch.git
cd SeqFetch
make test       # go test -race ./...  (or: ginkgo -race ./internal/...)
make lint       # golangci-lint run
make build      # stripped binary in ./bin
make install    # stripped binary in $(go env GOPATH)/bin
make snapshot   # cross-compile every release target into ./dist
```

Or install straight from source without cloning:

```bash
go install github.com/oswaldom-code/seqfetch/cmd/seqfetch@latest
```

Layout:

```
cmd/seqfetch/          cobra commands (fetch, config)
internal/pattern/      URL template parsing and rendering, no I/O
internal/config/       YAML config in the user config dir
internal/downloader/   worker pool, backward walk, stop-on-miss logic
install.sh             end-user install script
```

Tests use Ginkgo/Gomega. Every function in a file is defined before it is
first used (callee above caller).

Branching follows git-flow: work lands on `develop` via pull requests and is
merged into `main` for releases. CI (lint, tests, cross-compile) runs on pushes
and pull requests to both branches.

### Releasing

Tag `main` with a semver tag and push it. The release workflow runs the tests,
cross-compiles stripped binaries (`-s -w -trimpath`, `CGO_ENABLED=0`) and
publishes them with `checksums.txt` to the GitHub release, which is what
`install.sh` consumes:

```bash
git checkout main && git merge --no-ff develop
git tag -a v1.0.0 -m "v1.0.0"
git push origin main v1.0.0
```

## License

[MIT](LICENSE)
