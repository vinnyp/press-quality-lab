# Confinement

**What this file covers.** How the engine confines the subject and each computation on macOS, how it tells a refusal from the subject's own result, and which limits remain. The rules are family QH. They rest on the confinement spike recorded in `docs/spikes/2026-11-28-confinement.md`, which tested these rules on synthetic programs and on one real printed CLI. The refusals that no harmless attempt could witness are named in §8. Nothing here depends on the spike's code: the template below is complete, and its comments say what each line is for. How to read the rules, and what binds, is as `collection-rules.md` says.

## 1. Where the engine may grade

**Frozen.** These rules were frozen on 2026-11-23, when the spike's whole suite passed on both systems QH1 names under the profile below. A change to what any rule here requires is a new specification version (QE1); an editorial correction goes in the errata note.

- **QH1. Supported systems.** The engine grades only on a macOS version this specification names as passed: macOS 27.0.1 and macOS 27.2. On any other system, a later 27 release included, every check that runs the subject is not observed with `network_unverifiable` and is never red (QO1). A version is added by a new specification version whose spike record names it as passed.
- **QH2. One profile per version.** The profile in §3 belongs to the versions QH1 names. A macOS version that does not know an operation the profile names rejects the whole profile, and the confinement then cannot be applied at all (QH6). The engine checks the running version against QH1 before any check runs.
- **QH3. Not as root.** The engine refuses to run when its effective user is root, before any check, with exit code 70 (QE16). The profile grants the privileged host port, which is harmless only to an ordinary user.

## 2. How a program is started

- **QH4. A trusted launcher.** Every confined program, the subject's binary and each computation's executable alike, is started by `sandbox-exec` running the engine's own trusted launcher, which then replaces itself with the program, keeping one process id.
- **QH5. The launcher's marker.** Before it replaces itself, the launcher writes a marker line to standard error: a fixed prefix followed by a random value the engine chose for that invocation. Only a first line of standard error that equals the expected marker counts as the marker, and the engine strips exactly that line before any check reads the stream. A subject that prints the prefix itself cannot know the random value.
- **QH6. Confinement not applied.** The confinement was not applied when no marker was written and either `sandbox-exec` itself exited, or the launcher was ended by `SIGKILL` before writing its marker. Examples of the first: exit 65 when it rejects the profile, or exit 71 when the engine already runs inside a sandbox that forbids nesting. The engine then never runs the program by another route, and every check that needed it is not observed with `network_unverifiable` (QO1).
- **QH7. Could not start.** When the launcher cannot replace itself with the program, it writes a second, distinct marker and exits. The engine reads that as the binary not starting (QO2), never as the program's own exit, even when the program would have exited with the same code.
- **QH8. The private copy.** The subject's binary, and each computation's executable, runs from a byte-identical copy in a throwaway directory that holds nothing else, because macOS starts no program unless it may read the directory that holds it.
  - The engine makes the copy once per grade and checks it unchanged before each use: the directory's entries, and the copy's inode, mode, size, modification time, change time and digest. A copy that changed fails the grade's invocations that would use it with `harness_fault`. It is never silently remade.
  - A new executable file is assessed by the system at its first launch, so making a copy per invocation slows every launch and can stall them under load.
  - The specification states the limit that a program sees its copy's path, and finds nothing installed beside the original.

## 3. The self-test

- **QH9. Before any check.** Before any check runs the subject, the grade runs self-test programs under the grade's own confinement, each taking the subject's place in the profile. The computation's confinement is self-tested the same way, and neither is ever bypassed.
  - Each kind of refused operation has a program that attempts it and must be killed: a TCP connect, a Unix-socket connect and a UDP send to loopback decoys; a bind and a listen; a read, a mapping, a write and a create outside the home; a refused service lookup; a read of a refused system value; and each route to another process: its arguments, its environment, its process record, a signal, its code signature, its scheduling priority and its resource use. For the subject, a child start must be killed. For a computation, a child must run, and `setsid` and `setpgid` must be killed.
  - Each of these programs is necessary: a profile that grants exactly one of these operations fails the self-test. The confinement spike's record, `docs/spikes/2026-11-28-confinement.md`, showed this once for every program, on the spike code that the main branch holds from commit `5aa554a`, on which the confinement rules froze. A grade runs the programs but does not repeat that demonstration.
  - A clean program must finish with its known output.
  - The self-test also shows that the volume records a read's access time (QH26).
  - When any self-test program does not behave as stated, every check that runs the subject is not observed with `network_unverifiable` (QO1).

