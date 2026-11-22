#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include <sys/wait.h>
#include <fcntl.h>
#include <errno.h>
#include <spawn.h>
#include <sys/stat.h>

extern char **environ;
#include "files.h"
#include "operations.h"
#include "witnesses.h"

/* All process targets are children created here, never caller-supplied pids. */
int main(int argc, char **argv) {
    alarm(4);
    int answer = witness(argc, argv);
    if (answer != -1) return answer;
    answer = file_operation(argc, argv);
    if (answer != -1) return answer;
    answer = operation(argc, argv);
    if (answer != -1) return answer;
    if (argc < 2 || argc > 3) return 64;
    int twin = argc == 3 && !strcmp(argv[2], "twin");
    if (argc == 3 && !twin) return 64;
    if (!strcmp(argv[1], "environment")) {
        for (char **entry = environ; *entry; entry++) puts(*entry);
        return 0;
    }
    if (!strcmp(argv[1], "terminal") || !strcmp(argv[1], "terminal-open")) {
        if (getsid(0) != getpid() || getpgrp() != getpid()) return 42;
        if (!strcmp(argv[1], "terminal-open")) {
            int fd = open("/dev/tty", O_RDWR | O_NOCTTY);
            if (fd >= 0) { close(fd); return 43; }
            if (errno != ENXIO) return 44;
        }
        puts("terminal absent"); return 0;
    }
    if (!strcmp(argv[1], "descriptors")) {
        struct stat input, output, diagnostic, null_device;
        if (fstat(0, &input) || fstat(1, &output) ||
            fstat(2, &diagnostic) || stat("/dev/null", &null_device)) return 7;
        if (!S_ISCHR(input.st_mode) || input.st_rdev != null_device.st_rdev ||
            !S_ISFIFO(output.st_mode) || !S_ISFIFO(diagnostic.st_mode)) return 8;
        for (int fd = 3; fd < 1024; fd++) {
            errno = 0;
            if (fcntl(fd, F_GETFD) != -1 || errno != EBADF) return 9;
        }
        puts("descriptors clean");
        return 0;
    }
    if (!strcmp(argv[1], "clean")) {
        puts("clean");
        return 0;
    }
    int escape = !strcmp(argv[1], "session") || !strcmp(argv[1], "group");
    if (!escape && strcmp(argv[1], "child")) return 64;
    pid_t child = !escape && twin ? -2 : fork();
    if (child == -2) {
        puts("fallback");
        return 0;
    }
    if (child < 0) {
        puts("fallback");
        return 0;
    }
    if (!child) {
        alarm(2);
        int rc = 0;
        pid_t original_group = getpgrp();
        if (!twin && !strcmp(argv[1], "session")) rc = setsid();
        if (!twin && !strcmp(argv[1], "group")) rc = setpgid(0, 0);
        if (rc < 0) {
            puts("fallback");
            fflush(stdout);
            _exit(0);
        }
        if (escape && (getpgrp() == original_group) != twin) _exit(6);
        puts("performed");
        fflush(stdout);
        _exit(0);
    }
    int status = 0;
    if (waitpid(child, &status, 0) != child) return 4;
    if (WIFSIGNALED(status) && WTERMSIG(status) == SIGKILL) { printf("child-status=%d\n", status); return 0; }
    if (!WIFEXITED(status) || WEXITSTATUS(status)) return 5;
    puts("fallback");
    return 0;
}
