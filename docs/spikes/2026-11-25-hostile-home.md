# Hostile home: cleanup feasibility

Draft U43 record. Results await an owner-controlled macOS run. This experiment
scopes the Darwin half of U40 and KTD20; it does not implement cleanup or change
the sealed subject profile.

## Run identity

| Field | Value |
|---|---|
| Spike branch commit (code remains on the branch) | TBD |
| Exact commit tested | TBD |
| macOS version and build | TBD |
| Go version and architecture | TBD |
| Home volume and filesystem | TBD |
| Ordinary-user run, outside tool sandbox | TBD |
| Log location and SHA-256 | TBD |

Run from the repository root, outside any sandbox:

```sh
CGO_ENABLED=0 go test -tags macos_evidence -count=1 -v -timeout 10m ./internal/spike/hostilehome
```

## Method and line format

The synthetic executable is built in a temporary directory. The confined test
uses the real `cmd/pql` launcher, `confine.NewCopies` and `Copies.Start` with the
unchanged QH11 subject variant. Every item gets a separate invocation and home,
so a kill does not suppress later items. The positive control runs the same
executable unconfined. Neither program invokes external utilities. The evidence
tests check that probe lines exist, not whether a mutation or cleanup succeeded.
A missing launcher marker is an infrastructure failure, not a refusal result.

`attempt ITEM STEP=success` or `errno=N(TEXT)` gives the exact syscall result.
Creation and close calls also appear. A kill before a call returns has no errno;
record the run end and last completed step, and leave the next result unknown.
A line from creation alone does not establish that a later mutation ran.

`cleanup ITEM` records `Root.Lstat` mode and flags, descriptor open results,
`Fstat` mode and flags, ACL presence/count (or unknown), the raw extended-security
attribute bytes, and xattr names. It then records flag clearing, descriptor ACL
removal, mode restoration, close, a `Root.Chmod` fallback, the directory child's
`Root.Lstat` and `Root.RemoveAll`, final `Root.RemoveAll`, and final absence.
The first open uses `Openat` with `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`; on failure,
the probe also measures `O_EVTONLY|O_NOFOLLOW|O_NONBLOCK`. This distinguishes a
failed read-open from inability to obtain any useful descriptor. FIFOs and
sockets are never opened for data. The directories contain one synthetic child.
The process has ended before cleanup; this bounded, fixed-child experiment is
not the racing-tree safety implementation required of U40.

The deny ACE targets everyone and denies delete and read-data/list-directory. The
combined items apply the ACE, mode 0000, then `uchg`, closing creation descriptors
first. Record each step separately: a refused earlier call means the resulting
entry is not the full combination. The descriptor strategy does not clear flags by path.

The pinned `golang.org/x/sys/unix` provides `Setattrlist`, `Openat`, `Fstat`,
`Fchflags`, `Fchmod`, `Flistxattr` and `Setxattr` on Darwin. It has no `acl_*`,
`Fgetattrlist` or `Fsetattrlist` wrappers. The experiment encodes Darwin's
`attrreference_t` and `kauth_filesec` (SDK `sys/attr.h` and `sys/kauth.h`), sets
`ATTR_CMN_EXTENDED_SECURITY` with `Setattrlist`, and tests descriptor removal
using `syscall.Syscall6` with `SYS_FSETATTRLIST` and `KAUTH_FILESEC_NOACL`.
Descriptor inspection uses `SYS_FGETATTRLIST`. Success of these raw calls is
TBD; API constants alone do not establish feasibility. `Fchmod` is measured
separately and is not presumed to remove an ACL.

## Second probe method (results TBD)

Each item runs twice in separate fresh homes, under subtests named
`ITEM/descriptor` and `ITEM/parent`. Thus the descriptor strategy cannot remove
or soften the parent strategy's fixture. Both confined runs and unconfined
controls keep every syscall result; a refusal is evidence, not a pass.

The parent strategy opens only the parent with `O_DIRECTORY|O_NOFOLLOW`.
`cleanup-parent ITEM setattrlistat-clear-flags=RESULT` clears `ATTR_CMN_FLAGS`
with a zero buffer, then `setattrlistat-clear-acl=RESULT` clears
`ATTR_CMN_EXTENDED_SECURITY` with `KAUTH_FILESEC_NOACL`. Both calls use
`FSOPT_NOFOLLOW`. Flags go first so an immutable entry need not accept an ACL
change before its flag is cleared. `fchmodat-nofollow=RESULT` measures
`Fchmodat(..., 0700, AT_SYMLINK_NOFOLLOW)` support.
`fstatat-nofollow=RESULT` inspects the entry without following it.
`parent-walk` lines report directory open, enumeration and close; only a
no-follow directory descriptor is used for recursion. The final
`cleanup-parent ITEM unlinkat=RESULT gone=BOOL final-fstatat=RESULT` reports
removal (with `AT_REMOVEDIR` for directories) and independent absence checking.
`parent-open` and `parent-close` report the parent descriptor's lifetime.
No FIFO or socket is opened for data by this strategy.

The pinned `x/sys/unix` has no `Setattrlistat` wrapper; the spike uses
`syscall.Syscall6` with `SYS_SETATTRLISTAT`. It does wrap `Fchmodat`, `Fstatat`
and `Unlinkat`. Local SA1019 suppressions name the spike at the raw syscall
and the two deprecated descriptor syscall constants. These are experimental
calls whose runtime results remain TBD.

