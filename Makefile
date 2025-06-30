
export GO111MODULE=on

.PHONY: all
all: test bin

.PHONY: test
test:
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: bin
bin: fmt vet
	go build -o bin/kubectl-unmount_pvs github.com/dancavallaro/kubectl-unmount-pvs/cmd/plugin

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ./pkg/... ./cmd/...

.PHONY: clean
clean:
	rm -rf bin