SRCS := \
	classify.go \
	compress.go \
	encode.go \
	filter.go \
	pipeline.go \
	s3_client.go \
	store.go \
	store_stdout.go \
	store_s3.go \
	subscriber.go \
	transform.go \

TESTS := \
	store_s3_test.go \

.PHONY: test
test:
	go test $(SRCS) $(TESTS)

