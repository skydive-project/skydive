#ifndef __BPF_H
#define __BPF_H

#define _GNU_SOURCE

// from iovisor/gobpf
#include "include/bpf.h"
#include "include/bpf_map.h"

#ifndef __inline
#define __inline inline __attribute__((always_inline))
#endif

#ifndef __section
#define __section(NAME)                  \
   __attribute__((section(NAME), used))
#endif

#ifndef memset
# define memset(dest, chr, n)   __builtin_memset((dest), (chr), (n))
#endif

#ifndef memcpy
# define memcpy(dest, src, n)   __builtin_memcpy((dest), (src), (n))
#endif

#ifndef memmove
# define memmove(dest, src, n)  __builtin_memmove((dest), (src), (n))
#endif

/* helper marcro to define a map, socket, kprobe section in the
 * eBPF elf file.
 */
#define MAP(NAME) struct bpf_map_def __section("maps/"#NAME) NAME =
#define SOCKET(NAME) __section("socket_"#NAME)
#define LICENSE __section("license")

/* llvm built-in functions */
unsigned long long load_byte(void *skb,
  unsigned long long off) asm("llvm.bpf.load.byte");
unsigned long long load_half(void *skb,
  unsigned long long off) asm("llvm.bpf.load.half");
unsigned long long load_word(void *skb,
  unsigned long long off) asm("llvm.bpf.load.word");

static __inline __u16 bpf_ntohs(__u16 val) {
	return (val << 8) | (val >> 8);
}

/* helper functions called from eBPF programs written in C
 */
static void *(*bpf_map_lookup_element)(void *map, void *key) =
  (void *) BPF_FUNC_map_lookup_elem;
static int (*bpf_map_update_element)(void *map, void *key, void *value,
  unsigned long long flags) = (void *) BPF_FUNC_map_update_elem;
static int (*bpf_map_delete_element)(void *map, void *key) =
  (void *) BPF_FUNC_map_delete_elem;
static unsigned long long (*bpf_ktime_get_ns)(void) =
  (void *) BPF_FUNC_ktime_get_ns;
static int (*bpf_trace_printk)(const char *fmt, int fmt_size, ...) =
  (void *) BPF_FUNC_trace_printk;
static void (*bpf_tail_call)(void *ctx, void *map, int index) =
  (void *) BPF_FUNC_tail_call;
static void (*bpf_skb_load_bytes)(void *skb, int offset, void *to, int len) =
  (void *) BPF_FUNC_skb_load_bytes;
static unsigned long long (*bpf_get_smp_processor_id)(void) =
  (void *) BPF_FUNC_get_smp_processor_id;
static unsigned long long (*bpf_get_current_pid_tgid)(void) =
  (void *) BPF_FUNC_get_current_pid_tgid;
static unsigned long long (*bpf_get_current_uid_gid)(void) =
  (void *) BPF_FUNC_get_current_uid_gid;

#define DEBUG
#ifdef DEBUG
#define bpf_printk(fmt, ...) \
({ \
  char ____fmt[] = fmt; \
  bpf_trace_printk(____fmt, sizeof(____fmt), \
  ##__VA_ARGS__); \
})
#else
#define bpf_printk(fmt, ...)
#endif
#endif
