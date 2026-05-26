export CGO_ENABLED := "0"
export GOFLAGS := "-tags=remote,containers_image_openpgp,exclude_graphdriver_btrfs"

build:
	cd src; go build .
run:
	cd src; go run .
