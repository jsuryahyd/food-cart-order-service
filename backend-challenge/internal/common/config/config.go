package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment     string                `mapstructure:"environment"`
	Server          ServerConfig          `mapstructure:"server"`
	Database        DatabaseConfig        `mapstructure:"database"`
	TestDB          TestDBConfig          `mapstructure:"testdb"`
	Redis           RedisConfig           `mapstructure:"redis"`
	ApiKeys         ApiKeysConfig         `mapstructure:"api_keys"`
	CouponProcessor CouponProcessorConfig `mapstructure:"coupon_preprocessor"`
	Misc            MiscConfig            `mapstructure:"misc"`
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int16         `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DbName          string        `mapstructure:"db_name"`
	Sslmode         string        `mapstructure:"sslmode"`
	MaxConns        int8          `mapstructure:"max_conns"`
	MinConns        int8          `mapstructure:"min_conns"`
	ConnMaxLifeTime time.Duration `mapstructure:"conn_max_lifetime"`
}

type TestDBConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int16         `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DbName          string        `mapstructure:"db_name"`
	Sslmode         string        `mapstructure:"sslmode"`
	MaxConns        int8          `mapstructure:"max_conns"`
	MinConns        int8          `mapstructure:"min_conns"`
	ConnMaxLifeTime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host                 string `mapstructure:"host"`
	Port                 int16  `mapstructure:"port"`
	Password             string `mapstructure:"password"`
	DB                   int    `mapstructure:"db"`
	CouponUpdateChannel  string `mapstructure:"coupon_update_channel"`
	WorkerTriggerChannel string `mapstructure:"worker_trigger_channel"`
}

type ApiKeysConfig struct {
	AdminKey string `mapstructure:"admin_api_key"`
}

type CouponProcessorConfig struct {
	COUPON_FILE1_URL              string
	COUPON_FILE2_URL              string
	COUPON_FILE3_URL              string
	OUTPUT_FILE_PATH              string
	OUTPUT_INDEX_FILE_PATH        string `mapstructure:"OUTPUT_INDEX_FILE_PATH"`
	ProcessedCouponsFilePath      string `mapstructure:"ProcessedCouponsFilePath"`
	ProcessedCouponsIndexFilePath string `mapstructure:"ProcessedCouponsIndexFilePath"`
	NumHotCouponsInCache          int    `mapstructure:"NumHotCouponsInCache"`
}

type MiscConfig struct {
	ShouldSeedData bool `mapstructure:"should_seed_Data"`
}

func LoadConfig(configFilePath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configFilePath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvPrefix("APP")

	// env vars (set from docker-compose) not .env file
	if err := v.BindEnv("database.host", "DB_HOST"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind DB_HOST env var: %v\n", err)
	}
	if err := v.BindEnv("database.db_name", "DB_NAME"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind DB_NAME env var: %v\n", err)
	}
	if err := v.BindEnv("database.password", "DB_PASSWORD"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind DB_PASSWORD env var: %v\n", err)
	}

	if err := v.BindEnv("redis.host", "REDIS_HOST"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind REDIS_HOST env var: %v\n", err)
	}

	if err := v.BindEnv("redis.password", "REDIS_PASSWORD"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind REDIS_PASSWORD env var: %v\n", err)
	}

	if err := v.BindEnv("api_keys.admin_api_key", "ADMIN_API_KEY"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind ADMIN_API_KEY env var: %v\n", err)
	}
	if err := v.BindEnv("misc.should_seed_data", "SHOULD_SEED_DATA"); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Failed to bind Should Seed data env var: %v\n", err)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found %s", configFilePath)
		} else {
			return nil, fmt.Errorf("error loading config %w", err)
		}
	}
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config %v %w", v, err)
	}

	//todo: refactor to avoid overriding env vars
	if os.Getenv("REDIS_HOST") != "" {
		config.Redis.Host = os.Getenv("REDIS_HOST")
	}
	if os.Getenv("REDIS_PORT") != "" {
		port, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
		if err == nil {
			config.Redis.Port = int16(port)
		}
	}
	if os.Getenv("REDIS_COUPON_UPDATE_CHANNEL") != "" {
		config.Redis.CouponUpdateChannel = os.Getenv("REDIS_COUPON_UPDATE_CHANNEL")
	}
	if os.Getenv("REDIS_WORKER_TRIGGER_CHANNEL") != "" {
		config.Redis.CouponUpdateChannel = os.Getenv("REDIS_WORKER_TRIGGER_CHANNEL")
	}
	if os.Getenv("PROCESSED_COUPONS_FILE_PATH") != "" {
		config.CouponProcessor.ProcessedCouponsFilePath = os.Getenv("PROCESSED_COUPONS_FILE_PATH")
	}
	if os.Getenv("PROCESSED_COUPONS_INDEX_FILE_PATH") != "" {
		config.CouponProcessor.ProcessedCouponsIndexFilePath = os.Getenv("PROCESSED_COUPONS_INDEX_FILE_PATH")
	}
	if os.Getenv("COUPON_FILE1_URL") != "" {
		config.CouponProcessor.ProcessedCouponsIndexFilePath = os.Getenv("COUPON_FILE1_URL")
	}
	if os.Getenv("COUPON_FILE2_URL") != "" {
		config.CouponProcessor.ProcessedCouponsIndexFilePath = os.Getenv("COUPON_FILE2_URL")
	}
	if os.Getenv("COUPON_FILE3_URL") != "" {
		config.CouponProcessor.ProcessedCouponsIndexFilePath = os.Getenv("COUPON_FILE2_URL")
	}
	if os.Getenv("NUM_HOT_COUPONS_IN_CACHE") != "" {
		num, err := strconv.Atoi(os.Getenv("NUM_HOT_COUPONS_IN_CACHE"))
		if err == nil {
			config.CouponProcessor.NumHotCouponsInCache = num
		}
	}

	return &config, nil

}
