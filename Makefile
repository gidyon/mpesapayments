PROJECT_NAME := mpesa-tracking-portal
PROJECT_ROOT := /Users/jessegitaka/go/src/bitbucket.org/gideonkamau/tpgo
PKG := bitbucket.org/gideonkamau/$(PROJECT_NAME)
API_IN_PATH := api/proto
API_OUT_PATH := pkg/api
SWAGGER_DOC_OUT_PATH := api/swagger

setup_dev: ## Sets up a development environment for the emrs project
	@cd deployments/compose/dev &&\
	docker-compose up -d

setup_redis:
	@cd deployments/compose/dev &&\
	docker-compose up -d redis

teardown_dev: ## Tear down development environment for the emrs project
	@cd deployments/compose/dev &&\
	docker-compose down

protoc_mpesa_payment:
	@protoc -I=$(API_IN_PATH) -I=third_party --go_out=plugins=grpc:$(API_OUT_PATH)/mpesapayment --go_opt=paths=source_relative mpesa_payment.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --grpc-gateway_out=logtostderr=true,paths=source_relative:$(API_OUT_PATH)/mpesapayment mpesa_payment.proto
	@protoc -I=$(API_IN_PATH) -I=third_party --swagger_out=logtostderr=true:$(SWAGGER_DOC_OUT_PATH) mpesa_payment.proto

protoc_all: protoc_mpesa_payment

run_mpesa_payment:
	cd cmd/mpesapayment && make run

run_apidoc:
	cd cmd/apidoc && make run

build_mpesa_payment:
	cd cmd/mpesapayment && sudo make build_service

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
