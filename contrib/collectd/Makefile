COLLECTD_SRC?=/usr/local/src/collectd

collectd:
	CGO_CFLAGS="-I${COLLECTD_SRC}/src/ -I${COLLECTD_SRC}/src/daemon" go build -o skydive.so -tags collectd -buildmode=c-shared skydive.go logging.go

clean:
	rm -rf skydive.h skydive.so