The `symlink` item creates and removes an ordinary entry, then attempts to
replace it with `../outside`. The test creates that outside file beside the
home, sets its deny ACL and `uchg` unconfined, and retains a descriptor from
before hardening. `link-control` reports whether replacement produced a link;
the unconfined control requires one. `outside before` and `outside after`
print mode, flags and extended-security bytes through the retained descriptor;
`unchanged=true` requires exact equality. A confined kill before replacement
is recorded, not treated as link-safety evidence.

Every test-created temporary tree registers a `t.Cleanup` that uses the parent
strategy recursively, including the outside control, then asserts the tree is
gone. This runs even after a failed probe. It is fixture teardown, not evidence
that an earlier cleanup attempt succeeded. The fixed trees are finite and the
subject has exited before either strategy runs; race safety remains U40 work.

| Second probe observation | Result |
|---|---|
| Parent-relative flags, ACL, mode and removal per item | TBD |
| Outside ACL, flags and mode unchanged | TBD |
| Both strategies compared on independent fixtures | TBD |
| Temporary trees fully removed | TBD |

## User immutable flags (`uchg`)

| Item | Confined syscall result | Unconfined control | Descriptor open | Clear flags | ACL removal | Mode restore | Finally gone |
|---|---|---|---|---|---|---|---|
| File | TBD | TBD | TBD | TBD | TBD | TBD | TBD |
| Directory | TBD | TBD | TBD | TBD | TBD | TBD | TBD |

## User append flags (`uappnd`)

| Item | Confined syscall result | Unconfined control | Descriptor open | Clear flags | Finally gone |
|---|---|---|---|---|---|
| File | TBD | TBD | TBD | TBD | TBD |
| Directory | TBD | TBD | TBD | TBD | TBD |

## Access-control deny entries

| Item | Confined syscall result | Unconfined control | ACL observed | Descriptor removal | Fchmod result | Finally gone |
|---|---|---|---|---|---|---|
| File | TBD | TBD | TBD | TBD | TBD | TBD |
| Directory | TBD | TBD | TBD | TBD | TBD | TBD |

## Extended attributes

| Item | Confined syscall result | Unconfined control | Observed names | Finally gone |
|---|---|---|---|---|
| File | TBD | TBD | TBD | TBD |
| Directory | TBD | TBD | TBD | TBD |

## Mode 0000

| Item | Confined syscall result | Unconfined control | Read-open | Event-only open | Fchmod | Root.Chmod | Finally gone |
|---|---|---|---|---|---|---|---|
| File | TBD | TBD | TBD | TBD | TBD | TBD | TBD |
| Directory | TBD | TBD | TBD | TBD | TBD | TBD | TBD |

## FIFO

| Confined mkfifo | Unconfined control | Lstat mode | Removal without data open | Finally gone |
|---|---|---|---|---|
| TBD | TBD | TBD | TBD | TBD |

## Unix socket file

| Confined socket/bind | Unconfined control | Lstat mode | Removal without data open | Finally gone |
|---|---|---|---|---|
| TBD | TBD | TBD | TBD | TBD |

## Combined mode 0000, `uchg`, and deny entry

| Item | Confined results of all three calls | Unconfined control | Read-open | Event-only open | Clear flags | Remove ACL | Restore mode | Finally gone |
|---|---|---|---|---|---|---|---|---|
| File | TBD | TBD | TBD | TBD | TBD | TBD | TBD | TBD |
| Directory | TBD | TBD | TBD | TBD | TBD | TBD | TBD | TBD |

Question: can the subject produce the full combination, and can an ordinary
engine user obtain a descriptor capable of clearing it? If both opens fail,
record the `Root.Chmod` result and remaining entry; do not infer cleanup works.

## Go socket options

The unconfined probe listens and dials TCP over IPv4 loopback only.
`socket ROLE OPTION value=VALUE result=RESULT` reports effective values after
`net.Listen` or `net.DialTimeout`. Readback alone cannot distinguish a kernel
default from an explicit Go setsockopt call. For attribution, inspect the tested
Go version's `net/sockopt_bsd.go`, `net/sockopt_posix.go`, `net/tcpsockopt_posix.go`
and `net/tcpsockopt_darwin.go`. IPv6, UDP, multicast, accepted sockets, and
caller-selected options are outside this probe's scope.

| Option | Listening value/result | Dialing value/result | Explicit Go setter or kernel default |
|---|---|---|---|
| SO_REUSEADDR | TBD | TBD | TBD |
| SO_REUSEPORT | TBD | TBD | TBD |
| SO_KEEPALIVE | TBD | TBD | TBD |
| SO_NOSIGPIPE | TBD | TBD | TBD |
| SO_BROADCAST | TBD | TBD | TBD |
| SO_LINGER | TBD | TBD | TBD |
| TCP_NODELAY | TBD | TBD | TBD |
| TCP_KEEPALIVE | TBD | TBD | TBD |
| TCP_KEEPINTVL | TBD | TBD | TBD |
| TCP_KEEPCNT | TBD | TBD | TBD |

## Consequences for U40

TBD from the measured confined results and their positive controls. Name each
reachable hazard, the descriptor call that removed it, and each remaining
failure requiring an owner decision before cleanup is designed. Do not widen
the subject profile to make this experiment pass.
