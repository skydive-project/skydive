LLC ?= llc
CLANG ?= clang

all: flow.o

%.o: %.c
	$(CLANG) \
	  -I ../../vendor/github.com/iovisor/gobpf/elf \
		-D__KERNEL__ -D__ASM_SYSREG_H -Wno-unused-value -Wno-pointer-sign \
		-Wno-compare-distinct-pointer-types \
		-O2 -emit-llvm -c $< -o -| $(LLC) -march=bpf -filetype=obj -o $@

clean:
	rm -f *.o
