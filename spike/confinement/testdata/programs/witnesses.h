#include <IOKit/IOKitLib.h>
#include <dlfcn.h>
#include <sys/syscall.h>
#include <netdb.h>
#include <dirent.h>
#include <sys/file.h>
#include <sys/mount.h>
#include <sys/ioctl.h>
#include <sys/resource.h>

static int witness(int argc, char **argv) {
    if (argc < 2) return -1;
    const char *mode = argv[1];
    if (!strcmp(mode,"extension-token")) {
        if(argc!=4)return 64;
        char *(*issue)(const char *,const char *,uint32_t)=dlsym(RTLD_DEFAULT,"sandbox_extension_issue_file");
        if(!issue)return 72;
        char *token=issue("com.apple.app-sandbox.read",argv[2],0);
        if(!token)return 73;
        size_t length=strlen(token);
        if(!length || length>=65536){free(token);return 74;}
        int fd=open(argv[3],O_WRONLY|O_CREAT|O_EXCL,0600);
        if(fd<0){free(token);return 75;}
        int ok=write(fd,token,length)==(ssize_t)length;
        free(token);
        if(close(fd) || !ok)return 76;
        puts("issued");return 0;
    }
    if (!strcmp(mode, "resolver")) {
        struct addrinfo *answer = NULL;
        operation_ready();
        int rc = getaddrinfo("decoy.invalid.", NULL, NULL, &answer);
        if (answer) freeaddrinfo(answer);
        if (!rc) return 45;
        puts("lookup failed"); return 0;
    }
    if (!strcmp(mode, "preboot")) {
        if (argc < 3) return 64;
        int twin = !strcmp(argv[argc-1], "twin");
        operation_ready();
        DIR *dir = opendir(twin ? getenv("HOME") : argv[2]);
        if (dir) { while (readdir(dir)) {} closedir(dir); puts("performed"); }
        puts("fallback"); return 0;
    }
    if (!strcmp(mode,"replace-tool")) {
        if(argc<3)return 64;
        operation_ready();
        execv(argv[2],&argv[2]);puts("fallback");return 0;
    }
    if (!strcmp(mode, "system-child")) {
        if (argc < 4) return 64;
        pid_t child = fork();
        if (child < 0) return 46;
        if (!child) {
            alarm(2);
            operation_ready();
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
    if(strcmp(op,"properties") && strcmp(op,"extension"))operation_ready();
    if (!strcmp(op,"properties")) {
        /* Read only public kernel-build metadata; never open a user client. */
        void *core=dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation",RTLD_LAZY|RTLD_LOCAL);
        void *io=dlopen("/System/Library/Frameworks/IOKit.framework/IOKit",RTLD_LAZY|RTLD_LOCAL);
        if(!core || !io)return 67;
        CFStringRef (*make_string)(CFAllocatorRef,const char *,CFStringEncoding)=dlsym(core,"CFStringCreateWithCString");
        void (*release)(CFTypeRef)=dlsym(core,"CFRelease");
        CFTypeID (*type_id)(CFTypeRef)=dlsym(core,"CFGetTypeID");
        CFTypeID (*string_type)(void)=dlsym(core,"CFStringGetTypeID");
        CFIndex (*string_length)(CFStringRef)=dlsym(core,"CFStringGetLength");
        io_registry_entry_t (*root)(mach_port_t)=dlsym(io,"IORegistryGetRootEntry");
        CFTypeRef (*property)(io_registry_entry_t,CFStringRef,CFAllocatorRef,IOOptionBits)=dlsym(io,"IORegistryEntryCreateCFProperty");
        kern_return_t (*release_service)(io_object_t)=dlsym(io,"IOObjectRelease");
        if(!make_string || !release || !type_id || !string_type || !string_length || !root || !property || !release_service)return 68;
        io_registry_entry_t service=root(0);if(!service)return 70;
        CFStringRef key=make_string(NULL,"IOKitBuildVersion",kCFStringEncodingUTF8);
        if(!key)return 71;
        operation_ready();
        if(!twin) {
            CFTypeRef value=property(service,key,NULL,0);
            if(value) {rc=type_id(value)==string_type() && string_length(value)>0 ? 0 : -1;release(value);}
        }
        release(key);release_service(service);dlclose(io);dlclose(core);
    } else if (!strcmp(op,"extension")) {
        long (*consume)(const char *)=dlsym(RTLD_DEFAULT,"sandbox_extension_consume");
        if(!consume)return 65;
        char token[65536];
        fd=open(argv[2],O_RDONLY);if(fd<0)return 77;
        ssize_t length=read(fd,token,sizeof(token)-1);
        if(close(fd) || length<=0)return 78;
        fd=-1;token[length]=0;
        /* Swallow library diagnostics, not the API result used by the control.
           Readiness still goes to the original diagnostic channel. */
        int diagnostic=dup(2),sink=open("/dev/null",O_WRONLY);
        if(diagnostic<0 || sink<0)return 79;
        if(dup2(sink,2)<0)return 80;
        if(write(diagnostic,"operation-ready\n",16)!=16)return 81;
        if(!twin)rc=consume(token)>=0 ? 0 : -1;
        if(dup2(diagnostic,2)<0 || close(diagnostic) || close(sink))return 82;
    } else if (!strcmp(op,"task-port")) {
        mach_port_t port=MACH_PORT_NULL;
        if(!twin)rc=task_get_special_port(mach_task_self(),TASK_HOST_PORT,&port)==KERN_SUCCESS ? 0 : -1;
        if(port)mach_port_deallocate(mach_task_self(),port);
    } else if (!strcmp(op,"exception-port")) {
        if(!twin)rc=task_set_exception_ports(mach_task_self(),EXC_MASK_BAD_ACCESS,MACH_PORT_NULL,EXCEPTION_DEFAULT,THREAD_STATE_NONE)==KERN_SUCCESS ? 0 : -1;
    } else if (!strcmp(op,"necp")) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        if(!twin) {fd=(int)syscall(SYS_necp_open,0);rc=fd;}
#pragma clang diagnostic pop
    } else if (!strcmp(op,"memory")) {
        /* GET_MEMLIMIT_EXCESS reads only this probe's memory limits. */
        int limits[4]={0};
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        if(!twin)rc=(int)syscall(SYS_memorystatus_control,11,getpid(),0,limits,sizeof(uint64_t));
#pragma clang diagnostic pop
    } else if (!strcmp(op,"coalition")) {
        /* Obtain only this probe's coalition ids, then make a read-only query. */
        uint64_t ids[2]={0};
        if(proc_pidinfo(getpid(),20 /* own coalition ids */,0,ids,sizeof(ids))!=(int)sizeof(ids))return 66;
        uint64_t data[64]={0};size_t length=sizeof(data);
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        if(!twin)rc=(int)syscall(SYS_coalition_info,ids[0],1,data,&length);
#pragma clang diagnostic pop
    } else if (!strcmp(op, "xattr")) {
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