## 4. The profile

- **QH10. Kill on refusal.** The profile starts from `(deny default (with send-signal SIGKILL))`. Refusing by default does not refuse every operation, so every operation the engine relies on refusing is also listed by name, with a kill.
- **QH11. A fixed template.** The profile is the fixed template below, the same for every subject. Every path reaches it as a parameter (`-D NAME=value`, read by `(param "NAME")`), canonical and absolute, and never as profile text. A path holding profile syntax therefore grants nothing.

  The template, with `{{PROCESS_TARGET}}`, `{{PROCESS_RULES}}` and `{{FILE_RULES}}` filled as the table after it says. The `;` lines are comments; each allowance's comment names what needs it. Lines are emitted in the order written: when two rules match one operation, the later rule decides, so each refusal that narrows an allowance comes after it.

```
(version 1)
(deny default (with send-signal SIGKILL))
(deny sysctl-read (with send-signal SIGKILL))
; Startup of the launcher or the program needs implicit dynamic-code-generation,
; file-test-existence, sandbox-check, syscall-mach, syscall-mig, syscall-unix,
; system-fcntl and system-mac-syscall: refusing one kills before the launcher's
; marker, so the witness shows startup needs it, not that the program alone does.
; A user lookup needs implicit mach-derive-port. Their scope is an open limit.
; Any program: file-test-existence remains visible with global file metadata.
; exception-entitlement and process-exec-update-label reject kill modifiers;
; both are refused silently.
(deny exception-entitlement)
(deny process-exec-update-label)
(deny fs-quota* (with send-signal SIGKILL))
(deny nvram* (with send-signal SIGKILL))
(deny process-iopolicy* (with send-signal SIGKILL))
(deny mach-task-special-port* (with send-signal SIGKILL))
; Any program: the loader sets its own task's bootstrap port.
(allow mach-task-special-port-set (target {{PROCESS_TARGET}}))
; Any program: pre-exec code validation needs the modern codesigning class.
; Foreign status queries are separately refused by the legacy target filters.
(deny asr-parser-enter (with send-signal SIGKILL))
(deny coalition-info (with send-signal SIGKILL))
(deny consume-extension (with send-signal SIGKILL))
(deny darwin-notification-post (with send-signal SIGKILL))
(deny file-clone (with send-signal SIGKILL))
(deny file-graft (with send-signal SIGKILL))
(deny file-link (with send-signal SIGKILL))
(deny file-lock (with send-signal SIGKILL))
(deny file-map-executable (with send-signal SIGKILL))
(deny file-read-xattr (with send-signal SIGKILL))
(deny file-ungraft (with send-signal SIGKILL))
; SQLite: statfs uses its home volume, not its directory.
(deny fs-info (with send-signal SIGKILL))
(allow fs-info (literal (param "VOLUME")))
; SQLite: locks on its own files.
(allow file-lock (subpath (param "HOME")))
(deny fs-snapshot-mount (with send-signal SIGKILL))
(deny iokit-get-properties (with send-signal SIGKILL))
(deny iokit-open-service (with send-signal SIGKILL))
(deny isp-command-send (with send-signal SIGKILL))
(deny mach-bootstrap (with send-signal SIGKILL))
(deny mach-cross-domain-lookup (with send-signal SIGKILL))
; A user lookup: directory lookup needs derived Mach reply ports; narrowing remains open.
(deny mach-task-exception-port-set (with send-signal SIGKILL))
(deny necp-client-open (with send-signal SIGKILL))
(deny socket-ioctl (with send-signal SIGKILL))
(deny socket-option-get (with send-signal SIGKILL))
(deny socket-option-set (with send-signal SIGKILL))
(deny system-memorystatus-control (with send-signal SIGKILL))
(deny system-necp-client-action (with send-signal SIGKILL))
(deny system-privilege (with send-signal SIGKILL))
(deny process-info-codesignature (with send-signal SIGKILL))
(deny process-info-sandbox-container (with send-signal SIGKILL))
; Any program: dynamic loader and C runtime startup.
(allow process-exec* (literal (param "LAUNCHER")) (literal (param "BIN")))
; Any program: dynamic loader host setup.
(allow mach-priv-host-port)
; Any program, and Go: loader, hostname and CPU-feature queries.
(allow sysctl-read (sysctl-name "kern.hostname") (sysctl-name "kern.maxfilesperproc") (sysctl-name "kern.osproductversion") (sysctl-name "kern.iossupportversion") (sysctl-name "kern.bootargs") (sysctl-name "hw.ncpu") (sysctl-name "hw.optional.arm.FEAT_SB") (sysctl-name "hw.optional.arm.FEAT_DIT") (sysctl-name "security.mac.lockdown_mode_state") (sysctl-name "kern.osvariant_status") (sysctl-name "hw.pagesize_compat") (sysctl-name "hw.optional.armv8_crc32") (sysctl-name "hw.optional.armv8_1_atomics") (sysctl-name "hw.optional.armv8_2_sha512") (sysctl-name "hw.optional.armv8_2_sha3"))
; Any program: dynamic loader validates its own code.
(allow process-codesigning* (target {{PROCESS_TARGET}}))
; Any program: dynamic loader path resolution; global visibility is an open limit.
(allow file-read-metadata)
; Any program: dynamic loader trace-device ioctl.
(allow file-ioctl (literal (param "TRACE_DEVICE")))
; Any program: macOS needs the private executable directories for startup.
(allow file-read-data (literal (param "BIN_PARENT")) (literal (param "LAUNCHER_PARENT")))
; Ordinary programs: null streams, randomness and zero-filled input.
(allow file-read-data file-write-data (literal (param "NULL_DEVICE")))
(allow file-read-data (literal (param "RANDOM_DEVICE")) (literal (param "ZERO_DEVICE")))
; Ordinary programs: reopen the captured stdout and stderr descriptors.
(allow file-write-data (literal (param "STDOUT_DEVICE")) (literal (param "STDERR_DEVICE")))
; Go: load the declared named time zone from the system database.
(allow file-read-data (literal (param "ZONEFILE")))
; The C library: libc also reads the system POSIX transition rules.
(allow file-read-data (literal (param "ZONE_RULES")))
; Go's resolver: Go resolves named services and protocols.
(allow file-read-data (literal (param "SERVICES")) (literal (param "PROTOCOLS")))
; Any program: executable reads.
(allow file-read-data (literal (param "BIN")) (literal (param "LAUNCHER")))
; Any program: loader root lookup.
(allow file-read* (literal (param "ROOT")))
; Any program: runtime image on the sealed preboot volume.
(allow file-read-data (subpath (param "RUNTIME")))
; Any program: dynamic loader trace-device read.
(allow file-read-data (literal (param "TRACE_DEVICE")))
; Any program: dynamic loader trace setup.
(allow file-write-data (literal (param "TRACE_DEVICE")))
{{PROCESS_RULES}}
; A user lookup: directory and notification lookups used by getpwuid.
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.libinfo"))
; A user lookup: runtime notification registration.
(allow mach-lookup (global-name "com.apple.system.notification_center"))
; A user lookup: runtime logging setup.
(allow mach-lookup (global-name "com.apple.logd"))
; A user lookup: notification shared-memory registration.
(allow ipc-posix-shm-read* (ipc-posix-name "apple.shm.notification_center"))
(deny process-info* (with send-signal SIGKILL))
; A program's own information: runtime and own-process queries.
(allow process-info* (target {{PROCESS_TARGET}}))

{{FILE_RULES}}

; SQLite: SQLite membership lookup.
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.membership"))

; SQLite: SQLite system information cannot be narrowed by type (open limit).
(allow system-info)
```

  | Slot | Subject | Computation |
  |---|---|---|
  | `{{PROCESS_TARGET}}` | `self` | `same-sandbox` |
  | `{{PROCESS_RULES}}` | empty | the computation block below |
  | `{{FILE_RULES}}` | the subject file block below | the computation file block below |

  Computation block:

