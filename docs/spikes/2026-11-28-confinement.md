# Confinement spike record

The spike asks whether macOS can confine a subject CLI and a computation tightly enough for the conformance collection to trust what it observes (CR1 to CR3). It was run on synthetic programs only. This record states what was tested, on which systems, and every limit left open (CR7). The owner decides the gate (CR6).

**Status:** passed for macOS 27.0.1 and 27.2, by the owner's decision on 2026-11-23 (CR6, KTD5). The suite passes on macOS 27.2 and on GitHub's macOS 27.0.1 image. On macOS 26.7.2 it fails closed, because that version rejects the profile, so macOS 26 is unsupported and falls under CR23.

## Systems tested

| System | macOS | Architecture | Go | Compiler | Result |
|---|---|---|---|---|---|
| Developer machine | 27.2 (26B123) | arm64, 18 cores | 1.27.4 | Apple clang 21.0.0 | passes: 1,956 of 1,956 scenarios, 25 of 25 runs each |
| GitHub runner `macos-26`, image `macos26` 20261117.0516.1 | 26.7.2 (25G332) | arm64 | 1.27.4 | Apple clang 21.0.0 | fails closed: the profile does not parse |
| GitHub runner `xcode-27` (preview), image 20261104.0337.1 | 27.0.1 (26A434) | arm64 | 1.27.4 | Apple clang 21.0.0 | passes: 1,956 of 1,956 scenarios, 25 of 25 runs each, in 77 minutes |

**macOS 26.7.2** does not know four operation names the profile uses: `fs-info`, `asr-parser-enter`, `coalition-info` and `isp-command-send`. `sandbox-exec` rejects the profile with exit 65 before any program starts. The harness counts that as a confinement that could not be applied, never as a subject's exit, so every confined clean program and violator failed. The checks that expect a broken profile to fail passed there for the wrong reason: 106 allowance-removal checks, the self-test's whole-kill mutations and the application test's without-kill form, because a profile that never started the launcher fails every program. Each of these now requires that sandbox-exec applied the profile, and treats a run where it did not as not evaluated. The profile's vocabulary belongs to one macOS version. A profile for macOS 26 would need its own operation list and its own run of this suite. On that runner's volume a read also did not move the access time, so the read proof could not be measured there.

