.PHONY: swagger
swagger:
	go run github.com/go-swagger/go-swagger/cmd/swagger generate spec -m -o /tmp/swagger.json
	for def in `ls api/server/*_swagger.json`; do \
		jq -s  '.[0] * .[1] * {tags: (.[0].tags + .[1].tags)}' /tmp/swagger.json $$def > swagger.json; \
		cp swagger.json /tmp; \
	done
	jq -s  '.[0] * .[1]' /tmp/swagger.json api/server/swagger_base.json > swagger.json
	sed -i 's/easyjson:json//g' swagger.json