```
(allow process-fork)
(allow signal (target same-sandbox))
(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))
```

  Subject file block:

```
(allow file-read* (subpath (param "HOME")))
(allow file-write* (subpath (param "HOME")))
(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))
(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))
(deny file-write-unlink file-write-create (literal (param "PLANTED_DIR_n")) (with send-signal SIGKILL))
```

  The last line appears once for each directory between the home and the planted file: `PLANTED_DIR_1` is the home's child on the file's path, and the last is the file's own directory. A planted file directly in the home has no such line.

  For the read proof's control invocation (QH27), the subject file block ends with one more line:

```
(deny file-read-data (literal (param "PLANTED")) (with send-signal SIGKILL))
```

  Computation file block, with one `DECLARED_n` line for each declared file and one `SEED_n` line for each file in the seed:

```
(allow sysctl-read (sysctl-name "kern.ngroups"))
(allow sysctl-read (sysctl-name "hw.memsize") (sysctl-name "hw.pagesize"))
(allow process-exec* file-read-data (subpath (param "BIN_DIR")) (subpath (param "USR_BIN_DIR")))
(allow file-read-data file-write-data (literal (param "TTY_DEVICE")))
(allow file-read-data (literal (param "DECLARED_n")))
(allow file-read-data (literal (param "SEED_n")))
(allow file-read* (subpath (param "HOME")))
(allow file-write* (subpath (param "HOME")))
(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))
(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))
(deny file-read-data process-exec* (literal (param "SUBJECT")) (literal (param "SUBJECT_COPY")) (with send-signal SIGKILL))
```

  For a computation, `HOME` is its scratch directory and `PLANTED` names no file.

  Parameters, each a canonical absolute path with every link resolved:

  | Parameter | Value |
  |---|---|
  | `ROOT` | `/` |
  | `RUNTIME` | `/System/Volumes/Preboot/Cryptexes` |
  | `VOLUME` | the mount point of the volume holding the throwaway home |
  | `HOME` | the throwaway home, or a computation's scratch directory |
  | `LAUNCHER`, `BIN` | the private copies of the launcher and of the program (QH8) |
  | `LAUNCHER_PARENT`, `BIN_PARENT` | the private directories holding them |
  | `TRACE_DEVICE` | `/dev/dtracehelper` |
  | `NULL_DEVICE`, `RANDOM_DEVICE`, `ZERO_DEVICE` | `/dev/null`, `/dev/urandom`, `/dev/zero` |
  | `STDOUT_DEVICE`, `STDERR_DEVICE` | `/dev/fd/1`, `/dev/fd/2` |
  | `ZONEFILE` | the system's zone file for `UTC`, resolved |
  | `ZONE_RULES` | `/usr/share/zoneinfo/posixrules`, resolved |
  | `SERVICES`, `PROTOCOLS` | `/private/etc/services`, `/private/etc/protocols` |
  | `PLANTED` | the planted secret's file in the throwaway home, or a path that names no file when the probe does not run |
  | `PLANTED_DIR_n` | each directory between the home and the planted file, as the subject file block says |
  | `BIN_DIR`, `USR_BIN_DIR` | `/bin`, `/usr/bin` (computation only) |
  | `TTY_DEVICE` | `/dev/tty` (computation only) |
  | `DECLARED_n`, `SEED_n` | each declared file, and each file of the pristine seed (computation only) |
  | `SUBJECT`, `SUBJECT_COPY` | the subject's binary and its private copy (computation only) |

