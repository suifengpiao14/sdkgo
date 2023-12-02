package sdkgolib

import "encoding/json"

type Config struct {
	Host          string `mapstructure:"url" json:"url" validate:"required"`
	CallerID      string `mapstructure:"callerId" json:"callerId"  validate:"required"`
	CallerAppName string `mapstructure:"callerAppName" json:"callerAppName"  validate:"required"`
}

func (c Config) String() string {
	b, _ := json.Marshal(c)
	s := string(b)
	return s
}
