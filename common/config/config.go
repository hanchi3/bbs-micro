// common/config/config.go
package config

import (
	"os"

	"github.com/spf13/viper"
)

var Conf *Config

type Config struct {
	Server   *Server             `yaml:"server"`
	MySQL    *MySQL              `yaml:"mysql"`
	Redis    *Redis              `yaml:"redis"`
	Etcd     *Etcd               `yaml:"etcd"`
	Kafka    *Kafka              `yaml:"kafka"`
	Services map[string]*Service `yaml:"services"`
}

type Server struct {
	Port      string `yaml:"port"`
	Version   string `yaml:"version"`
	JwtSecret string `yaml:"jwtSecret"`
}

type MySQL struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Database     string `yaml:"database"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Charset      string `yaml:"charset"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
}

type Redis struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	DB           int    `yaml:"db"`
	Password     string `yaml:"password"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type Etcd struct {
	Address string `yaml:"address"`
}

type Kafka struct {
	Brokers string `yaml:"brokers"`
	Topic   string `yaml:"topic"`
}

type Service struct {
	Name        string   `yaml:"name"`
	LoadBalance bool     `yaml:"loadBalance"`
	Addr        []string `yaml:"addr"`
}

func InitConfig() {
	workDir, _ := os.Getwd()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 获取项目根目录
	rootDir := workDir
	for {
		if _, err := os.Stat(rootDir + "/common/config/config.yaml"); err == nil {
			break
		}
		parent := rootDir + "/.."
		if parent == rootDir {
			panic("无法找到配置文件")
		}
		rootDir = parent
	}

	// 添加配置文件路径
	viper.AddConfigPath(rootDir + "/common/config")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	err = viper.Unmarshal(&Conf)
	if err != nil {
		panic(err)
	}
}
