package helpers

import (
	"time"
)

var yesterday = time.Now().Add(-time.Hour * 24)
var past = time.Now().Add(-time.Hour * 20000)
var mkMock = MakeMockDigest
var CommonMockDigests = []MockDigest{
	mkMock("4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90", yesterday, "docker.io/library/ubuntu:24.04"),
	mkMock("5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb", past, "docker.io/hashicorp/vault:1.13.3"),
}

var CommonMockDigestFunc = MockDigestImage(CommonMockDigests)
