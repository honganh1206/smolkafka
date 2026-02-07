# Where to put generated certs
CONFIG_PATH=${HOME}/.smolkafka/
TAG ?= 0.0.1

.PHONY: init
init:
	mkdir -p ${CONFIG_PATH}

.PHONY: gencert
gencert:
	# Generate CA's private key and matching CA cert
	cfssl gencert \
		-initca tests/ca-csr.json | cfssljson -bare ca

	# Produce server's certificate and its private key
	# How it works
	# Take a CSR, sign it with provided CA materials
	# Output to a JSON payload containing certificate, private key and metadata
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=tests/ca-config.json \
		-profile=server \
		tests/server-csr.json | cfssljson -bare server

	# Produce client's certificate and private key
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=tests/ca-config.json \
		-profile=client \
		tests/client-csr.json | cfssljson -bare client

	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=tests/ca-config.json \
		-profile=client \
		-cn="root" \
		tests/client-csr.json | cfssljson -bare root-client

	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=tests/ca-config.json \
		-profile=client \
		-cn="nobody" \
		tests/client-csr.json | cfssljson -bare nobody-client

	mv *.pem *.csr ${CONFIG_PATH}

.PHONY: compile
compile:
	protoc api/v1/*.proto \
			--go_out=. \
			--go-grpc_out=. \
			--go_opt=paths=source_relative \
			--go-grpc_opt=paths=source_relative \
			--proto_path=.

$(CONFIG_PATH)/model.conf:
	cp tests/model.conf $(CONFIG_PATH)/model.conf
$(CONFIG_PATH)/policy.csv:
	cp tests/policy.csv $(CONFIG_PATH)/policy.csv

.PHONY: test
test: $(CONFIG_PATH)/policy.csv $(CONFIG_PATH)/model.conf
	go test -race ./...

build-docker:
	docker build -t github.com/honganh1206/smolkafka:$(TAG) .
