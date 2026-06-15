MODULE := zerogravity-82/goph-keeper

.PHONY: proto

proto:
	protoc --go_out=. \
	  --go_opt=module=$(MODULE) \
	  --go-grpc_out=. \
	  --go-grpc_opt=module=$(MODULE) \
	  --go_opt=default_api_level=API_OPAQUE \
	  api/*.proto
