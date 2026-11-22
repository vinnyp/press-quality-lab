#include <sqlite3.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/stat.h>

int main(int argc,char **argv) {
    alarm(4);
    if (argc!=3) return 64;
    sqlite3 *db=NULL;
    if (sqlite3_open(argv[2],&db)!=SQLITE_OK) return 1;
    if (!strcmp(argv[1],"write")) {
        if (sqlite3_exec(db,"PRAGMA journal_mode=WAL; CREATE TABLE values_table(value); INSERT INTO values_table VALUES(7);",NULL,NULL,NULL)!=SQLITE_OK) return 2;
        if (sqlite3_close(db)!=SQLITE_OK) return 3;
        puts("written 7");return 0;
    }
    if (strcmp(argv[1],"read")) return 64;
    sqlite3_stmt *stmt=NULL;
    if (sqlite3_prepare_v2(db,"SELECT value FROM values_table",-1,&stmt,NULL)!=SQLITE_OK) return 4;
    if (sqlite3_step(stmt)!=SQLITE_ROW || sqlite3_column_int(stmt,0)!=7) return 5;
    char wal[4096],shm[4096];struct stat st;
    if (snprintf(wal,sizeof(wal),"%s-wal",argv[2])>=(int)sizeof(wal) || snprintf(shm,sizeof(shm),"%s-shm",argv[2])>=(int)sizeof(shm)) return 6;
    if (stat(wal,&st) || stat(shm,&st)) return 7;
    puts("side-files=wal,shm");
    if (sqlite3_finalize(stmt)!=SQLITE_OK || sqlite3_close(db)!=SQLITE_OK) return 8;
    puts("value=7");return 0;
}