- **QH12. The environment.** The engine builds the environment of the subject, and of each computation, from this allow-list and the subject file's literal declarations (QS7), and passes nothing else through:

  | Variable | Value |
  |---|---|
  | `HOME` | the throwaway home |
  | `TMPDIR` | a directory inside the throwaway home |
  | `PATH` | `/usr/bin:/bin` |
  | `LANG`, `LC_ALL` | `C` |
  | `TZ` | `UTC` |
  | `__CF_USER_TEXT_ENCODING` | `0x<uid>:0x0:0x0`, where `<uid>` is the engine's real user id in uppercase hexadecimal with no leading zeros |

  The last line keeps the system framework that Go programs load for TLS from reading a preference file in the user's real home directory, which it finds without `HOME`. The framework uses the variable only when its user id matches the process's, so a fixed value works for one user alone.

  `TMPDIR` is `<home>/.pql-tmp/`, which the engine creates empty in every home.

  A declaration rejects the subject file when it sets any variable in this table, or any variable whose name begins `DYLD_` or `__CF_`.
- **QH13. Descriptors and terminal.** A confined program starts in a new session with no controlling terminal, with standard input reading `/dev/null`, and with no descriptor open beyond its three standard streams.
- **QH14. The throwaway home.** Each runbook, and each invocation of a probe, gets a fresh throwaway home holding a copy of the seed. A runbook's steps share its home.
- **QH15. Allowed paths, services and metadata.** What the profile allows, as the template states it, and nothing else:
  - **Startup of any program:** executing and reading its own private copy and the launcher's, and reading their two private directories; reading `/` as a literal and the system runtime under `RUNTIME`; reading every file's metadata; the trace device; the privileged host port; its own code signing and task port; a fixed list of system values the Go and C runtimes read.
  - **Devices and streams:** reading and writing `/dev/null`, reading `/dev/urandom` and `/dev/zero`, and writing its own standard output and error.
  - **System files:** the `UTC` zone file and the POSIX zone rules, and `/private/etc/services` and `/private/etc/protocols`, which Go's resolver reads for a service or protocol name.
  - **Services:** looking up `com.apple.system.opendirectoryd.libinfo`, `com.apple.system.opendirectoryd.membership`, `com.apple.system.notification_center` and `com.apple.logd`, and reading the notification centre's shared memory. Every other service name, a sibling or an unregistered name under one of these included, is refused with a kill. Each allowed service's own protocol is not examined.
  - **Its own state:** reading and writing inside its throwaway home, locking files there, querying the home's volume, and querying its own process (for a computation, processes in its own sandbox).
  - **System information:** `system-info` in full, because SQLite needs it and it cannot be narrowed.
  - **Visible metadata:** global metadata reads mean any program can learn that any file exists, its size and its times, though not its contents.

