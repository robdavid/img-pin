package kube

import (
	"io"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func GetClusterVersion() (string, error) {
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
