package main

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"ktwins/internal/ui"
)

func main() {
	rest.SetDefaultWarningHandler(rest.NoWarnings{})

	ns := ""
	if len(os.Args) > 1 {
		ns = os.Args[1]
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	dash := ui.NewDashboard(ns, clientset)
	if err := dash.Run(); err != nil {
		panic(err)
	}
}
