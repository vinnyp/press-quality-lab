#include "textflag.h"

TEXT bootstrapLookupTrampoline<>(SB),NOSPLIT,$0-0
	JMP pql_bootstrap_lookup(SB)

GLOBL ·bootstrapLookupAddr(SB),RODATA,$8
DATA ·bootstrapLookupAddr(SB)/8,$bootstrapLookupTrampoline<>(SB)

TEXT resourceUsageTrampoline<>(SB),NOSPLIT,$0-0
	JMP pql_resource_usage(SB)
GLOBL ·resourceUsageAddr(SB),RODATA,$8
DATA ·resourceUsageAddr(SB)/8,$resourceUsageTrampoline<>(SB)

TEXT codeSignatureTrampoline<>(SB),NOSPLIT,$0-0
	JMP pql_code_signature(SB)
GLOBL ·codeSignatureAddr(SB),RODATA,$8
DATA ·codeSignatureAddr(SB)/8,$codeSignatureTrampoline<>(SB)

TEXT sysctlTrampoline<>(SB),NOSPLIT,$0-0
	JMP pql_sysctl(SB)
GLOBL ·sysctlAddr(SB),RODATA,$8
DATA ·sysctlAddr(SB)/8,$sysctlTrampoline<>(SB)

TEXT ·bootstrapPort(SB),NOSPLIT,$0-4
	MOVD $pql_bootstrap_port(SB), R0
	MOVWU (R0), R0
	MOVW R0, ret+0(FP)
	RET
