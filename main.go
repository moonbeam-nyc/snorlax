package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	IngressName    string    `mapstructure:"INGRESS_NAME"`
	Port           int       `mapstructure:"PORT"`
}

func init() {
	viper.SetDefault("PORT", 8080)
}

var config Config
var awake bool

//go:embed static/*
var staticFiles embed.FS

var rootCmd = &cobra.Command{
	Use:   "snorlax",
	Short: "A service to that sleeps and wakes your Kubernetes deployments (by schedule and requests).",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var wakeCmd = &cobra.Command{
	Use:   "wake",
	Short: "Wake up the deployment",
	Run: func(cmd *cobra.Command, args []string) {
		wake()
	},
}

var sleepCmd = &cobra.Command{
	Use:   "sleep",
	Short: "Put the deployment to sleep",
	Run: func(cmd *cobra.Command, args []string) {
		sleep()
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the time, and wake or sleep the deployment when it's time",
	Run: func(cmd *cobra.Command, args []string) {
		watch()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the wake HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		serve()
	},
}

var watchServeCmd = &cobra.Command{
	Use:   "watch-serve",
	Short: "Watch the time and serve the wake HTTP server",
	Run: func(cmd *cobra.Command, args []string) {
		go watch()
		serve()
	},
}

func watch() {
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

		// fmt.Println()
		// fmt.Println("now", now)
		// fmt.Println("wake", wakeTime)
		// fmt.Println("sleep", sleepTime)
		// fmt.Println("awake", awake)
		// fmt.Println("should sleep", shouldSleep)

		// Replace the date part of the WakeTime and SleepTime with the current date
		if awake && shouldSleep {
			fmt.Printf("\nGoing to sleep 💤\n")
			go sleep()
		} else if !awake && !shouldSleep {
			fmt.Printf("\nWaking up ☀️\n")
			go wake()
		}

		// Sleep for 10 seconds before checking again
		time.Sleep(10 * time.Second)
	}
}

func serve() {
	// var k8sClient = createK8sClient()

	// Create a new sub-filesystem from the `static` directory within the embedded filesystem
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	// Define the HTTP handler function
	fileServer := http.FileServer(http.FS(subFS))

	// http.Handle("/waking-up/", http.StripPrefix("/waking-up/", fileServer))

	http.HandleFunc("/still-sleeping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		go wake()
		fileServer.ServeHTTP(w, r)
		fmt.Fprintf(w, "Deployment scaled successfully to %d replicas", config.ReplicaCount)
	})

	// Start the web server
	log.Println("Starting server on http://localhost:", config.Port, "...")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil))
}

func scaleDeployment(replicaCount int32) {
	k8sClient := createK8sClient()
	scale, err := k8sClient.AppsV1().Deployments(config.Namespace).GetScale(context.TODO(), config.DeploymentName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	// Set the replica count to 0
	scale.Spec.Replicas = replicaCount

	// Update the scale
	_, err = k8sClient.AppsV1().Deployments(config.Namespace).UpdateScale(context.TODO(), config.DeploymentName, scale, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func wake() {
	scaleDeployment(int32(config.ReplicaCount))
	loadIngressCopy()
	awake = true
}

func sleep() {
	takeIngressCopy()
	pointIngressToSnorlax()
	scaleDeployment(0)
	awake = false
}

func pointIngressToSnorlax() {
	k8sClient := createK8sClient()

	// Get the Ingress object
	ingress, err := k8sClient.NetworkingV1().Ingresses(config.Namespace).Get(context.TODO(), config.IngressName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	// Update the Ingress object
	pathType := networkingv1.PathTypeImplementationSpecific
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "snorlax-nginx",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	}

	// Update the Ingress
	_, err = k8sClient.NetworkingV1().Ingresses(config.Namespace).Update(context.TODO(), ingress, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func takeIngressCopy() {
	k8sClient := createK8sClient()

	// Get the Ingress object
	ingress, err := k8sClient.NetworkingV1().Ingresses(config.Namespace).Get(context.TODO(), config.IngressName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	// Convert the Ingress object to YAML
	ingressYAML, err := yaml.Marshal(ingress)
	if err != nil {
		panic(err.Error())
	}

	// Create the ConfigMap object
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snorlax.ingress." + config.IngressName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			"ingressYAML": string(ingressYAML),
		},
	}

	// Create the ConfigMap
	_, err = k8sClient.CoreV1().ConfigMaps(config.Namespace).Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	if err != nil {
		_, err = k8sClient.CoreV1().ConfigMaps(config.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}
	} else {
		_, err = k8sClient.CoreV1().ConfigMaps(config.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			panic(err.Error())
		}
	}
}

func loadIngressCopy() {
	k8sClient := createK8sClient()

	// Get the ConfigMap object
	configMap, err := k8sClient.CoreV1().ConfigMaps(config.Namespace).Get(context.TODO(), "snorlax.ingress."+config.IngressName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	// Unmarshal the YAML from the ConfigMap into an Ingress object
	var ingress networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(configMap.Data["ingressYAML"]), &ingress); err != nil {
		panic(err.Error())
	}

	// Marshal the IngressSpec into JSON
	ingressSpec, err := json.Marshal(ingress.Spec)
	if err != nil {
		panic(err.Error())
	}

	// Wrap the json in a 'spec' key
	ingressSpec = []byte(fmt.Sprintf(`{"spec": %s}`, string(ingressSpec)))

	// Patch the existing Ingress with the new spec
	_, err = k8sClient.NetworkingV1().Ingresses(config.Namespace).Patch(context.TODO(), config.IngressName, types.MergePatchType, ingressSpec, metav1.PatchOptions{})
	if err != nil {
		panic(err.Error())
	}
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
	viper.BindEnv("INGRESS_NAME")

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

	fmt.Printf("%+v\n", config)
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

	rootCmd.AddCommand(serveCmd, watchCmd, watchServeCmd, wakeCmd, sleepCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	loadConfig()
	runCli()
}
