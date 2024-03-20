package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Read environment variables
	deploymentName := os.Getenv("DEPLOYMENT_NAME")
	if deploymentName == "" {
		log.Fatal("DEPLOYMENT_NAME environment variable is not set.")
	}

	replicaCountStr := os.Getenv("REPLICA_COUNT")
	if replicaCountStr == "" {
		log.Fatal("REPLICA_COUNT environment variable is not set.")
	}

	namespace := metav1.NamespaceDefault
	if ns := os.Getenv("NAMESPACE"); ns != "" {
		namespace = ns
	}

	replicaCount, err := strconv.Atoi(replicaCountStr)
	if err != nil {
		log.Fatalf("Failed to convert REPLICA_COUNT to integer: %v", err)
	}

    // Setup config
    var config *rest.Config
    if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
        if err != nil {
            log.Fatalf("Failed to create config from KUBECONFIG: %v", err)
        }
    } else {
        config, err = rest.InClusterConfig()
        if err != nil {
            log.Fatalf("Failed to create in-cluster config: %v", err)
        }
    }

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	// Define the HTTP handler function
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Scale the deployment
		// scale := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicaCount)
		// _, err = clientset.AppsV1().Deployments("default").Patch(r.Context(), deploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
		// if err != nil {
		// 	log.Printf("Failed to scale deployment: %v", err)
		// 	http.Error(w, "Failed to scale deployment", http.StatusInternalServerError)
		// 	return
		// }

		log.Printf("Scaling %s/%s to to %d replicas", namespace, deploymentName, replicaCount)

		// Get the deployment
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(r.Context(), deploymentName, metav1.GetOptions{})
		if err != nil {
			log.Printf("Failed to get deployment: %v", err)
			http.Error(w, "Failed to get deployment", http.StatusInternalServerError)
			return
		}

		// Print the deployment JSON with indents
		indentedJSON, err := json.MarshalIndent(deployment, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal deployment to indented JSON: %v", err)
			http.Error(w, "Failed to marshal deployment to indented JSON", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%s", indentedJSON)
	})


	// Start the web server
	log.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
