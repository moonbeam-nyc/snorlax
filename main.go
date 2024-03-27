package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	DeploymentName string    `mapstructure:"DEPLOYMENT_NAME"`
	Kubeconfig     string    `mapstructure:"KUBECONFIG"`
	Namespace      string    `mapstructure:"NAMESPACE"`
	ReplicaCount   int       `mapstructure:"REPLICA_COUNT"`
	SleepTime      time.Time `mapstructure:"SLEEP_TIME"`
	WakeTime       time.Time `mapstructure:"WAKE_TIME"`
}

var config Config

var rootCmd = &cobra.Command{
	Use:   "snorlax",
	Short: "a service to that sleeps and wakes your containers",
	Run: func(cmd *cobra.Command, args []string) {
		// cmd.Help()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		var k8sClient = createK8sClient()

		// Define the HTTP handler function
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Scale the deployment
			scale := fmt.Sprintf(`{"spec":{"replicas":%d}}`, config.ReplicaCount)
			_, err := k8sClient.AppsV1().Deployments(config.Namespace).Patch(r.Context(), config.DeploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
			if err != nil {
				log.Printf("Failed to scale deployment: %v", err)
				http.Error(w, "Failed to scale deployment", http.StatusInternalServerError)
				return
			}

			fmt.Fprintf(w, "Deployment scaled successfully to %d replicas", config.ReplicaCount)
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
		var k8sClient = createK8sClient()

		// Start the watch loop
		for {
			now := time.Now()

			if now.After(config.WakeTime) && now.Before(config.SleepTime) {
				// Scale up the deployment
				scale := fmt.Sprintf(`{"spec":{"replicas":%d}}`, config.ReplicaCount)
				_, err := k8sClient.AppsV1().Deployments(config.Namespace).Patch(context.Background(), config.DeploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
				if err != nil {
					log.Printf("Failed to scale up deployment: %v", err)
				}
			} else {
				// Scale down the deployment
				scale := `{"spec":{"replicas":0}}`
				_, err := k8sClient.AppsV1().Deployments(config.Namespace).Patch(context.Background(), config.DeploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
				if err != nil {
					log.Printf("Failed to scale down deployment: %v", err)
				}
			}

			// Sleep for 1 minute before checking again
			time.Sleep(1 * time.Minute)
		}
	},
}

func createK8sClient() *kubernetes.Clientset {
	// Setup config
	var k8sConfig *rest.Config
	var err error

	if kubeconfig := config.Kubeconfig; kubeconfig != "" {
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Failed to create config from KUBECONFIG: %v", err)
		}
	} else {
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Failed to create in-cluster config: %v", err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	return clientset
}

func loadConfig() {
	// Bind environment variables to config struct
	viper.AutomaticEnv()
	viper.BindEnv("DEPLOYMENT_NAME")
	viper.BindEnv("KUBECONFIG")
	viper.BindEnv("NAMESPACE")
	viper.BindEnv("REPLICA_COUNT")
	viper.BindEnv("SLEEP_TIME")
	viper.BindEnv("WAKE_TIME")
	if err := viper.Unmarshal(&config); err != nil {
		fmt.Printf("Error unmarshaling config: %s\n", err)
		return
	}

	// fmt.Printf("%+v\n", cliConfig)
}

func runCli() {
	cc.Init(&cc.Config{
		RootCmd:  rootCmd,
		Headings: cc.HiCyan + cc.Bold + cc.Underline,
		Commands: cc.HiYellow + cc.Bold,
		Example:  cc.Italic,
		ExecName: cc.Bold,
		Flags:    cc.Bold,
	})

	rootCmd.AddCommand(serveCmd, watchCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	loadConfig()
	runCli()
}
