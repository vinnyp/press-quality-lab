#include <sys/file.h>
#include <sys/mman.h>
#include <sys/time.h>
#include <time.h>

static volatile sig_atomic_t received;
static void receive_signal(int sig) { received = sig; }

static int ordinary(int argc, char **argv) {
    if (argc < 2 || strncmp(argv[1],"ordinary-",9)) return -1;
    const char *mode=argv[1]+9;
    if (!strcmp(mode,"devices")) {
        char bytes[32];
        int fd=open("/dev/null",O_RDWR); if(fd<0)return 80;
        if(write(fd,"x",1)!=1 || read(fd,bytes,1)!=0 || close(fd))return 81;
        fd=open("/dev/urandom",O_RDONLY); if(fd<0)return 82;
        if(read(fd,bytes,sizeof(bytes))!=sizeof(bytes)||close(fd))return 83;
        fd=open("/dev/zero",O_RDONLY); if(fd<0)return 84;
        if(read(fd,bytes,sizeof(bytes))!=sizeof(bytes)||close(fd))return 85;
        for(size_t i=0;i<sizeof(bytes);i++)if(bytes[i])return 86;
    } else if (!strcmp(mode,"streams")) {
        char c; if(read(0,&c,1)!=0)return 87;
        int fd=open("/dev/stdout",O_WRONLY);if(fd<0)return 88;
        if(write(fd,"stream\n",7)!=7||close(fd))return 89;
        fd=open("/dev/stderr",O_WRONLY);if(fd<0)return 90;
        if(write(fd,"diagnostic\n",11)!=11||close(fd))return 90;
    } else if (!strcmp(mode,"files")) {
        char path[4096],next[4096],dir[4096];
        if(snprintf(dir,sizeof(dir),"%s/ordinary.XXXXXX",getenv("TMPDIR"))>=(int)sizeof(dir)||!mkdtemp(dir))return 91;
        if(snprintf(path,sizeof(path),"%s/file",dir)>=(int)sizeof(path)||snprintf(next,sizeof(next),"%s/next",dir)>=(int)sizeof(next))return 92;
        int fd=open(path,O_RDWR|O_CREAT|O_EXCL,0600); if(fd<0)return 93;
        if(write(fd,"hello",5)!=5||flock(fd,LOCK_EX|LOCK_NB))return 94;
        char *data=mmap(NULL,5,PROT_READ|PROT_WRITE,MAP_SHARED,fd,0);
        if(data==MAP_FAILED||memcmp(data,"hello",5))return 95;
        data[0]='j';if(msync(data,5,MS_SYNC)||munmap(data,5))return 96;
        char value[5];if(pread(fd,value,5,0)!=5||memcmp(value,"jello",5)||close(fd))return 97;
        if(rename(path,next)||chmod(next,0400))return 98;
        DIR *d=opendir(dir);if(!d)return 99;int count=0;struct dirent *e;
        while((e=readdir(d)))if(strcmp(e->d_name,".")&&strcmp(e->d_name,".."))count++;
        if(closedir(d)||count!=1||unlink(next)||rmdir(dir))return 100;
    } else if (!strcmp(mode,"identity")) {
        char cwd[4096],host[256];
        if(!getcwd(cwd,sizeof(cwd))||strcmp(cwd,getenv("HOME"))||gethostname(host,sizeof(host))||!host[0])return 101;
    } else if (!strcmp(mode,"unlinked")) {
        char path[4096];if(snprintf(path,sizeof(path),"%s/unlinked.XXXXXX",getenv("TMPDIR"))>=(int)sizeof(path))return 108;
        int fd=mkstemp(path);if(fd<0)return 109;
        if(write(fd,"hello",5)!=5||lseek(fd,0,SEEK_SET)!=0||unlink(path))return 110;
        char data[5];if(read(fd,data,5)!=5||memcmp(data,"hello",5)||close(fd))return 111;
    } else if (!strcmp(mode,"timezone")) {
        if(setenv("TZ",":America/New_York",1))return 106;
        tzset();time_t epoch=946684800;struct tm local;
        if(!localtime_r(&epoch,&local)||local.tm_hour!=19||local.tm_mday!=31||local.tm_mon!=11)return 107;
    } else if (!strcmp(mode,"signal")) {
        if(signal(SIGUSR1,receive_signal)==SIG_ERR||kill(getpid(),SIGUSR1)||received!=SIGUSR1)return 102;
        int fd[2];if(pipe(fd)||close(fd[0]))return 103;
        if(signal(SIGPIPE,SIG_IGN)==SIG_ERR)return 104;
        errno=0; if(write(fd[1],"x",1)!=-1||errno!=EPIPE||close(fd[1]))return 105;
    } else return -1;
    puts("ordinary ok");return 0;
}
