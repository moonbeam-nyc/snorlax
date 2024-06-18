package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Proxy struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

var (
	activityThreshold    = 1 * time.Minute
	clientset            *kubernetes.Clientset
	config               *rest.Config
	configmap            string
	deploymentName       string
	destinationHost      string
	destinationPort      string
	healthCheckUserAgent = "ELB-HealthChecker/2.0"
	kubeconfig           string
	namespace            string
	port                 string
)

func init() {
	var err error

	configmap = os.Getenv("DATA_CONFIGMAP_NAME")
	deploymentName = os.Getenv("DEPLOYMENT_NAME")
	destinationHost = os.Getenv("DESTINATION_HOST")
	destinationPort = os.Getenv("DESTINATION_PORT")
	kubeconfig = os.Getenv("KUBECONFIG")
	namespace = os.Getenv("NAMESPACE")
	port = os.Getenv("PORT")

	// Check for missing environment variables
	var missingVars []string
	variables := []string{"NAMESPACE", "DATA_CONFIGMAP_NAME", "PORT", "DESTINATION_HOST", "DESTINATION_PORT", "DEPLOYMENT_NAME"}
	for _, variable := range variables {
		if os.Getenv(variable) == "" {
			missingVars = append(missingVars, variable)
		}
	}

	// Exit if there are missing environment variables
	if len(missingVars) > 0 {
		log.Fatalf("missing environment variables: %s", strings.Join(missingVars, ", "))
	}

	// Initialize the right Kubernetes config
	if kubeconfig != "" {
		log.Default().Print("Using local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("failed to create config from KUBECONFIG: %v", err)
		}
	} else {
		log.Default().Print("Using in-cluster config")
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("failed to create in-cluster config: %v", err)
		}
	}

	// Create the k8s client
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("error creating Kubernetes client: %v", err)
	}
}

func NewProxy(target string) *Proxy {
	url, _ := url.Parse(target)
	return &Proxy{target: url, proxy: httputil.NewSingleHostReverseProxy(url)}
}

func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request) {
	// If the request is not a health check, update the configmap
	userAgent := r.Header.Get("User-Agent")
	if userAgent != healthCheckUserAgent {
		activityDetected()
	}

	r.URL.Host = p.target.Host
	r.URL.Scheme = p.target.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = p.target.Host
	p.proxy.ServeHTTP(w, r)
}

func patchConfigMap(name string, data map[string]string) error {
	patchData := make(map[string]interface{})
	patchData["data"] = data

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("error marshaling patch data: %v", err)
	}

	_, err = clientset.CoreV1().ConfigMaps(namespace).Patch(context.Background(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error patching configmap: %v", err)
	}

	return nil
}

func activityDetected() {
	log.Printf("activity detected for %s", deploymentName)

	now := time.Now().Format(time.RFC3339)
	data := map[string]string{
		fmt.Sprintf("deployment.%s.last-active-at", deploymentName): now,
	}

	patchConfigMap(configmap, data)
}

func inactivityDetected() {
	log.Printf("inactivity detected for %s", deploymentName)

	now := time.Now().Format(time.RFC3339)
	data := map[string]string{
		fmt.Sprintf("deployment.%s.last-inactive-at", deploymentName): now,
	}

	patchConfigMap(configmap, data)
}

func getLastActivity() time.Time {
	// Get the configmap
	configmap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), os.Getenv("DATA_CONFIGMAP_NAME"), metav1.GetOptions{})
	if err != nil {
		// log.Printf("error getting configmap: %v", err)
		return time.Time{}
	}

	lastActivity, err := time.Parse(time.RFC3339, configmap.Data[fmt.Sprintf("deployment.%s.last-active-at", deploymentName)])
	if err != nil {
		// log.Printf("error parsing last active time: %v", err)
		return time.Time{}
	}

	return lastActivity
}

func detectInactivity() {
	go func() {
		for {
			time.Sleep(time.Second * 5)

			lastActivity := getLastActivity()
			log.Printf("checking last activity: %v", lastActivity)

			// Check if it's been inactive
			if !lastActivity.IsZero() && time.Since(lastActivity) > activityThreshold {
				inactivityDetected()
			}
		}
	}()
}

func main() {
	detectInactivity()

	target := "http://" + destinationHost + ":" + destinationPort
	proxy := NewProxy(target)

	http.HandleFunc("/", proxy.Handle)
	log.Println("Starting proxy server on :" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
