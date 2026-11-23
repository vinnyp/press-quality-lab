#include <dirent.h>

/* The caller supplies only a temporary directory. The twin lists the home
   through the same calls; the violator ignores failure and reaches fallback. */
static int directory_operation(int argc, char **argv) {
    if (argc < 2) return -1;
    if(!strcmp(argv[1],"directory-writable")) {
        if(argc<3)return 64;
        operation_ready();
        if(access(argv[2],W_OK))return 74;
        puts("writable");return 0;
    }
    if(!strcmp(argv[1],"directory-remove")) {
        if(argc<3)return 64;
        operation_ready();
        if(!rmdir(argv[2]))puts("performed");
        puts("fallback");return 0;
    }
    if (!strncmp(argv[1],"root-",5)) {
        if(argc<4)return 64;
        int twin=!strcmp(argv[argc-1],"twin");
        const char *path=twin?argv[3]:getenv("HOME");
        operation_ready();
        int rc;
        if(!strcmp(argv[1],"root-replace")) { rc=rmdir(path);if(!rc)rc=symlink(argv[2],path); }
        else if(!strcmp(argv[1],"root-rename"))rc=rename(path,argv[2]);
        else return 64;
        if(!rc)puts("performed");puts("fallback");return 0;
    }
    if (!strncmp(argv[1], "image-", 6)) {
        char path[4096];
        if (strlen(argv[0]) >= sizeof(path)) return 64;
        strcpy(path, argv[0]);
        char *end = strrchr(path, '/'); if (!end) return 64; *end = 0;
        int twin = !strcmp(argv[argc-1], "twin");
        if (!strcmp(argv[1], "image-parent") && !twin) { end = strrchr(path,'/'); if (!end) return 64; *end = 0; }
        operation_ready();
        if (!strcmp(argv[1],"image-chmod") || !strcmp(argv[1],"image-unlink")) {
            char target[4096];const char *base=twin?getenv("HOME"):path;
            if(snprintf(target,sizeof(target),"%s/%s",base,twin?"state":"image")>=(int)sizeof(target))return 64;
            int rc=!strcmp(argv[1],"image-chmod")?chmod(target,0700):unlink(target);
            if(!rc)puts("performed");
        } else if (!strcmp(argv[1], "image-write")) {
            const char *base = twin ? getenv("HOME") : path;
            char dest[4096]; if (snprintf(dest,sizeof(dest),"%s/new",base)>=(int)sizeof(dest)) return 64;
            int fd = open(dest,O_WRONLY|O_CREAT,0600);
            if (fd>=0) { close(fd); unlink(dest); puts("performed"); }
        } else {
            DIR *dir=opendir(path);
            if(dir) { int count=0; struct dirent *entry;
                while((entry=readdir(dir))) if(strcmp(entry->d_name,".")&&strcmp(entry->d_name,".."))count++;
                if(closedir(dir))return 72;
                if(!strcmp(argv[1],"image-list") && count!=1)return 73;
                puts("performed");
            }
        }
        puts("fallback"); return 0;
    }
    if (strcmp(argv[1], "directory-list")) return -1;
    if (argc < 3) return 64;
    int twin = !strcmp(argv[argc-1], "twin");
    operation_ready();
    DIR *dir = opendir(twin ? getenv("HOME") : argv[2]);
    if (dir) {
        struct dirent *entry;
        int found = 0;
        while ((entry = readdir(dir))) {
            if (strcmp(entry->d_name, ".") && strcmp(entry->d_name, "..")) found++;
        }
        if (closedir(dir)) return 72;
        if (found) puts("performed");
    }
    puts("fallback");
    return 0;
}