## 5. The computation's confinement

- **QH16. A computation's own profile.** A computation runs under the same template with the computation's variant, which differs only as follows:
  - it may start child processes, and may signal and read process information of processes in its own sandbox;
  - it may run and read the system's tools under `/bin` and `/usr/bin`;
  - it may read its executable, its declared files and the pristine seed, and it may write only in its own scratch directory, never in the seed;
  - it may use `/dev/tty` read and write, which reaches nothing because it has no controlling terminal (QH13).
- **QH17. The subject's binary.** A computation cannot run or read the subject's binary or its private copy, and no file it is given has the same content: QS12 rejects such a subject file. The specification states the limit that a copy at any other path escapes this.
- **QH18. Sessions and groups.** A computation is killed when it calls `setsid` or `setpgid`. When it ends, its whole process group is ended with it.
- **QH19. A child that leaves.** A child started through `posix_spawn` with a new session or process group leaves the group and outlives its ending. It stays inside the computation's confinement. The specification states this limit.
- **QH20. No network.** A computation has no network, as the subject does, and the same rules decide a kill (QH21).

## 6. Seeing what happened

- **QH21. A kill by the confinement.** An invocation ended by the confinement when its process was ended by `SIGKILL` after the launcher's marker, not by the engine, not at its time budget, and not with an exit detail from the system that names a memory or code-signing kill. The engine observes the exit with a kernel event queue filter for the process's exit (`kqueue` with `EVFILT_PROC`, `NOTE_EXIT` and `NOTE_EXIT_DETAIL`), without reaping the process first, so its id cannot be reused before its group is ended. Registering that filter after the process has already exited fails with `ESRCH`, which means it exited. A wait the system interrupts is retried.
  - The specification states the limit that the system reports no exit detail for a code-signing kill, so such a kill after the marker looks like the confinement's.
