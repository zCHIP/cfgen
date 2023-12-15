package main

import (
	"cfgen"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	listenPort         = 8080
	envNamespace       = "WORKING_NAMESPACE"
	defaultNamespace   = "default"
	envConfOutPath     = "CONFS_OUTPUT_PATH"
	defaultConfOutPath = "/confsout"

	confFileNameMask         = "%s.conf"
	confDisabledFileExt      = ".disabled"
	confDisabledFileNameMask = confFileNameMask + ".%s" + confDisabledFileExt
)

// health used in HTTP response for application health-check
type health struct {
	status string
}

// cleanFileList removes dirs and disabled configurations from a given slice of os.FileInfo
func cleanFileList(files *[]os.FileInfo) []os.FileInfo {
	var newFiles []os.FileInfo

	for _, file := range *files {
		// if file is not a dir and not disabled, adds it to a new slice
		if !(file.IsDir() || (path.Ext(file.Name()) == confDisabledFileExt)) {
			newFiles = append(newFiles, file)
		}
	}

	return newFiles
}

// serviceInServices checks if a given service name in a given services slice
func serviceInServices(name string, list []v1.Service) bool {
	for _, svc := range list {
		if svc.Name == name {
			return true
		}
	}
	return false
}

// getWorkingNamespace returns current working pod's namespace or default one if envNamespace is not set
func getWorkingNamespace() (namespace string) {
	ns := os.Getenv(envNamespace)

	if ns != "" {
		return ns
	}

	// If environment variable is not set, returning default namespace
	log.Printf("WARN The %s env var is not set, falling back to \"%s\" namespace\n", envNamespace, defaultNamespace)
	return defaultNamespace
}

// getConfOutPath returns the configured path to store generated configs or default one if envConfOutPath is not set
func getConfOutPath() (confOutPath string) {
	p := os.Getenv(envConfOutPath)

	if p != "" {
		return p
	}

	// If environment variable is not set, returning default path
	log.Printf("WARN The %s env var is not set, falling back to \"%s\" path\n", envConfOutPath, defaultConfOutPath)
	return defaultConfOutPath
}

// createInClusterClient create and returns kubernetes.Clientset based on in-cluster config
func createInClusterClient() (cs *kubernetes.Clientset) {
	// Creates an in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("ERROR Unable to build the in cluster client config, error: %s", err)
	}

	// Creates the clientset
	cs, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("ERROR Unable to create the in cluster clientset, error: %s", err)
	}

	return cs
}

// serveHealthCheck responds with application status
func serveHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := health{"up"}

	js, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(js); err != nil {
		log.Printf("ERROR Unable to respond to HTTP request: \"%s\", error: %s\n", r.Body, err)
	}
}

// svcAdded runs every time as new service get added
func svcAdded(obj interface{}) {
	svc := obj.(*v1.Service)

	cfgFileName := fmt.Sprintf(confFileNameMask, svc.Name)
	cfgFilePath := path.Join(getConfOutPath(), cfgFileName)

	log.Printf("INFO Discovered a new service: %s\n", svc.Name)

	// Checking if the config already exists
	if _, err := os.Stat(cfgFilePath); os.IsExist(err) {
		log.Printf("WARN Config file named \"%s\" exists for new "+
			"service \"%s\" and is going to be re-written.\n", cfgFileName, svc.Name)
	}

	// Creating file-writer
	wr, err := os.Create(cfgFilePath)
	if err != nil {
		log.Printf("ERROR Unable to create file on %s, error: %s\n", cfgFilePath, err)
		return
	}

	// Generating config for the new service
	if err := cfgen.Generate(svc.Name, svc.Namespace, wr); err != nil {
		log.Printf("ERROR Unable to generate a config for service \"%s\", error: %s\n", svc.Name, err)
	} else {
		log.Printf("INFO Generated config for service \"%s\"\n", svc.Name)
	}
}

// svcDeleted runs every time as a service get deleted
func svcDeleted(obj interface{}) {
	svc := obj.(*v1.Service)

	cfgFileName := fmt.Sprintf(confFileNameMask, svc.Name)
	cfgFilePath := path.Join(getConfOutPath(), cfgFileName)

	log.Printf("INFO Service has deleted: %s\n", svc.Name)

	// Checking if the config exists
	if _, err := os.Stat(cfgFilePath); os.IsNotExist(err) {
		log.Printf("WARN Config file named \"%s\" does not exist for service \"%s\"", cfgFileName, svc.Name)
		return
	}

	// Deleting the config file
	if err := os.Remove(cfgFilePath); err != nil {
		log.Printf("ERROR Unable to remove the config file for service \"%s\", error: \"%s\"", svc.Name, err)
	} else {
		log.Printf("INFO Removed the config file for deleted service \"%s\"", svc.Name)
	}
}

