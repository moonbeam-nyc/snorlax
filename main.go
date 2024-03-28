package main

import (
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
	Short: "A service to that sleeps and wakes your Kubernetes deployments (by schedule and requests).",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the wake HTTP server",
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
	Short: "Watch the time, and wake or sleep the deployment when it's time",
	Run: func(cmd *cobra.Command, args []string) {
		// k8sClient := createK8sClient()
		awake := true

		// Start the watch loop
		for {
			now := time.Now()

			// Setup temp wake and sleep times with the same date as now
			wakeTime := time.Date(now.Year(), now.Month(), now.Day(), config.WakeTime.Hour(), config.WakeTime.Minute(), 0, 0, time.Local)
			sleepTime := time.Date(now.Year(), now.Month(), now.Day(), config.SleepTime.Hour(), config.SleepTime.Minute(), 0, 0, time.Local)

			var shouldSleep bool
			if wakeTime.Before(sleepTime) {
				shouldSleep = now.Before(wakeTime) || now.After(sleepTime)
			} else {
				shouldSleep = now.Before(sleepTime) || now.After(wakeTime)
			}

			fmt.Println()
			fmt.Println("now", now)
			fmt.Println("wake", wakeTime)
			fmt.Println("sleep", sleepTime)
			fmt.Println("awake", awake)
			fmt.Println("should sleep", shouldSleep)

			// Replace the date part of the WakeTime and SleepTime with the current date
			if awake && shouldSleep {
				fmt.Printf("\nGoing to sleep 💤\n")
				awake = false

				// Scale up the deployment
				// scale := fmt.Sprintf(`{"spec":{"replicas":%d}}`, config.ReplicaCount)
				// _, err := k8sClient.AppsV1().Deployments(config.Namespace).Patch(context.Background(), config.DeploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
				// if err != nil {
				// 	log.Printf("Failed to scale up deployment: %v", err)
				// }
			} else if !awake && !shouldSleep {
				fmt.Printf("\nWaking up ☀️\n")
				awake = true

				// Scale down the deployment
				// scale := `{"spec":{"replicas":0}}`
				// _, err := k8sClient.AppsV1().Deployments(config.Namespace).Patch(context.Background(), config.DeploymentName, types.StrategicMergePatchType, []byte(scale), metav1.PatchOptions{})
				// if err != nil {
				// 	log.Printf("Failed to scale down deployment: %v", err)
				// }
			}

			// Sleep for 10 seconds before checking again
			time.Sleep(10 * time.Second)
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

	if err := viper.Unmarshal(&config); err != nil {
		fmt.Printf("Error unmarshaling config: %s\n", err)
		return
	}

	// Load wake and sleep times
	sleepTimeStr := os.Getenv("SLEEP_TIME")
	wakeTimeStr := os.Getenv("WAKE_TIME")

	sleepTime, err := time.Parse("15:04", sleepTimeStr)
	if err != nil {
		log.Fatalf("Failed to parse sleep time: %v", err)
	}

	wakeTime, err := time.Parse("15:04", wakeTimeStr)
	if err != nil {
		log.Fatalf("Failed to parse wake time: %v", err)
	}

	config.SleepTime = sleepTime
	config.WakeTime = wakeTime

	// fmt.Printf("%+v\n", config)
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
