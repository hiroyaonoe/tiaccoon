package main

import (
	"github.com/hiroyaonoe/tiaccoon/pkg/cni"
	"github.com/hiroyaonoe/tiaccoon/pkg/version"
)

func main() {
	h := &cni.Handler{}
	h.Start(version.Version)
}
