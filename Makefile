# beego 的编译文件
# GOFLAGS: 

.PHONY: all test clean build install

# ?= 未赋值则赋值
# $(GOFLAGS:)是一个环境变量，我只能猜测了，我猜是-gcflags
# 本地运行go tool compile --help查看相应的参数列表
# -l disable inlining
# -m print optimization decisions
# $(GOFLAGS:) 这个解释询问的作者 感谢@astaxie
GOFLAGS ?= $(GOFLAGS:)

all: install test

# ./...表示当前目录的所有子目录 感谢@astaxie
build:
	go build $(GOFLAGS) ./...

install:
	go get $(GOFLAGS) ./...

test: install
	go test $(GOFLAGS) ./...

bench: install
	go test -run=NONE -bench=. $(GOFLAGS) ./...

clean:
	go clean $(GOFLAGS) -i ./...
