package helpers

import (
	"time"
)

var yesterday = time.Now().Add(-time.Hour * 24)
var past = time.Now().Add(-time.Hour * 20000)
var harbor = time.Now().Add(-time.Hour * 170000)
var mkMock = MakeMockDigest
var CommonMockDigests = []MockDigest{
	mkMock("4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90", yesterday, "docker.io/library/ubuntu:24.04"),
	mkMock("5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb", past, "docker.io/hashicorp/vault:1.13.3"),
	mkMock("b7a54e6b04ffe19096cc5a788fa3364bc2dea742c26a990ea3270bf20eaa723d", harbor, "docker.io/goharbor/nginx-photon:v2.11.1"),
	mkMock("e35d3cac38395f0c306d83ac9b587c73a93188040ec018f7e8fcfe015e507175", harbor, "docker.io/goharbor/harbor-portal:v2.11.1"),
	mkMock("c017dd84ee96df33f54677fce6a03e3dc8e00e2557447edc6015e42f34452236", harbor, "docker.io/goharbor/harbor-core:v2.11.1"),
	mkMock("30edb2ca02e57bf6cbcac36fdea373ba6551942637c9f404c127cc8c9446f837", harbor, "docker.io/goharbor/harbor-jobservice:v2.11.1"),
	mkMock("5645d459af2ba7200020a3473bc345ae87a2e2e189375b9a04cd6ec035c850fa", harbor, "docker.io/goharbor/registry-photon:v2.11.1"),
	mkMock("7bdbfc97215e25879ff5e6de25f1e343b7816251458813c03a77d6b692f1b88c", harbor, "docker.io/goharbor/harbor-registryctl:v2.11.1"),
	mkMock("b4bfaabbbaaca306c5e9c36dd438a5745bfc1af58e0810a74677f9f022348958", harbor, "docker.io/goharbor/trivy-adapter-photon:v2.11.1"),
	mkMock("6b40bd47abae66669c05d00b479bd41a2a016d9d50ef25adadfc929bc9d4fb38", harbor, "docker.io/goharbor/harbor-db:v2.11.1"),
	mkMock("76a158e5c3a2edac5d0d77311244111843d0bbfe543c12e691b49e8e3dba7f6c", harbor, "docker.io/goharbor/redis-photon:v2.11.1"),
	mkMock("22caf9ff7131a278674bff2ed2494d7a47ee49ee6a8c6ed859740000d0322bca", harbor, "docker.io/goharbor/harbor-exporter:v2.11.1"),
}

var CommonMockDigestFunc = MockDigestImage(CommonMockDigests)
