package main

import (
    "cfgen"
    "flag"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/homedir"
    "log"
    "os"
    "path/filepath"
)

var (
	namespace      string
	cfgOutputPath  string
	kubeConfigPath string
)

func init() {
	// Command-line arguments
	flag.StringVar(&namespace, "namespace", "default",
	    "Sets the k8s namespace to work with. If not set, uses default namespace.")
	flag.StringVar(&cfgOutputPath, "output-path", "",
		"Sets path to save the generated configs. If not set, prints generated configs to stdout.")
	flag.StringVar(&kubeConfigPath, "kube-config", filepath.Join(homedir.HomeDir(), ".kube", "config"),
		"(Optional) Sets absolute path to kubeconfig to use. If not set, takes from the home dir.")
}

func main() {
    // Parses the command line arguments
	flag.Parse()

	// Checks if kubeconfig exists
	if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
		log.Fatalf("ERROR The kubeconfig does not exist: %s\n", kubeConfigPath)
	}

	// If output path is specified
	if cfgOutputPath != "" {
        // Checking if the specified output path exists
        fileInfo, err := os.Stat(cfgOutputPath)
        if os.IsNotExist(err) {
            log.Fatalf("ERROR The specified output path does not exist: %s\n", cfgOutputPath)
        }
        // Checking if specified output path is directory
        if !fileInfo.IsDir() {
            log.Fatalf("ERROR The specified output path is not a directory: %s\n", cfgOutputPath)
        }
    }

	log.Printf("INFO Using kubeconfig:    %s\n", kubeConfigPath)
	log.Printf("INFO Using k8s namespace: %s\n", namespace)
	log.Printf("INFO Configs output path: %s\n", cfgOutputPath)

	// Builds the clientConfig from file
	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.Fatalf("ERROR Unable to build the client config, error: %s\n", err)
	}

	// Creates the clientSet
	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		log.Fatalf("ERROR Unable to create k8s client from the config, error: %s\n", err)
	}

	// Gets all services in the given k8s namespace
	// TODO Implement additional filtering using ListOptions
	services, err := clientSet.CoreV1().Services(namespace).List(metav1.ListOptions{LabelSelector: "", FieldSelector: ""})
	if err != nil {
		log.Fatalf("ERROR Unable to get the list of services, error: %s\n", err)
	}

	servicesCount := len(services.Items)

	log.Printf("INFO Found %d services in the namespace: %s\n", servicesCount, namespace)
	for i, service := range services.Items {
		log.Printf("INFO Generating config for service %d of %d | Name: %s, Type: %s\n",
			i+1, servicesCount, service.Name, service.Spec.Type)

        if cfgOutputPath != "" {
            // TODO Check if the file exists
            // Writes to file
            wr, err := os.Create(filepath.Join(cfgOutputPath, service.Name))
            if err != nil {
                log.Fatalf("ERROR Can't create the file, error: %s\n", err)
            }

            if err := cfgen.Generate(service.Name, namespace, wr); err != nil {
                log.Fatalf("ERROR Can't generate the config for %s, error: %s\n", service.Name, err)
            }
        } else {
            // Writes to stdout
            if err := cfgen.Generate(service.Name, namespace, os.Stdout); err != nil {
                log.Fatalf("ERROR Can't generate the config for %s, error: %s\n", service.Name, err)
            }
        }

	}
}