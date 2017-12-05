#ifndef __BPF_H
#define __BPF_H

#define _GNU_SOURCE

// from iovisor/gobpf
#include "include/bpf.h"

/* helper macro to place different sections in eBPF elf file. This is a generic
 * macro, more specific macro should be used instead of this one.
 */
#define SEC(NAME) __attribute__((section(NAME), used))

/* helper marcro to define a map, socket, kprobe section in the
 * eBPF elf file.
 */
#define MAP(NAME) struct bpf_map_def __attribute__((section("maps/"#NAME), used)) NAME =
#define SOCKET(NAME) __attribute__((section("socket_"#NAME), used))
#define LICENSE SEC("license")

/* llvm built-in functions */
unsigned long long load_byte(void *skb,
  unsigned long long off) asm("llvm.bpf.load.byte");
unsigned long long load_half(void *skb,
  unsigned long long off) asm("llvm.bpf.load.half");
unsigned long long load_word(void *skb,
  unsigned long long off) asm("llvm.bpf.load.word");

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
static unsigned long long (*bpf_get_smp_processor_id)(void) =
  (void *) BPF_FUNC_get_smp_processor_id;
static unsigned long long (*bpf_get_current_pid_tgid)(void) =
  (void *) BPF_FUNC_get_current_pid_tgid;
static unsigned long long (*bpf_get_current_uid_gid)(void) =
  (void *) BPF_FUNC_get_current_uid_gid;

#define bpf_printk(fmt, ...) \
({ \
  char ____fmt[] = fmt; \
  bpf_trace_printk(____fmt, sizeof(____fmt), \
  ##__VA_ARGS__); \
})

#endif