- **QH22. Refusals a program survives.** Some refusals a process normally survives become kills under this profile. A program the profile refuses at startup is killed before the launcher's marker can reach the program's own code; QH6 or QH21 then decide what that means.
- **QH23. Ending a group.** When an invocation ends, the engine ends the invocation's whole process group.
- **QH24. The known-fast control.** Before the subject's first check, the engine runs its own launcher five times under the subject's confinement, in a mode that writes the marker and exits 0 without starting a program. The control kept within its bound when the slowest of the five finished within 2 seconds. QO4 reads its result.
- **QH25. Inspecting a used home.** The engine inspects a used home without following a link, opening a special file or reading past a bound, and reports a link or special file the subject created as a change (QP9).

## 7. The read proof for the planted secret

- **QH26. Calibration.** Before a planted-secret probe, the self-test plants a decoy, sets its access time a day before its modification time, reads it under the subject's confinement, and checks that the access time moved. When it did not, the volume does not record reads, so no invocation of any planted-secret probe in the grade proves a read (QP11): QO7's earlier steps still decide a leak, an invocation QO1 to QO4 decide, or a scan bound, and otherwise the probe is not observed with `input_missing`.
- **QH27. The two signals.** For the planted file the engine:
  - sets the access time a day before the modification time before each invocation;
  - refuses, with a kill, any write to the planted file and any unlink or create on the throwaway home and on every directory below it on the file's path, so the subject cannot set the access time by writing, renaming or replacing the file;
  - records the file's device and inode before and after;
  - when QO7's condition holds, runs an otherwise identical control invocation whose profile also kills on opening the planted file for reading, with the extra line of QH11.
  The read is proved when the ordinary invocation finished with its declared successful exit, the access time moved, the device and inode are unchanged, and the control was killed (QP11).
- **QH28. Stated limits of the proof.** Another process reading the file during the invocation looks like the subject's read. A read that returns no bytes at the end of the file moves its access time and counts as a read.

## 8. Stated limits

The confinement does not prevent the following. Each was found by the spike and is stated so that a green is read with it:

  - A refused operation that macOS cannot refuse with a kill, `exception-entitlement` and `process-exec-update-label`, is refused silently. A computation that executes a setuid or setgid program fails with EPERM and no kill.
  - A child that leaves a computation's group through `posix_spawn` outlives the group's end, still confined (QH19).
  - Global file metadata stays visible (QH15).
  - Operation classes that every process needs at startup stay open implicitly: dynamic code generation, file existence tests, `sandbox-check`, Mach, MIG and Unix system calls, `fcntl`, and MAC system calls. Their scope is unmeasured.
  - The protocols of the allowed services are not examined. `com.apple.logd` lets a program write to the unified log outside its home, and on a Mac bound to a directory server, user and group lookups can become network queries.
  - Notification state is shared with other processes.
  - `/bin/sh` creates the temporary files of here-documents and here-strings in the system temporary directories, whatever `TMPDIR` says; a computation that uses them is killed.
  - A shared directory's write-access probe cannot be permitted without also permitting its removal, so the probe is refused.
  - A copy of the subject's binary at a path the engine does not know escapes a computation's refusal (QH17).
  - The system reports no exit detail for a code-signing kill (QH21).
  - A program that resolves a service or protocol name through the C library, as a binary built with cgo may, is killed by an operation the spike did not identify; Go's own resolver works.
  - The private copy was tested on unsigned and ad-hoc signed binaries only.
  - The refusals the profile names are the operations this macOS version knows; a later version may add operations the profile does not refuse (QH2). A change to the profile is a new specification version, and macOS 27.0.1 stays supported only with the profile it passed.
  - These refusals are in the profile but no harmless attempt could witness them, so only the profile's text shows them: `system-memorystatus-control`, `process-info-sandbox-container`, `mach-task-special-port*`, `coalition-info`, `iokit-open-service`, `nvram*`, `fs-quota*`, `fs-snapshot-mount`, `file-graft`, `file-ungraft`, `system-privilege`, `mach-bootstrap`, `mach-cross-domain-lookup`, `isp-command-send`, `asr-parser-enter`, `darwin-notification-post` and `system-necp-client-action`.
  - `system-info` is granted in full (QH15).
  - The filter that keeps a memory or code-signing kill from counting as the confinement's has no witness: no test made the system report either exit detail.
