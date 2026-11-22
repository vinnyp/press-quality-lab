#include <stdio.h>
#include <unistd.h>
int main(void) { alarm(4); puts("performed"); puts("fallback"); return 0; }