// svcUpdated runs every time as a service get updates
func svcUpdated(oldObj, newObj interface{}) {
	oldSvc := oldObj.(*v1.Service)
	newSvc := newObj.(*v1.Service)

	log.Printf("INFO Service %s has changed to %s\n", oldSvc.ObjectMeta.Name, newSvc.ObjectMeta.Name)
}

// svcConfigsInit gets all services from the k8s API and generates configs for them. If a config already exists,
// but a service for it is not there, it will be renamed with the following pattern:
// "<existing_file>.<current_date>.disabled" where "current_date" is "yyyyMMddHHmmss"
func svcConfigsInit(client *kubernetes.Clientset) error {
	ctx := context.TODO()

	// Gets all available services in the working namespace
	services, err := client.CoreV1().Services(getWorkingNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("ERROR Unable to get services list for \"%s\" namespace, error: %s\n", getWorkingNamespace(), err)
		return err
	}

	// Get all config files in the pre-defined path
	outPath := getConfOutPath()
	log.Printf("[INFO] out path: %q\n", outPath)
	dirFiles, err := ioutil.ReadDir(outPath)
	if err != nil {
		log.Printf("ERROR Unable to read files in \"%s\", error: %s\n", getConfOutPath(), err)
		return err
	}

	// Cleans up the files list from dirs and disabled configs
	files := cleanFileList(&dirFiles)
	log.Printf("[INFO] Found next files: %q\n", files)
	// Checking existing config files
	for _, file := range files {
		// Gets name of the file without an extension (for ex: ".conf")
		fname := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

		// Checks if service for the file does not exists
		if !serviceInServices(fname, services.Items) {
			newName := fmt.Sprintf(confDisabledFileNameMask, file.Name(), time.Now().Format("20060102150405"))
			// Renaming file
			if err := os.Rename(
				path.Join(getConfOutPath(), file.Name()),
				path.Join(getConfOutPath(), newName)); err != nil {
				log.Printf("ERROR Unable to rename file from \"%s\" to \"%s\", error: %s\n", file.Name(), newName, err)
			}
		}
	}

	// Checking services
	for _, svc := range services.Items {
		cfgFilePath := path.Join(getConfOutPath(), fmt.Sprintf(confFileNameMask, svc.Name))

		if _, err := os.Stat(cfgFilePath); os.IsExist(err) {
			log.Printf("INFO Udating config for %s\n", svc.Name)
		}

		// Creating file-writer
		wr, err := os.Create(cfgFilePath)
		if err != nil {
			log.Printf("ERROR Unable to write in file at %s, error: %s\n", cfgFilePath, err)
		}

		// Generating config for a service
		if err := cfgen.Generate(svc.Name, getWorkingNamespace(), wr); err != nil {
			log.Printf("ERROR Unable to generate a config for service \"%s\", error: %s\n", svc.Name, err)
		} else {
			log.Printf("INFO Generated config for service \"%s\"\n", svc.Name)
		}
	}

	return nil
}

func main() {
	log.Println("[INFO] Starting cfgen service")
	// Gets a k8s client
	client := createInClusterClient()

	// Checks existing and generates new configs after the pod startup
	if err := svcConfigsInit(client); err != nil {
		os.Exit(1)
	}

	// ListWatch knows how to list and watch a set of apiserver resources
	watchlist := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		string(v1.ResourceServices),
		getWorkingNamespace(),
		fields.Everything(),
	)

	// Creates k8s events informer
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Service{}, // Indicates that going to get services
		0,             // resyncPeriod is int64 and if non-zero, will re-list this often
		cache.ResourceEventHandlerFuncs{
			AddFunc:    svcAdded,
			DeleteFunc: svcDeleted,
			UpdateFunc: svcUpdated,
		},
	)

	// Run a controller for the informer
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	// Adds handler func to serve health check
	http.HandleFunc("/", serveHealthCheck)

	// Listen to the port
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", listenPort), nil))
}
