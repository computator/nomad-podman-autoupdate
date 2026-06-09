export CGO_ENABLED := "0"
export GOFLAGS := "-tags=remote,containers_image_openpgp,exclude_graphdriver_btrfs"

build:
	go build .
run:
	go run .
