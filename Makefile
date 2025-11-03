
EPS_GO = out/eps-go

EPS_GO_VERSION = v0.1.0

all: $(EPS_GO)

$(EPS_GO): out FORCE
	go build -ldflags "-s -w -X main.Version=$(EPS_GO_VERSION)" \
	-o $@ .

out:
	mkdir -p out

check:
	export ROOT_PATH=$(PWD) \
	; go test ./ -v -count=1 -p=1

fmt:
	gofmt -w .

dist:
	export EPS_GO_VERSION=$(EPS_GO_VERSION) \
	; ./scripts/dist.sh

ctdist:
	podman run --rm \
	-v="./:/volume" \
	--workdir="/volume" \
	--env="EPS_GO_VERSION=$(EPS_GO_VERSION)" \
	docker.io/library/golang:alpine3.22 ./scripts/dist.sh

clean:
	rm -r out

FORCE:
