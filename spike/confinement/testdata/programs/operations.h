#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/sysctl.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <mach/mach.h>
#include <servers/bootstrap.h>
#include <pwd.h>
#include <sys/proc.h>
#include <sys/user.h>
#include <libproc.h>
#include <stdint.h>
enum { code_status = 0, code_hash = 5 };
extern int csops(pid_t,unsigned int,void *,size_t);
extern int sandbox_container_path_for_pid(pid_t,char *,size_t);

static int operation(int argc, char **argv) {
    if (argc < 2) return -1;
    const char *mode = argv[1];
    if (!strcmp(mode,"pipeline")) {
        if(argc!=6)return 64;
        execl(argv[2],argv[2],"-c","\"$1\" \"$3\" | \"$2\" -c | { read n; printf '%s\\n' \"$n\"; }","pipeline",argv[3],argv[4],argv[5],NULL);
        return 35;
    }
    if (!strcmp(mode,"system-value")) {
        int status=system("exit 7");
        if(status==-1 || !WIFEXITED(status) || WEXITSTATUS(status)!=7)return 36;
        puts("value=7");return 0;
    }
    if (!strcmp(mode,"execute")) {
        if(argc<4)return 64;
        int twin=!strcmp(argv[argc-1],"twin");
        const char *path=twin?argv[3]:argv[2];
        operation_ready();
        execl(path,path,"clean",NULL);
        puts("fallback");return 0;
    }
    int twin = argc > 2 && !strcmp(argv[argc-1], "twin");

    if (!strcmp(mode,"orphan-session") || !strcmp(mode,"orphan-group")) {
        if(argc!=4)return 64;
        struct sigaction action={0};action.sa_handler=SIG_DFL;
        if(sigemptyset(&action.sa_mask) || sigaction(SIGCHLD,&action,NULL))return 54;
        posix_spawnattr_t attr;
        if(posix_spawnattr_init(&attr))return 55;
        short flags=!strcmp(mode,"orphan-session") ? POSIX_SPAWN_SETSID : POSIX_SPAWN_SETPGROUP;
        if(posix_spawnattr_setflags(&attr,flags))return 56;
        if(flags==POSIX_SPAWN_SETPGROUP && posix_spawnattr_setpgroup(&attr,0))return 57;
        char *args[]={argv[0],"orphan",argv[2],argv[3],NULL};
        pid_t child;
        if(posix_spawn(&child,argv[0],NULL,&attr,args,environ))return 58;
        posix_spawnattr_destroy(&attr);
        printf("%d\n",child);fflush(stdout);
        /* Keep even an exited child unreaped until the observer is bound. */
        char observed[4096];
        if(snprintf(observed,sizeof(observed),"%s/observed",argv[3])>=(int)sizeof(observed))return 64;
        int gate=-1;
        while((gate=open(observed,O_RDONLY))<0)usleep(1000);
        if(close(gate))return 63;
        return 0;
    }
    if (!strcmp(mode,"orphan")) {
        alarm(2);
        if(argc!=4)return 64;
        close(0);close(1);close(2);
        char path[4096];
        if(snprintf(path,sizeof(path),"%s/go",argv[3])>=(int)sizeof(path))return 64;
        int gate=-1;
        while((gate=open(path,O_RDONLY))<0)usleep(1000);
        close(gate);
        if(snprintf(path,sizeof(path),"%s/ready",argv[3])>=(int)sizeof(path))return 64;
        int fd=open(path,O_WRONLY|O_CREAT|O_EXCL,0600);if(fd<0)return 59;
        if(write(fd,"ready",5)!=5 || close(fd))return 60;
        readiness_is_a_file=1;
        char *args[]={argv[0],"tcp",argv[2],NULL};
        int rc=operation(3,args);
        if(snprintf(path,sizeof(path),"%s/fallback",argv[3])>=(int)sizeof(path))return 64;
        fd=open(path,O_WRONLY|O_CREAT|O_EXCL,0600);if(fd<0)return 61;
        if(close(fd))return 62;
        return rc;
    }
    if (!strcmp(mode, "left-child") || !strcmp(mode, "timeout-child")) {
        pid_t child = fork();
        if (child < 0) return 15;
        if (!child) { close(0); close(1); close(2); sleep(2); _exit(0); }
        printf("%d\n", child); fflush(stdout);
        if (!strcmp(mode, "timeout-child")) { alarm(0); sleep(3); }
        return 0;
    }
    if (!strcmp(mode, "spawn-session") || !strcmp(mode, "spawn-group")) {
        if (argc < 3) return 64;
        posix_spawnattr_t attr;
        if (posix_spawnattr_init(&attr)) return 16;
        short flags = !strcmp(mode,"spawn-session") ? POSIX_SPAWN_SETSID : POSIX_SPAWN_SETPGROUP;
        if (posix_spawnattr_setflags(&attr,flags)) return 17;
        if (!strcmp(mode,"spawn-group") && posix_spawnattr_setpgroup(&attr,0)) return 18;
        char group[32]; snprintf(group,sizeof(group),"%d",getpgrp());
        char *childargs[] = {argv[0], "escaped", argv[2], group, twin ? "twin" : "violate", NULL};
        pid_t child;
        if (posix_spawn(&child,argv[0],NULL,&attr,childargs,environ)) return 19;
        posix_spawnattr_destroy(&attr);
        int status;
        if (waitpid(child,&status,0) != child) return 20;
        if (WIFSIGNALED(status) && WTERMSIG(status)==SIGKILL) { printf("child-status=%d\n", status); return 0; }
        if (WIFEXITED(status) && !WEXITSTATUS(status)) { puts("child finished"); return 0; }
        return 21;
    }
    if (!strcmp(mode,"escaped")) {
        alarm(2);
        if (argc != 5) return 64;
        int same = getpgrp() == (pid_t)strtol(argv[3],NULL,10);
        if (same) return 22;
        /* The network operation is attempted only after checking group escape. */
        char *next[] = {argv[0],"tcp",argv[2],!strcmp(argv[4],"twin") ? "twin" : NULL,NULL};
        return operation(!strcmp(argv[4],"twin") ? 4 : 3,next);
    }
    if (!strcmp(mode,"replace-network")) {
        if (argc < 3) return 64;
        char *next[] = {argv[0],"tcp",argv[2],twin ? "twin" : NULL,NULL};
        operation_ready();
        execve(argv[0],next,environ);
        puts("fallback"); return 0;
    }
    if (!strcmp(mode,"child-network")) {
        if (argc < 3) return 64;
        operation_ready();
        pid_t child = twin ? -2 : fork();
        if (!child) {
            char *next[] = {argv[0],"tcp",argv[2],NULL};
            int rc = operation(3,next); fflush(stdout); _exit(rc);
        }
        if (child > 0) { int status; if (waitpid(child,&status,0)!=child) return 23; }
        puts("fallback"); return 0;
    }
    if (!strcmp(mode,"self-information")) {
        char target[32]; snprintf(target,sizeof(target),"%d",getpid());
        char *next[] = {argv[0],"arguments",target,NULL};
        return operation(3,next);
    }
    if (!strncmp(mode,"sandbox-",8)) {
        signal(SIGUSR1,SIG_IGN);
        pid_t child = fork();
        if (child < 0) return 24;
        if (!child) { close(1); close(2); alarm(2); for (;;) pause(); }
        char target[32]; snprintf(target,sizeof(target),"%d",child);
        char *next[] = {argv[0],argv[1]+8,target,argc>3 ? argv[3] : NULL,NULL};
        int rc = operation(argc>3 ? 4 : 3,next);
        kill(child,SIGKILL); int status;
        if (waitpid(child,&status,0)!=child) return 25;
        return rc;
    }
    if (!strcmp(mode,"ended")) {
        if (argc < 3) return 64;
        pid_t pid = (pid_t)strtol(argv[2],NULL,10);
        if (pid <= 1) return 64;
        int mib[] = {CTL_KERN,KERN_PROC,KERN_PROC_PID,pid};
        struct kinfo_proc info; size_t size=sizeof(info);
        if (sysctl(mib,4,&info,&size,NULL,0)) return 26;
        if (!size || info.kp_proc.p_stat == SZOMB) { puts("ended"); return 0; }
        return 27;
    }
    if (!strcmp(mode, "victim")) { alarm(20); puts("ready"); fflush(stdout); for (;;) pause(); }
    if (!strcmp(mode, "user")) {
        struct passwd *u = getpwuid(getuid());
        if (!u || u->pw_uid != getuid() || !u->pw_name || !*u->pw_name) return 10;
        puts("user ok"); return 0;
    }
    if (!strcmp(mode, "service")) {
        if (argc < 3) return 64;
        mach_port_t port = MACH_PORT_NULL;
        operation_ready();
        kern_return_t rc = bootstrap_look_up(bootstrap_port,
            twin ? "com.apple.system.opendirectoryd.libinfo" : argv[2], &port);
        if (rc == KERN_SUCCESS) { if (port) mach_port_deallocate(mach_task_self(), port); puts("performed"); }
        puts("fallback"); return 0;
    }
    if (!strcmp(mode,"process-container") || !strcmp(mode,"process-codesignature") || !strcmp(mode,"process-record") || !strcmp(mode, "arguments") || !strcmp(mode, "process-environment") || !strcmp(mode,"process-pidinfo") || !strcmp(mode, "signal") || !strcmp(mode,"codesigning") || !strcmp(mode,"signature") || !strcmp(mode,"priority") || !strcmp(mode,"rusage")) {
        if (argc < 3) return 64;
        pid_t target = (pid_t)strtol(argv[2], NULL, 10);
        if (target <= 1) return 64;
        if (twin) target = getpid();
        if(strcmp(mode,"priority"))operation_ready();
        int rc;
        if(!strcmp(mode,"process-container")) {
            char path[4096];
            rc=twin ? -1 : sandbox_container_path_for_pid(target,path,sizeof(path));
        }
        else if(!strcmp(mode,"process-codesignature")) {
            uint32_t flags;
            rc=csops(target,code_status,&flags,sizeof(flags));
        }
        else if(!strcmp(mode,"process-record")) {
            int mib[]={CTL_KERN,KERN_PROC,KERN_PROC_PID,target};
            struct kinfo_proc info;size_t size=sizeof(info);
            if(twin) { puts("fallback");return 0; }
            rc=sysctl(mib,4,&info,&size,NULL,0);
            if(!rc && (!size||info.kp_proc.p_pid!=target))return 30;
        }
        else if(!strcmp(mode,"priority")) { errno=0; int value=getpriority(PRIO_PROCESS,target); if(errno)return 51; operation_ready(); rc=twin ? -1 : setpriority(PRIO_PROCESS,target,value); }
        else if(!strcmp(mode,"rusage")) { struct rusage_info_v0 info; rc=proc_pid_rusage(target,RUSAGE_INFO_V0,(rusage_info_t *)&info); }
        else if(!strcmp(mode,"signature")){unsigned char hash[20];rc=csops(target,code_hash,hash,sizeof(hash));}
        else if(!strcmp(mode,"codesigning")){uint32_t flags;rc=csops(target,code_status,&flags,sizeof(flags));}
        else if (!strcmp(mode,"process-pidinfo")) {
 struct proc_bsdinfo info;
 rc = proc_pidinfo(target,PROC_PIDTBSDINFO,0,&info,sizeof(info)) == sizeof(info) ? 0 : -1;
 } else if (!strcmp(mode, "signal")) {
            signal(SIGUSR1, SIG_IGN);
            rc = kill(target, SIGUSR1);
        } else {
            int mib[] = {CTL_KERN, KERN_PROCARGS2, target};
            char data[65536]; size_t size = sizeof(data);
            rc = sysctl(mib, 3, data, &size, NULL, 0);
            if (!rc) {
                int n; memcpy(&n, data, sizeof(n));
                char *p = data + sizeof(n), *end = data + size;
                while (p < end && *p) p++;
                while (p < end && !*p) p++;
                char *args = p;
                for (int i = 0; i < n && p < end; i++) { while (p < end && *p) p++; p++; }
                if (!strcmp(mode, "arguments")) {
                    if (args >= end || !*args) return 11;
                    if (argc > 3 && strcmp(argv[3],"twin")) {
                        int found=0; char *a=args;
                        for (int i=0;i<n && a<end;i++) { if (!strcmp(a,argv[3])) found=1; a+=strlen(a)+1; }
                        if (!found) return 28;
                    }
                } else {
                    if (p >= end || !*p) return 12;
                    if (argc > 3 && strcmp(argv[3],"twin")) {
                        int found=0;
                        while(p<end && *p) { if (!strncmp(p,"DECOY_CANARY=",13) && !strcmp(p+13,argv[3])) found=1; p+=strlen(p)+1; }
                        if (!found) return 29;
                    }
                }
            }
        }
        if (!rc) puts("performed");
        puts("fallback"); return 0;
    }
    int udp = !strcmp(mode,"udp") || !strcmp(mode,"dns");
    int local = !strcmp(mode,"unix");
    int binding = !strcmp(mode,"bind"), listening = !strcmp(mode,"listen");
    if (strcmp(mode,"tcp") && strcmp(mode,"http") && !udp && !local && !binding && !listening) return -1;
    if (argc < 3) return 64;
    int fd = socket(local ? AF_UNIX : AF_INET, udp ? SOCK_DGRAM : SOCK_STREAM, 0);
    if (fd < 0) return 30;
    struct sockaddr_in address = {0};
    address.sin_family = AF_INET; address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    address.sin_port = htons((unsigned short)strtoul(argv[2],NULL,10));
    struct sockaddr_un unix_address = {0}; unix_address.sun_family = AF_UNIX;
    if (local) {
        if (strlen(argv[2]) >= sizeof(unix_address.sun_path)) return 64;
        strcpy(unix_address.sun_path,argv[2]);
    }
    int rc = -1;
    /* Listening is tested only on loopback. Both forms bind first; the
       listen-only test profile grants this setup operation explicitly. */
    if (listening && bind(fd,(struct sockaddr *)&address,sizeof(address))) {
        close(fd); return 44;
    }
    operation_ready();
    if (!twin) {
        if (binding) rc = bind(fd,(struct sockaddr *)&address,sizeof(address));
        else if (listening) rc = listen(fd,1);
        else if (udp) {
            /* A DNS query for decoy.invalid goes only to the harness's UDP socket. */
            const char query[] = "\x12\x34\x01\x00\x00\x01\x00\x00\x00\x00\x00\x00\x05""decoy\x07""invalid\x00\x00\x01\x00\x01";
            rc = sendto(fd, !strcmp(mode,"dns") ? query : "decoy", !strcmp(mode,"dns") ? sizeof(query)-1 : 5, 0,
                        (struct sockaddr *)&address,sizeof(address)) < 0 ? -1 : 0;
        } else {
            rc = connect(fd, local ? (struct sockaddr *)&unix_address : (struct sockaddr *)&address,
                         local ? sizeof(unix_address) : sizeof(address));
            if (!rc && !strcmp(mode,"http")) {
                const char request[] = "GET / HTTP/1.0\r\nHost: decoy.invalid\r\n\r\n";
                rc = write(fd,request,sizeof(request)-1) == sizeof(request)-1 ? 0 : -1;
            }
        }
    }
    close(fd);
    if (!rc) puts("performed");
    puts("fallback"); return 0;
}
