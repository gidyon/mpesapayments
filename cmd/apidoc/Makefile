PROJECT_ROOT := /Users/jessegitaka/go/src/bitbucket.org/gideonkamau/ussd-gateway
SERVICE := mpesapaymentapidoc

image := nofno/$(SERVICE):latest
context := .

ifdef IMAGE
	image=$(IMAGE)
endif

ifdef BUILD_CONTEXT
	context=$(BUILD_CONTEXT)
endif

run:
	cp -r /Users/jessegitaka/Desktop/tpgo/api/swagger /Users/jessegitaka/Desktop/tpgo/cmd/apidoc/dist &&\
	go build -v -o service && \
	./service

compile:
	@GOOS=linux go build -i -v -o service .

docker_build:
	@docker build -t $(image) .

docker_tag:
	@docker tag $(image) $(image)

docker_push:
	@docker push $(image)

build_service: compile docker_build docker_tag docker_push

deploy:
	@kubectl delete -f deploy.yaml && kubectl apply -f deploy.yaml

build_and_deploy: gotest compile docker_build docker_tag docker_push deploy



