define PROTOC_GEN
go get github.com/gogo/protobuf/protoc-gen-gogofaster@v1.3.1
go get github.com/golang/protobuf/protoc-gen-go@v1.3.2
protoc -I. -Iflow/layers -I$${GOPATH}/pkg/mod/github.com/gogo/protobuf@v1.3.1 --plugin=$${GOPATH}/bin/protoc-gen-gogofaster --gogofaster_out $$GOPATH/src $1
endef

GEN_PROTO_FILES = $(patsubst %.proto,%.pb.go,$(shell find . -name *.proto | grep -v ^./vendor))

%.pb.go: %.proto
	$(call PROTOC_GEN,$<)

flow/flow.pb.go: flow/flow.proto filters/filters.proto
	$(call PROTOC_GEN,flow/flow.proto)

	# always export flow.ParentUUID as we need to store this information to know
	# if it's a Outer or Inner packet.
	sed -e 's/ParentUUID\(.*\),omitempty\(.*\)/ParentUUID\1\2/' \
		-e 's/Protocol\(.*\),omitempty\(.*\)/Protocol\1\2/' \
		-e 's/ICMPType\(.*\),omitempty\(.*\)/ICMPType\1\2/' \
		-e 's/int64\(.*\),omitempty\(.*\)/int64\1\2/' \
		-i $@
	# add omitempty to RTT as it is not always filled
	sed -e 's/json:"RTT"/json:"RTT,omitempty"/' -i $@
	# do not export LastRawPackets used internally
	sed -e 's/json:"LastRawPackets,omitempty"/json:"-"/g' -i $@
	# add flowState to flow generated struct
	sed -e 's/type Flow struct {/type Flow struct { XXX_state flowState `json:"-"`/' -i $@
	# to fix generated layers import
	sed -e 's/layers "flow\/layers"/layers "github.com\/skydive-project\/skydive\/flow\/layers"/' -i $@
	sed -e 's/type FlowMetric struct {/\/\/ gendecoder\ntype FlowMetric struct {/' -i $@
	sed -e 's/type FlowLayer struct {/\/\/ gendecoder\ntype FlowLayer struct {/' -i $@
	sed -e 's/type TransportLayer struct {/\/\/ gendecoder\ntype TransportLayer struct {/' -i $@
	sed -e 's/type ICMPLayer struct {/\/\/ gendecoder\ntype ICMPLayer struct {/' -i $@
	sed -e 's/type IPMetric struct {/\/\/ gendecoder\ntype IPMetric struct {/' -i $@
	sed -e 's/type TCPMetric struct {/\/\/ gendecoder\ntype TCPMetric struct {/' -i $@
	# This is to allow calling go generate on flow/flow.pb.go
	sed -e 's/DO NOT EDIT./DO NOT MODIFY/' -i $@
	sed '1 i //go:generate go run github.com/skydive-project/skydive/scripts/gendecoder' -i $@
	gofmt -s -w $@

flow/flow.pb_easyjson.go: flow/flow.pb.go
	go run github.com/mailru/easyjson/easyjson -all $<

websocket/structmessage.pb.go: websocket/structmessage.proto
	$(call PROTOC_GEN,$<)

	sed -e 's/type StructMessage struct {/type StructMessage struct { XXX_state structMessageState `json:"-"`/' -i websocket/structmessage.pb.go
	gofmt -s -w $@

.proto: $(GEN_PROTO_FILES)

.PHONY: .proto.clean
.proto.clean:
	find . \( -name *.pb.go ! -path './vendor/*' \) -exec rm {} \;
	rm -rf flow/layers/generated.proto
