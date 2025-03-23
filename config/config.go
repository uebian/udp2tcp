package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	UDPListenAddr    string `mapstructure:"udp_listen_addr"`
	UDPTargetAddr    string `mapstructure:"udp_target_addr"`
	TCPURL           string `mapstructure:"tcp_url"`
	NMux             int    `mapstructure:"n_mux"`
	BufferSize       int    `mapstructure:"buffer_size"`
	TCPListenAddr    string `mapstructure:"tcp_listen_addr"`
	MetricListenAddr string `mapstructure:"metric_listen_addr"`
}

func New() *Config {
	return &Config{}
}

func (cfg *Config) Load() {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic("failed to read config file, error: " + err.Error())
	}
	if err := viper.UnmarshalKey("udp2tcp", cfg); err != nil {
		panic("failed to init api config, error: " + err.Error())
	}
}
