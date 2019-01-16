package main

import (
    "fmt"
    "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/fields"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/cache"
    "log"
    "time"
)

var (
    namespace = "default"
)

func main() {
    // Creates the in-cluster config
    config, err := rest.InClusterConfig()
    if err != nil {
        log.Fatalf("ERROR Unable to build the in cluster client config, error: %s", err)
    }

    // Creates the clientset
    clientSet, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Fatalf("ERROR Unable to build the in cluster client config, error: %s", err)
    }

    services, err := clientSet.CoreV1().Services(namespace).List(metav1.ListOptions{LabelSelector: "", FieldSelector: ""})
    if err != nil {
        log.Fatalf("ERROR Unable to get the list of services, error: %s\n", err)
    }

    log.Printf("Services count: %d", len(services.Items))

    // ListWatch knows how to list and watch a set of apiserver resources
    watchlist := cache.NewListWatchFromClient(
        clientSet.CoreV1().RESTClient(),
        string(v1.ResourceServices),
        v1.NamespaceAll,  // TODO pass namespace here from env or args
        fields.Everything(),
    )

    // Creates k8s events informer
    _, controller := cache.NewInformer(
        watchlist,
        &v1.Service{}, // Going to get services
        0, //Duration is int64
        cache.ResourceEventHandlerFuncs{
            // TODO pull to a separate Add-function
            AddFunc: func(obj interface{}) {
                fmt.Printf("Service has added: %s \n", obj)
            },
            // TODO pull to a separate Delete-function
            DeleteFunc: func(obj interface{}) {
                fmt.Printf("Service has deleted: %s \n", obj)
            },
            // TODO pull to a separate Update-function
            UpdateFunc: func(oldObj, newObj interface{}) {
                fmt.Printf("Service has changed \n")
            },
        },
    )

    stop := make(chan struct{})
    defer close(stop)
    go controller.Run(stop)
    for {
        time.Sleep(time.Second)
    }

    // TODO introduce http-based health check which will help to keep app running
    //Keep alive
    //log.Fatal(http.ListenAndServe(":8080", nil))
}