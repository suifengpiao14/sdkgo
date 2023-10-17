package sdkgolib

type Config struct {
	Host          string `mapstructure:"url" json:"url" validate:"required"`
	CallerID      string `mapstructure:"callerId" json:"callerId"  validate:"required"`
	CallerAppName string `mapstructure:"callerAppName" json:"callerAppName"  validate:"required"`
}
