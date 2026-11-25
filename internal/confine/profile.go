// Package confine builds the sealed confinement profile and implements the trusted launcher.
package confine

const profileTemplate = `(version 1)
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
`

const computationProcess = `(allow process-fork)
(allow signal (target same-sandbox))
(deny syscall-unix (syscall-number SYS_setsid SYS_setpgid) (with send-signal SIGKILL))
`

const subjectFiles = `(allow file-read* (subpath (param "HOME")))
(allow file-write* (subpath (param "HOME")))
(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))
(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))
(deny file-write-unlink file-write-create (literal (param "PLANTED_DIR_n")) (with send-signal SIGKILL))
`

const readControl = `(deny file-read-data (literal (param "PLANTED")) (with send-signal SIGKILL))
`

const computationFiles = `(allow sysctl-read (sysctl-name "kern.ngroups"))
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
`
