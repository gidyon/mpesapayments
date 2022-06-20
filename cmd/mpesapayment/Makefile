PROJECT_ROOT := /Users/jessegitaka/go/src/bitbucket.org/gideonkamau/ussd-gateway
SERVICE := mpesapayment

JWT_KEY := hDI0eBv11TbuboZ01qpnOuYRYLh6gQUOQhC9Mfagzv9l3gJso7CalTt7wGzJCVwbeDIfOX6fwS79pnisW7udhQ

image := gidyon/$(SERVICE)
context := .

ifdef IMAGE
	image=$(IMAGE)
else
	imagex := $(image)
	ifdef tag
		image=$(imagex):$(tag)
	else
		image=$(imagex):latest
	endif
endif

ifdef BUILD_CONTEXT
	context=$(BUILD_CONTEXT)
endif

run:
	go build -v -o service && \
	DISABLE_STK_SERVICE=true
	DISABLE_MPESA_SERVICE=true
	JWT_SIGNING_KEY=$(JWT_KEY) \
	SAF_CONSUMER_KEY=Ne5FBXQ2GQHmy8t3hsMmteh7X5k1zyzv \
	SAF_CONSUMER_SECRET=rjPVlpAQxmGUcl0W \
	MPESA_ACCESS_TOKEN=PmI1UA2pkJOXhxWwqhHmo2b72QZA \
	MPESA_ACCESS_TOKEN_URL=https://api.safaricom.co.ke/oauth/v1/generate?grant_type=client_credentials \
	REDIS_KEY_PREFIX="test" \
	TABLE_PREFIX="test" \
	B2C_QUEUE_TIMEOUT_URL=https://b599a397bfc2.ngrok.io/api/mpestx/incoming/b2c \
	B2C_RESULT_URL=https://b599a397bfc2.ngrok.io/api/mpestx/incoming/b2c \
	B2C_RESULT_URL_V2=https://b599a397bfc2.ngrok.io/api/mpestx/incoming/b2c \
	B2C_QUERY_BALANCE_URL=https://api.safaricom.co.ke/mpesa/accountbalance/v1/query \
	B2C_URL=https://api.safaricom.co.ke/mpesa/b2c/v1/paymentrequest \
	B2C_REVERSAL_URL=https://api.safaricom.co.ke/mpesa/reversal/v1/request \
	B2C_PUBLISH_CHANNEL=mpesa:btc:pubsub \
	B2C_PUBLISH_CHANNEL_LOCAL=local \
	B2C_INITIATOR_USERNAME=JESEPHINE \
	B2C_INITIATOR_ENCRYPTED_PASSWORD=WDZVY3FeA1dyjX77cfIBUNfHtbsU3wElZT74Vab37GKQtRCM3ZohaMQyAbL/0rwzzn6z7QoCgnOz+HJVru02yIFbCti+USwaI+JLKqapEDeyCJhNitpqm5ALxwFDJcipmhTFY4GdtZCKmWdN0twiJ+nnGmWXc7+0WUhBg4pcANsbe4sMzU+UQjVQ1Qmi8yU6vqLpJKQslJSSpMoT93PV9re75CbDXiqqkDcTuuBm1D3Ts7jaO0OWHbb44fl9h+HngBufI4QQsXZnSb9cGOEM7sYxij4LqXMehrt+fe5Xx469+vv9ua70iboL6/f9U0jzJnYiUbQhtWyN7qLFZMcl/g== \
	STK_BUSINESS_SHORT_CODE=174379 \
	STK_MPESA_ACCOUNT_REFERENCE='mpesa stk test' \
	STK_MPESA_ACCESS_TIMESTAMP=20171121160303 \
	STK_LNM_PASSKEY=bfb279f9aa9bdbcf158e97dd71a467cd2e0c893059b10f78e6b72ada1ed2c919 \
	STK_RESULT_URL=https://onfon.gidyon.jessegitaka.org/api/mpesa/incoming/stkpush \
	STK_RESULT_URL_V2=https://onfon.gidyon.jessegitaka.org/api/mpesa/incoming/stkpush \
	STK_MPESA_POST_URL=https://api.safaricom.co.ke/mpesa/stkpush/v1/processrequest \
	STK_MPESA_QUERY_URL=https://api.safaricom.co.ke/mpesa/stkpushquery/v1/query \
	STK_PUBLISH_CHANNEL=mpesa:stk:pub \
	C2B_PUBLISH_CHANNEL=mpesa:paybill:pubsub \
	B2C_STATS_TABLE=ofoefef \
	B2C_TRANSACTIONS_TABLE=ooefoe \
	./service -config-file=./config.yaml

gotest:
	@cd $(PROJECT_ROOT)/internal/mpesapayment && ginkgo -cover
	
compile:
	@GOOS=linux CGO_ENABLED=0 go build -tags netgo -installsuffix netgo -v -o service .

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



