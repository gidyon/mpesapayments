PROJECT_ROOT := /Users/jessegitaka/go/src/bitbucket.org/gideonkamau/ussd-gateway
SERVICE := mpesatrackingportal

JWT_KEY := hDI0eBv11TbuboZ01qpnOuYRYLh6gQUOQhC9Mfagzv9l3gJso7CalTt7wGzJCVwbeDIfOX6fwS79pnisW7udhQ
API_BLOCK_KEY := 9AI8o4ta02gdqWsVhYe0r276z7my6yDwY78rCsrcofT7pCNq4WwnRoW93hn8WFJM0HheZHDYPc4tD+tUXVNEGw
API_HASH_KEY := 73H/I3+27Qp3ZETqMzbYa/EGT826Zxx2821cmHUl7fTX/DmkIWPJatczkxN3p8RHbdOGWT/HDRAf7gqhZcZOow

image := nofno/$(SERVICE):latest
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
	MPESA_ACCESS_TOKEN=sfwdyd892dv8873dv \
	MPESA_ACCESS_TOKEN_URL=https://sandbox.safaricom.co.ke/oauth/v1/generate?grant_type=client_credentials \
	SAF_CONSUMER_KEY=7iFrAmDprJJ5rS9k9JTaGZ8Sm2kHb8r8 \
	SAF_CONSUMER_SECRET=UNKLK3bm1hI0tbl0 \
	BUSINESS_SHORT_CODE=174379 \
	MPESA_ACCOUNT_REFERENCE=edefefe \
	MPESA_ACCESS_TIMESTAMP=20171121160303 \
	MPESA_ACCESS_PASSWORD=MTc0Mzc5YmZiMjc5ZjlhYTliZGJjZjE1OGU5N2RkNzFhNDY3Y2QyZTBjODkzMDU5YjEwZjc4ZTZiNzJhZGExZWQyYzkxOTIwMTcxMTIxMTYwMzAz \
	MPESA_CALLBACK_URL=https://onfon.gidyon.jessegitaka.org/api/mpesa/incoming/stkpush \
	MPESA_POST_URL=https://sandbox.safaricom.co.ke/mpesa/stkpush/v1/processrequest \
	MPESA_QUERY_URL=https://sandbox.safaricom.co.ke/mpesa/stkpushquery/v1/query \
	./service -config-file=./config.yaml

gotest:
	@cd $(PROJECT_ROOT)/internal/mpesapayment && ginkgo -cover
	
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



