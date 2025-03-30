package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uebian/udp2tcp/config"
	"github.com/uebian/udp2tcp/service"
)

var (
	// Used for flags.
	cfgFile string
	cfg     *config.Config

	rootCmd = &cobra.Command{
		Use:   "udp2tcp",
		Short: "A lightweight tool to proxy udp traffic through various TCP based protocols",
		Run: func(cmd *cobra.Command, args []string) {
			// Get base config (possibly from config file)
			cfg := getConfigFromViper()

			// Override with command-line args if provided
			if udpListen, _ := cmd.Flags().GetString("udp-listen"); udpListen != "" {
				cfg.UDPListenAddr = udpListen
			}

			if udpTarget, _ := cmd.Flags().GetString("udp-target"); udpTarget != "" {
				cfg.UDPTargetAddr = udpTarget
			}

			if tcpURL, _ := cmd.Flags().GetString("tcp-url"); tcpURL != "" {
				cfg.TCPURL = tcpURL
			}

			if tcpListen, _ := cmd.Flags().GetString("tcp-listen"); tcpListen != "" {
				cfg.TCPListenAddr = tcpListen
			}

			if nMux, _ := cmd.Flags().GetInt("n-mux"); nMux > 0 {
				cfg.NMux = nMux
			}

			if timeout, _ := cmd.Flags().GetInt("timeout"); cfg.Timeout > 0 {
				cfg.Timeout = timeout
			}

			cfg.BufferSize, _ = cmd.Flags().GetInt("buffer-size")

			// Verify required configuration is present

			if cfg.TCPListenAddr == "" && cfg.TCPURL == "" {
				fmt.Println("Error: Required configuration missing. Please provide --tcp-listen or --tcp-url flags or use a config file.")
				os.Exit(1)
			}

			if cfg.TCPListenAddr != "" && cfg.TCPURL != "" {
				fmt.Println("Error: Duplicated configuration entries. Please provide either --tcp-listen or --tcp-url flags or use a config file.")
				os.Exit(1)
			}

			if cfg.MetricListenAddr != "" {
				go service.ListenAndServeMetrics(cfg)

			}

			if cfg.TCPListenAddr != "" {
				// Server mode

				// Verify required configuration is present
				if cfg.UDPTargetAddr == "" {
					fmt.Println("Error: Required configuration missing. Please provide --udp-target flags or use a config file.")
					os.Exit(1)
				}

				// Start server with config
				fmt.Printf("Starting server with config: %+v\n", cfg)
				server := service.NewServer(cfg)
				err := server.Init()
				if err != nil {
					log.Fatal("Failed to start Server:", err)
				}
				defer server.Close()

				go server.ListenAndServe()

				quit := make(chan os.Signal, 1)
				signal.Notify(quit, os.Interrupt)
				<-quit
				log.Println("Shutdown Server ...")

				if err = server.Close(); err != nil {
					log.Fatal("Server Shutdown:", err)
				}
				log.Println("Server exiting")

			} else {
				// Client mode
				if cfg.UDPListenAddr == "" {
					fmt.Println("Error: Required configuration missing. Please provide --udp-listen and --tcp-url flags or use a config file.")
					os.Exit(1)
				}

				// Start client with config
				fmt.Printf("Starting client with config: %+v\n", cfg)
				client := service.NewClient(cfg)
				err := client.Init()
				if err != nil {
					log.Fatal("Failed to start client:", err)
				}

				defer client.Close()

				go client.ListenAndServe()

				quit := make(chan os.Signal, 1)
				signal.Notify(quit, os.Interrupt)
				<-quit
				log.Println("Shutdown Client ...")

				if err := client.Close(); err != nil {
					log.Fatal("Client Shutdown:", err)
				}
				log.Println("Client exiting")
			}

		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Create config instance
	cfg = config.New()

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.udp2tcp.yaml)")
	rootCmd.Flags().String("udp-target", "", "UDP target address to forward traffic to (e.g., 127.0.0.1:5000)")
	rootCmd.Flags().String("udp-listen", "", "UDP address to listen on (optional)")
	rootCmd.Flags().String("tcp-listen", "", "TCP address to listen on (e.g., :8080)")
	rootCmd.Flags().String("tcp-url", "", "TCP server URL to connect to (e.g., tcp://example.com:8080)")
	rootCmd.Flags().Int("n-mux", 0, "Number of multiplexed connections to use")
	rootCmd.Flags().Int("buffer-size", 1024, "Size of buffer to use for reading UDP")
	rootCmd.Flags().Int("timeout", 5000, "Timeout in milliseconds for TCP connections")

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	// Initialize config structure
	if cfg == nil {
		cfg = config.New()
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".udp2tcp" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".") // Also look in current directory
		viper.SetConfigType("yaml")
		viper.SetConfigName(".udp2tcp")
	}

	viper.AutomaticEnv()

	// Try to read config file, but don't fail if it doesn't exist
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())

		// Unmarshal into our config structure
		if err := viper.UnmarshalKey("udp2tcp", cfg); err != nil {
			fmt.Printf("Error parsing config file: %v\n", err)
		}
	}
}

// getConfigFromViper loads config values from viper (which may come from
// config file or env vars) and command-line flags, with command-line flags
// taking precedence
func getConfigFromViper() *config.Config {
	// If we already loaded from a config file, use that as base
	if cfg == nil {
		cfg = config.New()
	}

	// Command-line flags should override config file values
	// This will be handled in the specific command implementations

	return cfg
}
