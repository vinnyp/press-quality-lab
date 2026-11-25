# Hostile home: what a confined subject can leave, and how to remove it

**Unit:** U43 of `docs/plans/2026-11-24-2115-feat-grade-command-plan.md`. The spike scopes the darwin half of U40 and KTD20. It changes neither the sealed subject profile nor any engine code.

**Result in one line:** under the unchanged subject profile, a subject can leave entries that no descriptor on the entry itself can undo: a deny ACL, or mode 0000 combined with `uchg` and a deny ACL. Every tested case can be undone without following a link, by calls relative to the **parent** directory's descriptor:

1. `setattrlistat` with `FSOPT_NOFOLLOW` to clear the flags, then the ACL;
2. `fchmodat`;
3. `unlinkat`.

That holds even for an ACL entry that also denies `writesecurity`, `writeattr` and `chown` (see the supplementary check). So KTD20's walk works only if it uses parent-relative calls, not descriptors on the hostile entries.

## Run identity

| Field | Value |
|---|---|
| Spike code (stays on the branch) | commit bf34a7d87d78bc38fd1e4028dbdb555918c5ec40, package `internal/spike/hostilehome` |
| Engine under test | `main` at 203a4e2 (#19): the real `pql` launcher, `confine.NewCopies`, `Copies.Start`, QH11's subject variant unchanged |
| System | macOS 27.2 (26B123), Apple silicon, APFS, an ordinary user, outside any tool sandbox |
| Go | go1.27.4 darwin/arm64 |
| Command | `CGO_ENABLED=0 go test -tags macos_evidence -count=1 -v -timeout 10m ./internal/spike/hostilehome` |
| Result | `TestHostileHomeConfined`, `TestHostileHomeUnconfined` and `TestHostileHomeSocketOptions` pass. They assert that every probe line was produced; the findings are the lines themselves. |
| Log SHA-256 | dd92d10f75c30ea04b7d34141f63732731b85d2acc5f36c072293c57a068985c. The log is kept locally and contains no home paths. |

An earlier run of the first probe alone (descriptor strategy only) left eight entries the test could not remove: the four ACL and combined items, confined and unconfined. The coordinator removed them by path as the owner: `chmod -N` removed the ACL, `chflags nouchg` the flag, then `rm -rf`. That observation led to the second probe. Its log is not part of this record.

## Method

- Each hostile item runs in its own fresh home, as its own confined invocation, so a kill cannot hide a later item. The same synthetic program also runs unconfined as a positive control.
- The hostile program makes raw `golang.org/x/sys/unix` and `syscall` calls and prints each call's exact result.
- After the program has exited, two cleanup strategies run, each on its own independent fixture:
  - **Descriptor:** open the entry with `O_NOFOLLOW|O_NONBLOCK` (then `O_EVTONLY` if that fails), then `Fchflags`, ACL removal by `fsetattrlist`, `Fchmod`, `Root.Chmod` and `Root.RemoveAll`.
  - **Parent:** open only the parent with `O_DIRECTORY|O_NOFOLLOW`, then on the entry by name: `setattrlistat(..., ATTR_CMN_FLAGS = 0, FSOPT_NOFOLLOW)`, `setattrlistat(..., ATTR_CMN_EXTENDED_SECURITY = KAUTH_FILESEC_NOACL, FSOPT_NOFOLLOW)`, `fchmodat(..., 0700, AT_SYMLINK_NOFOLLOW)`, `fstatat` with no-follow, and recursion through no-follow directory descriptors, then `unlinkat`.
- **Link safety:** the hostile program replaces an entry with a symlink to `../outside`, a file the test made beside the home with a deny ACL and `uchg`. The test compares that file's mode, flags and ACL bytes before and after cleanup.
- FIFOs and sockets are never opened for data.
- `golang.org/x/sys/unix` (the pinned version) wraps `Setattrlist`, `Fchflags`, `Fchmod`, `Fchmodat`, `Fstatat`, `Unlinkat` and the descriptor xattr calls. It has no wrapper for `setattrlistat`, `fgetattrlist`, `fsetattrlist` or the `acl_*` calls, so the spike used raw syscalls for those.

## What a confined subject can leave

| Item | Under the subject profile | Unconfined control |
|---|---|---|
| `uchg` on a file and a directory | succeeds | succeeds |
| `uappnd` on a file and a directory | succeeds | succeeds |
| Deny ACL (`everyone deny read,delete`) on a file and a directory | succeeds. The spike's entry deliberately did not deny `writesecurity`; the supplementary check covers that case | succeeds |
| Extended attribute | succeeds | succeeds. On this system every file created, confined or not, also carries `com.apple.provenance` |
| `chmod 0000` on a file and a directory | succeeds | succeeds |
| Mode 0000 + `uchg` + deny ACL on a file and a directory | all three calls succeed | succeeds |
| FIFO (`mkfifo`) | succeeds | succeeds |
| Unix socket (`socket` then `bind`) | `socket` succeeds. No `bind` result line is printed, and the program ends by signal 9 after the marker, so the kill came at or right after `bind` (QO3). No socket file is left | `bind` succeeds |
| Symlink pointing out of the home | succeeds. The sealed profile kills `file-link` (hard links); this spike tested only symlinks | succeeds |

## How each is removed

| Item | Descriptor strategy | Parent strategy |
|---|---|---|
| `uchg`, `uappnd` | removed (`openat`, `Fchflags`, then removal) | removed |
| Extended attribute | removed (the descriptor strategy removed both attributes, including `com.apple.provenance`, then the entry) | removed |
| Mode 0000 | removed (no descriptor; `Root.Chmod` then removal) | removed |
| Deny ACL, file and directory | **left behind**: no descriptor (`EACCES` for both open modes); `Root.Chmod` succeeds but removal fails `EACCES` | removed |
| 0000 + `uchg` + deny ACL, file and directory | **left behind**: no descriptor; `Root.Chmod` fails `EPERM` (the flag); removal fails | removed (flags cleared first, then the ACL, then mode, then `unlinkat`) |
| FIFO | removed without opening it | removed |
| Symlink out of the home (a leaf link to a file) | the link is removed | the link is removed; the outside file's mode, `uchg` flag and ACL bytes are unchanged (`unchanged=true` in all four checks) |

Every test tree in the second run was fully removed by the parent strategy at teardown; the teardown asserts it.

The evidence tests assert four things:
- that each probe line was produced;
- `Confinement:true` on every confined run;
- the outside file unchanged;
- the teardown.

They do not assert each step's result, so the tables above come from reading the log lines. The log's SHA-256 is given above.

## Supplementary check: an entry that also denies `writesecurity`

The spike's entry denied only read and delete. A subject could instead deny everything, including `writesecurity`, `writeattr` and `chown`. The coordinator checked that case unconfined, as the file's owner, on the same system:
- a file with `everyone deny read,write,append,delete,writeattr,writeextattr,writesecurity,chown`;
- a directory with the matching directory rights denied, plus `uchg`.

The results:
- `chmod -N` removed the file's ACL; the owner keeps the right to change it.
- On the directory, `chmod -N` failed `EPERM` while `uchg` was set. After `chflags nouchg` (which the deny entry did not block), the ACL removal and the deletion succeeded.

So the owner can always clear the flag, then the ACL, then delete. The order is required: an ACL change is refused while `uchg` is set. A subject setting this broader entry under confinement was not itself tested; the confined program set the narrower entry with the same call (`setattrlist`), which the profile allowed.

## Go socket options

These were read back after `net.Listen` and `net.Dial` on IPv4 loopback, unconfined. Readback alone cannot tell an explicit Go `setsockopt` from a kernel default.

| Option | Listening | Dialing |
|---|---|---|
| `SO_REUSEADDR` | 4 (set) | 0 |
| `SO_REUSEPORT` | 0 | 0 |
| `SO_KEEPALIVE` | 0 | 8 (set) |
| `SO_NOSIGPIPE` | 0 | 0 |
| `SO_BROADCAST` | 0 | 0 |
| `SO_LINGER` | off | off |
| `TCP_NODELAY` | 0 | 4 (set) |
| `TCP_KEEPALIVE` | 7200 | 15 |
| `TCP_KEEPINTVL` | 75 | 15 |
| `TCP_KEEPCNT` | 8 | 9 |

A Go listening socket shows `SO_REUSEADDR` on, and a dialing socket shows keep-alive and `TCP_NODELAY` on; whether Go or the kernel set each is not attributed here (see Go's `net/sockopt_*.go`). Under the subject profile the network calls are killed anyway (QH15), so these values matter only for how QO3 reads such a kill.

## Consequences for U40 and KTD20

1. **KTD20's cleanup must be parent-relative.** "Clear user flags with `Fchflags` on a descriptor opened `O_NOFOLLOW`" cannot work for an entry with a deny ACL, because no descriptor can be opened on it. The walk should open each directory with `O_DIRECTORY|O_NOFOLLOW` and act on its entries by name: `setattrlistat` with `FSOPT_NOFOLLOW` to clear the flags, then the ACL, then `fchmodat` with `AT_SYMLINK_NOFOLLOW`, then `unlinkat`. The order matters: an ACL change and a mode change are refused (`EPERM`) while `uchg` is set, so clear the flags first.
2. **`internal/throwaway` needs one raw syscall.** `golang.org/x/sys` v0.38.0 defines `SYS_SETATTRLISTAT` but has no `setattrlistat` wrapper, so the package will carry `syscall.Syscall6(SYS_SETATTRLISTAT, ...)` with a documented lint exception, plus an evidence test. Adding a dependency is not needed.
3. **The no-follow calls do not leak through a planted leaf symlink to a file.** The outside file was untouched. A symlink to a directory, or one swapped in for an intermediate directory, was not tested; U40's tests should add both. The spike does not test a subject racing the walk; U40 still owns the time-of-check race once the subject has exited (the group is ended first, QH23).
4. **The system adds `com.apple.provenance` to new files, confined or not.** It does not block removal, though removing it by descriptor fails `EPERM` while `uchg` is set.
5. **No sealed rule needs to change** and no owner decision is needed. The profile stays as sealed; the engine's cleanup carries the burden.
