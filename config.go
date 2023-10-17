package sdkgolib

import (
	"github.com/go-playground/validator"
	"github.com/pkg/errors"
)

type Config struct {
	Host          string `mapstructure:"url" json:"url" validate:"required"`
	CallerID      string `mapstructure:"callerId" json:"callerId"  validate:"required"`
	CallerAppName string `mapstructure:"callerAppName" json:"callerAppName"  validate:"required"`
}

var (
	configInstance *Config

	ERROR_Init_Config = errors.New("use SetConfig init Config")
)

func SetConfig(c *Config) (err error) {
	validate := validator.New()
	err = validate.Struct(c)
	if err != nil {
		return err
	}
	configInstance = c
	return nil
}

func getConfig() (c *Config, err error) {
	if configInstance == nil {
		return nil, ERROR_Init_Config
	}
	return configInstance, nil
}
