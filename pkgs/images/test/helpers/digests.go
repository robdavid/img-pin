package helpers

import (
	"time"

	"github.com/robdavid/img-pin/pkgs/images"
)

var unset time.Time
var yesterday = time.Now().Add(-time.Hour * 24)
var past = time.Now().Add(-time.Hour * 20000)
var crusty = time.Now().Add(-time.Hour * 40000)
var mkMock = MakeMockDigest
var errMock = MakeMockDigestErr
var CommonMockDigests = []MockDigest{
	mkMock("4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90", yesterday, "docker.io/library/ubuntu:24.04"),
	mkMock("5eba321fbeb624163a45c1aee5379caf6ec16fe6f644cc89f203a209eafba5eb", past, "docker.io/hashicorp/vault:1.13.3"),
	mkMock("b7a54e6b04ffe19096cc5a788fa3364bc2dea742c26a990ea3270bf20eaa723d", past, "docker.io/goharbor/nginx-photon:v2.11.1"),
	mkMock("e35d3cac38395f0c306d83ac9b587c73a93188040ec018f7e8fcfe015e507175", past, "docker.io/goharbor/harbor-portal:v2.11.1"),
	mkMock("c017dd84ee96df33f54677fce6a03e3dc8e00e2557447edc6015e42f34452236", past, "docker.io/goharbor/harbor-core:v2.11.1"),
	mkMock("30edb2ca02e57bf6cbcac36fdea373ba6551942637c9f404c127cc8c9446f837", past, "docker.io/goharbor/harbor-jobservice:v2.11.1"),
	mkMock("5645d459af2ba7200020a3473bc345ae87a2e2e189375b9a04cd6ec035c850fa", past, "docker.io/goharbor/registry-photon:v2.11.1"),
	mkMock("7bdbfc97215e25879ff5e6de25f1e343b7816251458813c03a77d6b692f1b88c", past, "docker.io/goharbor/harbor-registryctl:v2.11.1"),
	mkMock("b4bfaabbbaaca306c5e9c36dd438a5745bfc1af58e0810a74677f9f022348958", past, "docker.io/goharbor/trivy-adapter-photon:v2.11.1"),
	mkMock("6b40bd47abae66669c05d00b479bd41a2a016d9d50ef25adadfc929bc9d4fb38", past, "docker.io/goharbor/harbor-db:v2.11.1"),
	mkMock("76a158e5c3a2edac5d0d77311244111843d0bbfe543c12e691b49e8e3dba7f6c", past, "docker.io/goharbor/redis-photon:v2.11.1"),
	mkMock("22caf9ff7131a278674bff2ed2494d7a47ee49ee6a8c6ed859740000d0322bca", past, "docker.io/goharbor/harbor-exporter:v2.11.1"),
	mkMock("4554f7ce1cbf453834e79838459f4f7d7036bb70cedad3bd5427eed21c585d3d", past, "ghcr.io/project-akri/akri/agent:v0.13.8"),
	mkMock("d7a84771026fa49d94f7fcd62472e9ff7df97175a6a11669ce65c796a37fbe84", past, "ghcr.io/project-akri/akri/udev-discovery:v0.13.8"),
	mkMock("003d1ab2f83eaf3969e0f689d523fc6d10d227dfbdd71b782a90ec729a72b615", past, "ghcr.io/project-akri/akri/controller:v0.13.8"),
	mkMock("6df09c63839ac3f0626ca4f8ad6cce9e4fbd2eb444e9392eff6c27eae86c1ec9", past, "ghcr.io/project-akri/akri/webhook-configuration:v0.13.8"),
	mkMock("95de17e6eb92da83a58c90a9df0c4cede634f898d2e6b92ea04f4a6ee6ace08d", yesterday, "docker.io/bitnami/kubectl:latest"),
	mkMock("64d8c73dca984af206adf9d6d7e46aa550362b1d7a01f3a0a91b20cc67868660", crusty, "registry.k8s.io/ingress-nginx/kube-webhook-certgen:v1.1.1"),
	mkMock("c64a643dd665db62c43aa089432eb2e74b13364c616fc12ca524baead6ccc332", yesterday, "docker.io/openpolicyagent/gatekeeper:dev"),
	errMock(images.ErrSchemaV1, "quay.io/dexidp/dex:v2.14.0"),
}

var CommonMockDigestFunc = MockDigestImage(CommonMockDigests)
