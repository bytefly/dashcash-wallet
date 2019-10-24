buildTime:=$(shell date --rfc-3339=seconds)
commitID:=`git rev-parse HEAD`

all:
	@go build -ldflags "-X 'main.buildTime=${buildTime}' -X 'main.commitID=${commitID}'"
