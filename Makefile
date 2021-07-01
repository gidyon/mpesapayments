PROJECT_NAME := mpesa-tracking-portal
PROJECT_ROOT := /Users/jessegitaka/go/src/bitbucket.org/gideonkamau/tpgo
PKG := bitbucket.org/gideonkamau/$(PROJECT_NAME)
API_IN_PATH := api/proto
API_OUT_PATH := pkg/api
OPEN_API_V2_OUT_PATH := api/swagger

setup_dev: ## Sets up a development environment for the emrs project
	@cd deployments/compose/dev &&\
	docker-compose up -d

setup_redis:
	@cd deployments/compose/dev &&\
	docker-compose up -d redis

teardown_dev: ## Tear down development environment for the emrs project
	@cd deployments/compose/dev &&\
	docker-compose down

protoc_c2b:
	@protoc -I=$(API_IN_PATH) -I=third_party --go-grpc_out=$(API_OUT_PATH)/c2b --go-grpc_opt=paths=source_relative --go_out=$(API_OUT_PATH)/c2b --go_opt=paths=source_relative c2b.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --grpc-gateway_out=logtostderr=true,paths=source_relative:$(API_OUT_PATH)/c2b c2b.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --openapiv2_out=logtostderr=true,repeated_path_param_separator=ssv:$(OPEN_API_V2_OUT_PATH) c2b.proto

protoc_mpesa_stk:
	@protoc -I=$(API_IN_PATH) -I=third_party --go-grpc_out=$(API_OUT_PATH)/stk --go-grpc_opt=paths=source_relative --go_out=$(API_OUT_PATH)/stk --go_opt=paths=source_relative stk.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --grpc-gateway_out=logtostderr=true,paths=source_relative:$(API_OUT_PATH)/stk stk.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --openapiv2_out=logtostderr=true,repeated_path_param_separator=ssv:$(OPEN_API_V2_OUT_PATH) stk.proto

protoc_b2c:
	@protoc -I=$(API_IN_PATH) -I=third_party --go-grpc_out=$(API_OUT_PATH)/b2c --go-grpc_opt=paths=source_relative --go_out=$(API_OUT_PATH)/b2c --go_opt=paths=source_relative b2c.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --grpc-gateway_out=logtostderr=true,paths=source_relative:$(API_OUT_PATH)/b2c b2c.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --openapiv2_out=logtostderr=true,repeated_path_param_separator=ssv:$(OPEN_API_V2_OUT_PATH) b2c.proto

copy_documentation:
	@cp -r $(OPEN_API_V2_OUT_PATH) cmd/apidoc/dist

protoc_all: protoc_c2b protoc_mpesa_stk protoc_b2c copy_documentation

run_c2b:
	cd cmd/mpesapayment && make run

run_apidoc:
	cd cmd/apidoc && make run

build_c2b:
	cd cmd/mpesapayment && sudo make build_service

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
