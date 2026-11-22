#include <netdb.h>
#include <dirent.h>
#include <sys/file.h>
#include <sys/mount.h>
#include <sys/ioctl.h>
#include <sys/resource.h>

static int witness(int argc, char **argv) {
    if (argc < 2) return -1;
    const char *mode = argv[1];
    if (!strcmp(mode, "resolver")) {
        struct addrinfo *answer = NULL;
        int rc = getaddrinfo("decoy.invalid", NULL, NULL, &answer);
        if (answer) freeaddrinfo(answer);
        if (!rc) return 45;
        puts("lookup failed"); return 0;
    }
    if (!strcmp(mode, "preboot")) {
        DIR *dir = opendir("/System/Volumes/Preboot");
        if (dir) { while (readdir(dir)) {} closedir(dir); puts("performed"); }
        puts("fallback"); return 0;
    }
    if (!strcmp(mode, "system-child")) {
        if (argc < 4) return 64;
        pid_t child = fork();
        if (child < 0) return 46;
        if (!child) {
            alarm(2);
            execv(argv[2], &argv[2]);
            puts("fallback"); fflush(stdout); _exit(47);
        }
        int status;
        if (waitpid(child, &status, 0) != child) return 48;
        printf("child-status=%d\n", status); return 0;
    }
    if (strncmp(mode, "witness-", 8)) return -1;
    if (argc < 3) return 64;
    int twin = !strcmp(argv[argc-1], "twin");
    int rc = -1, fd = -1;
    const char *op = mode + 8;
    if (!strcmp(op, "xattr")) {
        char data[8];
        if (!twin) rc = (int)getxattr(argv[2], "user.decoy", data, sizeof(data), 0, 0);
    } else if (!strcmp(op, "link") || !strcmp(op, "clone")) {
        if (argc < 4) return 64;
        if (!twin) rc = !strcmp(op,"link") ? link(argv[2],argv[3]) : clonefile(argv[2],argv[3],0);
    } else if (!strcmp(op, "lock") || !strcmp(op, "map")) {
        fd = open(argv[2], O_RDONLY);
        if (fd < 0) return 49;
        if (!twin && !strcmp(op,"lock")) rc = flock(fd, LOCK_SH | LOCK_NB);
        if (!twin && !strcmp(op,"map")) {
            void *p = mmap(NULL, 5, PROT_READ | PROT_EXEC, MAP_PRIVATE, fd, 0);
            if (p != MAP_FAILED) { rc = 0; munmap(p,5); }
        }
    } else if (!strcmp(op, "fs")) {
        struct statfs info;
        if (!twin) rc = statfs(argv[2], &info);
    } else if (!strcmp(op, "policy")) {
        if (!twin) rc = getiopolicy_np(IOPOL_TYPE_DISK, IOPOL_SCOPE_PROCESS);
    } else if (!strcmp(op,"socket-get") || !strcmp(op,"socket-set") || !strcmp(op,"socket-ioctl")) {
        fd = socket(AF_INET, SOCK_STREAM, 0);
        if (fd < 0) return 50;
        int value = 1; socklen_t size = sizeof(value);
        if (!twin) {
            if (!strcmp(op,"socket-get")) rc = getsockopt(fd,SOL_SOCKET,SO_TYPE,&value,&size);
            if (!strcmp(op,"socket-set")) rc = setsockopt(fd,SOL_SOCKET,SO_KEEPALIVE,&value,size);
            if (!strcmp(op,"socket-ioctl")) rc = ioctl(fd,SIOCATMARK,&value);
        }
    } else return -1;
    if (fd >= 0) close(fd);
    if (rc >= 0) puts("performed");
    puts("fallback"); return 0;
}