**macOS 27.0.1** parses the profile and starts every clean program. A full run on that three-core runner took 81 minutes. Of 1,956 scenarios, 1,953 were correct in 25 of 25 runs ([run](https://github.com/vinnyp/press-quality-lab/actions/runs/45128403814)). Three Go clean-program scenarios were each correct in 24 of 25. Each Go run took about 2.6 seconds there, against a 5-second budget that only bounds a hung run, so a timeout is the likely cause, but that run did not log it. An earlier run there also failed the resolver smoke test in 1 or 2 of 25 runs, because a lookup can wait on slow DNS. That test now has a fixed 20-second budget, which cannot let a confined row pass, because a kill must not be a timeout. The liveness budgets that failed on the runner were then made scalable through `CONFINEMENT_BUDGET_SCALE`, and runs now log a timeout. A rerun with the scale at four fixed the Go scenarios. It then showed that the compiler helper and the 250 ms group-cleanup budget were also too short on that runner. Once those were scaled too, a full run passed every scenario. The scale changes no budget a scenario measures. The group-cleanup scenario's child now lives 10 seconds, and the scenario fails as not measured if its check finishes after that, so only the group kill can end the child in time.

Runner jobs:
- macOS 27.0.1 full pass: https://github.com/vinnyp/press-quality-lab/actions/runs/45152589899
- macOS 27.0.1 negative control, with the link kill removed: failed on both link scenarios and the structural check, as it should. https://github.com/vinnyp/press-quality-lab/actions/runs/45164671093
- macOS 26.7.2, the profile rejected: https://github.com/vinnyp/press-quality-lab/actions/runs/45115715933
- macOS 26.7.2, the startup diagnosis that named the rejected lines: https://github.com/vinnyp/press-quality-lab/actions/runs/45121203539

**Linux** is unsupported in version 1.0 (CR5):
- a seccomp filter cannot decide by path, because it never sees the path a pointer argument names;
- Landlock refuses an operation without ending the process;
- a ptrace supervisor was not attempted.

Support is per version (KTD5): version 1.0 supports macOS 27.0.1 and 27.2, the versions this record names as passed. Every other system, a later 27 release included, falls under CR23 until this record names it as passed.

## Method

- One template renders two profiles, one for a subject and one for a computation. Each starts from `(deny default (with send-signal SIGKILL))`. Every path goes in as a `-D` parameter, never as profile text (CR78).
- A trusted launcher writes a marker before it starts the program. A profile that cannot be applied is therefore told apart from the program's own exit.
- **Violators** attempt one refused operation each. A violator passes only when its wait status is SIGKILL, it printed nothing after a readiness marker written just before the operation, and its decoy is unchanged. A violator's targets are decoys only: files in a temporary root, a victim process the test started, loopback or `.invalid` names, and random canaries (KTD20).
- **Controls around each violator:**
  - a twin that differs only in that operation;
  - an unconfined positive control, showing the operation really works;
  - where it can be isolated, an attribution form that grants exactly that operation, plus stated setup grants, and shows the violator then performs. Tests that cannot isolate their rule are named smoke tests and say so.
- **Clean programs** pass only on exact output and exit status. They cover C, Go, a shell pipeline and SQLite.
- **Self-test:** it runs one program for each kind of refused operation under each profile. Each of its modes is individually necessary: granting only that mode's operation makes it fail.
- **Allowances:**
  - each one exists because a named clean program fails without it, and a test removes each allowance and shows that program fail;
  - every allow line, and every fixed system path parameter, is pinned to a reviewed literal list.
- Every scenario runs 25 times, and a run with no scenario fails.

## Results against CR1 to CR3

| Requirement | macOS 27.2, developer machine | macOS 27.0.1, runner |
|---|---|---|
| CR1. A run with no attempt is verified as such, and one with an attempt, or one that cannot be verified, never is | holds | holds |
| CR2. Swallowed errors, DNS, child processes, image replacement, local sockets and delegation to system daemons | holds, with the limits below | holds, with the limits below |
| CR3. File and computation confinement (CR16, CR17, CR35 to CR37, CR62 to CR64, CR67, CR78, CR80 to CR82) and the CR31 read proof | holds, with the limits below; the read proof holds against every forgery tested | holds, with the limits below |

### Rules a test caught when broken by hand

Each of these was broken in a scratch copy, and a named test failed:
- each deny and each kill;
- the planted-file rule, narrowed back to `file-write-times`;
- the `sysctl-read` refusal;
- each input of the read proof (calibration, access time, control kill);
- widened system path parameters and widened grants;
- the computation's session and group kill;
- network, file-read and seed-write grants;
- keeping the terminal.

## Allowances

The two profiles share these grants. Each names the clean program that fails without it.

| Grant | Needed by |
|---|---|
| exec and read of the launcher and the program, and a read of each one's private directory | every program (macOS starts no program without a read of its directory, CR112) |
| `file-read*` on `/` as a literal, `file-read-data` under the Preboot cryptex path, global `file-read-metadata` | every program's loader |
| `/dev/dtracehelper` read, write and ioctl; the host port; own code signing; own task special port | every program's startup |
| reads of `/dev/null`, `/dev/urandom` and `/dev/zero`; writes to `/dev/null`, stdout and stderr | the device and stream programs |
| reads of the local time zone file and the POSIX rules file | the time zone program and the Go runtime |
| fifteen named sysctls | the Go runtime (CPU features, page size, OS version, limits, lockdown state) |
| `kern.hostname` | the identity program |
| lookups of the directory, notification and log services, and the notification shared memory | a user lookup |
| membership lookup, `system-info`, volume information, file locks in the home | SQLite writes |
| own process information | the self-information program |
| read and write of the throwaway home | the home programs |

The computation profile adds:
- a fork, and signals within its own sandbox;
- process information within its own sandbox;
- exec and read under `/bin` and `/usr/bin`;
- `/dev/tty` read and write, which is harmless because the harness gives no terminal;
- reads of its declared files and the seed;
- `kern.ngroups`, `hw.memsize` and `hw.pagesize`, needed by `id` and `sort` in a pipeline.

## Refusals checked by profile text only

These operations are refused with a kill, but no harmless attempt could witness them. A closed list in the tests names each one, so an operation outside the list with no witness fails the suite.

| Operation | Reason |
|---|---|
| `system-memorystatus-control` | the own-process getters return EPERM even with no confinement, so no positive control exists |
| `process-info-sandbox-container` | the container-path lookup returns nothing even for an unconfined victim |
| `mach-task-special-port*` | the host-special-port read is not stopped by this rule, so it cannot witness it |
| `coalition-info` | the own-coalition query never succeeds, even unconfined |
| `iokit-open-service` | opening a user client may drive hardware, so it was never attempted |
| `nvram*`, `fs-quota*`, `fs-snapshot-mount`, `file-graft`, `file-ungraft`, `system-privilege`, `mach-bootstrap`, `mach-cross-domain-lookup`, `isp-command-send`, `asr-parser-enter`, `darwin-notification-post`, `system-necp-client-action` | machine-wide or destructive, never attempted |
| `exception-entitlement`, `process-exec-update-label` | refused without a kill, because macOS rejects a kill modifier on them (CR67) |

## Limits left open

- **Descendants (CR35):** a child that leaves the process group through `posix_spawn` with `SETSID` or `SETPGROUP` outlives the group's end. It stays confined.
- **Read proof (CR31):**
  - another process reading the planted file during the invocation looks like the subject's read;
  - a zero-byte read at end of file moved the access time in 25 of 25 runs, on this machine and on the macOS 27 runner. Under CR31 as written, that would count as a read although no content was read. CR31 states this as a limit. The spike logs it and asserts nothing.
- **Here-documents and here-strings (CR64):** `/bin/sh` creates their temporary files in the system temporary directories, whatever `TMPDIR` says.
- **Metadata:** global file metadata is visible, so a program can learn that any file exists and its size.
- **Implicit classes:** some operation classes stay open implicitly, because refusing one kills startup before the launcher's marker:
  - dynamic code generation;
  - file existence tests;
  - `sandbox-check`;
  - Mach, MIG and Unix system calls;
  - `fcntl`;
  - MAC system calls.

  Their scope is unmeasured.
- **Allowed services:** their protocols are untested.
  - `com.apple.logd` lets a program write to the unified log, outside its home, where a leak scan of streams and home does not look.
  - On a Mac bound to a directory server, user and group lookups can become network queries.
  - Notification state is shared with other processes.
- **Silent refusals (CR67):** a setuid or setgid tool fails to execute with EPERM and no kill.
- **Shared directories:** where the shell probes a shared directory for write access, permitting the probe would also permit removing the directory, so the probe is refused.
- **Other copies of the subject's binary (CR36):** a byte-identical copy at a path the engine does not know escapes the computation's refusal.
- **System information:** `system-info` is granted whole for SQLite.
- **Exit detail:** the system kills a program whose signed code page is corrupted, but reports no exit detail for it. The harness's filter for memory and code-signing kills is therefore unwitnessed, and such a kill after a violator's readiness marker would look like a refusal kill.
- **Private copy:** untested on signed and Homebrew-installed binaries.
- **Nested planted files:** only a planted file directly in the home was tested. The refusal of unlink and create on each directory down to a deeper planted file is required by CR31 but untested.
- **Operation catalog:** the inventory queries a fixed catalog of operation names taken from this macOS version. A later version may add operations it does not know.

## Findings for the specification

- **Refusing by default does not refuse everything.** Queried with `sandbox_check`, 64 of about 170 operations stayed allowed under `(deny default)`. `sysctl-read` was one of them, so it needs its own explicit deny. Every refusal the specification relies on must be stated explicitly and witnessed.
- **`sandbox_check` can kill the process it queries** when that process has a matching kill rule. Query only the probe's own process, without reporting.
- **Kill-on-refusal makes startup fragile.** Refusals a process normally survives become fatal. The minimal allowances were found by allowing every class and then removing one at a time, never by adding one at a time.
- **A new executable file costs about 20 ms at its first launch**, because the system assesses it. Copying the program for every run made the system's assessment service fall behind under parallel load, and launches stalled past their budget. Make the CR112 copy once per grade and check it unchanged before each reuse.
- **The access time is not proof of a read on its own.** Under the home's write grant, `setattrlist`, a rename or replace around the file, and unlink and recreate each moved it with no read. Killing every write to the planted file closes these. A credential file deeper than the home's top level also needs every directory above it protected from unlink and create.
- **Running as root.** The profile grants the privileged host port (`mach-priv-host-port`) unconditionally, and that port is inert only for an ordinary user, so the engine refuses to run as root (CR115).
- **Exit observation:** registering for a child's exit after it has exited fails with ESRCH. Because the child is not yet reaped, that can only mean it already exited.
- **The profile is tied to a macOS version.** An operation name one version does not know makes the whole profile unparsable, and the confinement then cannot be applied at all. The engine needs one reviewed profile per supported version, and a check that the version it runs on is one of them.
- **Some volumes do not record a read's access time.** On the macOS 26 runner's volume a read did not move it. The CR31 calibration then makes the read proof not-measured, which the suite reports as a failure, never as a pass.

## Owner decisions, 2026-11-23

1. **Supported systems:** in version 1.0, the macOS 27 versions this record names as passed. The owner confirmed that a later 27 release is unsupported until this record names it. A profile for macOS 26 was not built.
2. **The planted file:** any write to it is refused with a kill, and so is any unlink or create on the home and on every directory below it on the file's path (CR31). A CLI that rewrites its credential file during the probe is therefore not observed.
3. **A zero-byte read at end of file:** CR31 states it as a limit. Such a read moves the access time and counts as a read.
4. **Further text:**
   - CR84 states that a leak through the unified log is not seen, and that a directory lookup can reach a server.
   - CR67 names a computation's setuid and setgid exec as a silent refusal.
   - CR115 makes the engine refuse to run as root.
