PROG := dummy

SRCS := \
	main.go \

.PHONY: all
all:
	go build -o $(PROG) $(SRCS)

.PHONY: clean
clean:
	rm -rf $(PROG)

.PHONY: run
run:
	./$(PROG) $(PROG).yml.default
