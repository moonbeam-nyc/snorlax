package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	cc "github.com/ivanpirog/coloredcobra"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var rootCmd = &cobra.Command{
	Use:   "snorlax",
	Short: "a service to that sleeps and wakes your containers",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		// Read environment variables
		deploymentName := os.Getenv("DEPLOYMENT_NAME")
		if deploymentName == "" {
			log.Fatal("DEPLOYMENT_NAME environment variable is not set.")
		}

		replicaCountStr := os.Getenv("REPLICA_COUNT")
		if replicaCountStr == "" {
			log.Fatal("REPLICA_COUNT environment variable is not set.")
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
			scale := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicaCount)
			_, err = clientset.AppsV1().Deployments("default").Patch(r.Context(), deploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
			if err != nil {
				log.Printf("Failed to scale deployment: %v", err)
				http.Error(w, "Failed to scale deployment", http.StatusInternalServerError)
				return
			}

			fmt.Fprintf(w, "Deployment scaled successfully to %d replicas", replicaCount)
		})

		// Start the web server
		log.Println("Starting server on port 8080...")
		log.Fatal(http.ListenAndServe(":8080", nil))
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Print 'hello world'",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hello, world!")
	},
}

func main() {
    cc.Init(&cc.Config{
        RootCmd:       rootCmd,
        Headings:      cc.HiCyan + cc.Bold + cc.Underline,
        Commands:      cc.HiYellow + cc.Bold,
        Example:       cc.Italic,
        ExecName:      cc.Bold,
        Flags:         cc.Bold,
    })

	rootCmd.AddCommand(serveCmd, watchCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}