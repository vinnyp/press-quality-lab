#include <stdio.h>
#include <unistd.h>
#include <spawn.h>

extern char **environ;

/* The marker precedes subject startup and is not emitted by the subject. */
int main(int argc, char **argv) {
    if (argc < 2) return 64;
    posix_spawnattr_t attributes;
    posix_spawn_file_actions_t actions;
    if (posix_spawnattr_init(&attributes) || posix_spawn_file_actions_init(&actions)) return 70;
    if (posix_spawnattr_setflags(&attributes, POSIX_SPAWN_SETEXEC | POSIX_SPAWN_CLOEXEC_DEFAULT)) return 70;
    for (int fd = 0; fd < 3; fd++) {
        if (posix_spawn_file_actions_addinherit_np(&actions, fd)) return 70;
    }
    if (write(2, "confined\n", 9) != 9) return 70;
    pid_t pid;
    if (posix_spawn(&pid, argv[1], &actions, &attributes, argv + 1, environ)) {
        if (write(2, "launch-failed\n", 14) != 14) return 70;
    }
    return 71;
}
