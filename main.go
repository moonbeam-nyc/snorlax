package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	Kubeconfig string `mapstructure:"KUBECONFIG"`
	Port       int    `mapstructure:"PORT"`
}

func init() {
	viper.SetDefault("PORT", 8080)
}

var config Config

//go:embed static/*
var staticFiles embed.FS

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
		serve()
	},
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

	http.HandleFunc("/still-sleeping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Set state in configmap
		fileServer.ServeHTTP(w, r)
	})

	// Start the web server
	log.Printf("Starting server on http://localhost:%d...\n", config.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil))
}

// func createK8sClient() *kubernetes.Clientset {
// 	// Setup config
// 	var k8sConfig *rest.Config
// 	var err error

// 	if kubeconfig := config.Kubeconfig; kubeconfig != "" {
// 		k8sConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
// 		if err != nil {
// 			log.Fatalf("Failed to create config from KUBECONFIG: %v", err)
// 		}
// 	} else {
// 		k8sConfig, err = rest.InClusterConfig()
// 		if err != nil {
// 			log.Fatalf("Failed to create in-cluster config: %v", err)
// 		}
// 	}

// 	// Create the clientset
// 	clientset, err := kubernetes.NewForConfig(k8sConfig)
// 	if err != nil {
// 		log.Fatalf("Failed to create clientset: %v", err)
// 	}

// 	return clientset
// }

func loadConfig() {
	// Bind environment variables to config struct
	viper.AutomaticEnv()
	viper.BindEnv("KUBECONFIG")
	viper.BindEnv("PORT")
	viper.Unmarshal(&config)

	//fmt.Printf("%+v\n", config)
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

	rootCmd.AddCommand(serveCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	loadConfig()
	runCli()
}
