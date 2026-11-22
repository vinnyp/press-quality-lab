#include <sys/mman.h>
#include <sys/xattr.h>
#include <copyfile.h>
#include <sys/clonefile.h>

extern int sandbox_check(pid_t, const char *, int, ...);
extern const int SANDBOX_CHECK_NO_REPORT;

static int file_operation(int argc, char **argv) {
    if (argc < 2) return -1;
    const char *mode = argv[1];
    if (!strcmp(mode, "inventory") || !strcmp(mode,"catalog")) {
        for (int i = 2; i < argc; i++) {
            /* Only query ourselves; a denial may signal the queried process. */
            int rc = sandbox_check(getpid(), argv[i], SANDBOX_CHECK_NO_REPORT);
            if (rc < 0) { if (!strcmp(mode,"catalog")) continue; return 32; }
            printf("%s=%d\n", argv[i], rc == 0);
        }
        return 0;
    }
    if (!strcmp(mode,"file-directory")) {
        const char *home=getenv("HOME");if(!home)return 64;
        int fd=open(home,O_RDONLY|O_DIRECTORY);if(fd<0)return 39;
        char path[4096];if(fcntl(fd,F_GETPATH,path)<0){close(fd);return 41;}
        struct stat st;int rc=fstat(fd,&st);if(close(fd)||rc||!S_ISDIR(st.st_mode))return 40;
        puts("directory ok");return 0;
    }
    if (!strcmp(mode,"file-home")) {
        const char *home=getenv("HOME");char path[4096];
        if (!home || snprintf(path,sizeof(path),"%s/state",home)>=(int)sizeof(path)) return 64;
        int fd=open(path,O_WRONLY|O_CREAT|O_TRUNC,0600);if(fd<0)return 33;
        int ok=write(fd,"decoy",5)==5;int rc=close(fd);
        if(!ok||rc)return 34;
        puts("home written");return 0;
    }
    if (strncmp(mode, "file-", 5)) return -1;
    if (argc < 5) return 64;
    int twin = !strcmp(argv[argc-1], "twin");
    const char *path = twin ? argv[3] : argv[2];
    const char *destination = argv[4];
    int rc = -1, fd = -1;
    if (!strncmp(mode,"file-secret-",12)) {
        if(argc!=6 || strlen(argv[5])!=32)return 64;
        if(!strcmp(mode,"file-secret-stat")){struct stat st;rc=stat(path,&st);if(!rc&&st.st_size!=32)return 38;}
        else {
            fd=open(path,O_RDONLY);
            if(fd>=0 && !strcmp(mode,"file-secret-read")){char data[32];rc=read(fd,data,32)==32&&!memcmp(data,argv[5],32)?0:-1;}
            else if(fd>=0 && !strcmp(mode,"file-secret-map")){void *data=mmap(NULL,32,PROT_READ,MAP_PRIVATE,fd,0);if(data!=MAP_FAILED){rc=memcmp(data,argv[5],32);munmap(data,32);}}
        }
    }
    else if (!strcmp(mode,"file-open")) { char data;fd=open(path,O_RDONLY);if(fd>=0)rc=read(fd,&data,1)==1?0:-1; }
    else if (!strcmp(mode,"file-read") || !strcmp(mode,"file-ignore-home")) {
        char data[5]; fd = open(path,O_RDONLY);
        if (fd >= 0) rc = read(fd,data,sizeof(data)) == 5 && !memcmp(data,"decoy",5) ? 0 : -1;
    } else if (!strcmp(mode,"file-map")) {
        fd = open(path,O_RDONLY);
        if (fd >= 0) {
            void *data = mmap(NULL,5,PROT_READ,MAP_PRIVATE,fd,0);
            if (data != MAP_FAILED) { rc = memcmp(data,"decoy",5); munmap(data,5); }
        }
    } else if (!strcmp(mode,"file-stat")) {
        struct stat st; rc = stat(path,&st); if (!rc && st.st_size != 5) return 31;
    } else if (!strcmp(mode,"file-write")) {
        fd = open(path,O_WRONLY); if (fd >= 0) rc = write(fd,"other",5) == 5 ? 0 : -1;
    } else if (!strcmp(mode,"file-truncate")) rc = truncate(path,0);
    else if (!strcmp(mode,"file-create")) { fd = open(path,O_WRONLY|O_CREAT|O_EXCL,0600); rc = fd < 0 ? -1 : 0; }
    else if (!strcmp(mode,"file-unlink")) rc = unlink(path);
    else if (!strcmp(mode,"file-rename")) rc = rename(path,destination);
    else if (!strcmp(mode,"file-chmod")) rc = chmod(path,0400);
    else if (!strcmp(mode,"file-xattr")) rc = setxattr(path,"user.decoy","x",1,0,0);
    else if (!strcmp(mode,"file-link") || !strcmp(mode,"file-clone")) {
        struct stat st;if(stat(path,&st))return 37;
        /* There is no permitted hard-link or clone target. The twin omits
           only that call, after the same source metadata lookup. */
        if (!twin) rc=!strcmp(mode,"file-link")?link(path,destination):clonefile(path,destination,0);
    }

    else if (!strcmp(mode,"file-symlink")) {
        rc = symlink(path,destination);
        if (!rc) { char data[5]; fd = open(destination,O_RDONLY); rc = fd >= 0 && read(fd,data,5)==5 ? 0 : -1; }
    }
    else return 64;
    if (fd >= 0) close(fd);
    if (!rc) puts("performed");
    puts("fallback");
    return 0;
}
