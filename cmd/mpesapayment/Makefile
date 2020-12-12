PROJECT_ROOT := /Users/jessegitaka/go/src/bitbucket.org/gideonkamau/ussd-gateway
SERVICE := mpesapayment

JWT_KEY := hDI0eBv11TbuboZ01qpnOuYRYLh6gQUOQhC9Mfagzv9l3gJso7CalTt7wGzJCVwbeDIfOX6fwS79pnisW7udhQ
API_BLOCK_KEY := 9AI8o4ta02gdqWsVhYe0r276z7my6yDwY78rCsrcofT7pCNq4WwnRoW93hn8WFJM0HheZHDYPc4tD+tUXVNEGw
API_HASH_KEY := 73H/I3+27Qp3ZETqMzbYa/EGT826Zxx2821cmHUl7fTX/DmkIWPJatczkxN3p8RHbdOGWT/HDRAf7gqhZcZOow

image := gidyon/$(SERVICE):v2
context := .

ifdef IMAGE
	image=$(IMAGE)
endif

ifdef BUILD_CONTEXT
	context=$(BUILD_CONTEXT)
endif

run:
	go build -v -o service && \
	JWT_SIGNING_KEY=$(JWT_KEY) \
	API_BLOCK_KEY=$(API_BLOCK_KEY) \
	API_HASH_KEY=$(API_HASH_KEY) \
	MPESA_ACCESS_TOKEN=PmI1UA2pkJOXhxWwqhHmo2b72QZA \
	MPESA_ACCESS_TOKEN_URL=https://sandbox.safaricom.co.ke/oauth/v1/generate?grant_type=client_credentials \
	SAF_CONSUMER_KEY=QggPD8t1QTEyRDrVOQi9roaEF1KTi8yn \
	SAF_CONSUMER_SECRET=vtDlJKRFAVREwffQ \
	BUSINESS_SHORT_CODE=174379 \
	MPESA_ACCOUNT_REFERENCE='mpesa stk test' \
	MPESA_ACCESS_TIMESTAMP=20171121160303 \
	LNM_PASSKEY=bfb279f9aa9bdbcf158e97dd71a467cd2e0c893059b10f78e6b72ada1ed2c919 \
	MPESA_CALLBACK_URL=https://onfon.gidyon.jessegitaka.org/api/mpesa/incoming/stkpush \
	MPESA_POST_URL=https://sandbox.safaricom.co.ke/mpesa/stkpush/v1/processrequest \
	MPESA_QUERY_URL=https://sandbox.safaricom.co.ke/mpesa/stkpushquery/v1/query \
	PUBLISH_CHANNEL_STK=mpesa:stk:pub \
	PUBLISH_CHANNEL_PAYBILL=mpesa:pyabill:pubsub \
	./service -config-file=./config.yaml

gotest:
	@cd $(PROJECT_ROOT)/internal/mpesapayment && ginkgo -cover
	
compile:
	@GOOS=linux CGO_ENABLED=0 go build -a -installsuffix cgo -v -o service .

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



