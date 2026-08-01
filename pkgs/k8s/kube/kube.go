package kube

import (
	"io"
	"os"

	"github.com/go-logr/logr"
	"github.com/robdavid/img-pin/pkgs/k8s/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var KubeVersion = types.EnvOpt{Env: "IMG_PIN_KUBE_VERSION"}

func GetClusterVersion() (string, error) {
	if version, ok := KubeVersion.Opt().GetOK(); ok {
		return version, nil
	}
	prev := klog.Background()
	klog.SetLogger(logr.Discard())
	defer klog.SetLogger(prev)

	// Also suppress the direct klog output path
	klog.SetOutput(io.Discard)
	defer klog.SetOutput(os.Stderr)

	cfg, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, configOverrides,
		).ClientConfig()
	}
	if err != nil {
		return "", err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}

	info, err := client.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}

	return info.GitVersion, nil
}
