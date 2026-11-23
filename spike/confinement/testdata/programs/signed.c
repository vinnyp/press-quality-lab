#include <stdio.h>
#include <unistd.h>
int signed_page(void);
__attribute__((aligned(16384))) int main(void) {
 if (write(2,"operation-ready\n",16)!=16) return 70;
 usleep(100000);
 if (signed_page()!=7) return 1;
 puts("clean"); return 0;
}
__attribute__((noinline, aligned(16384))) int signed_page(void) { return 7; }
