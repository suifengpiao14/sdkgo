package sdkgolib

import (
	"context"
	"encoding/json"

	"github.com/suifengpiao14/stream"
)

func MakeUnmarshalFn(dst interface{}) (fn stream.HandlerFn) {
	return func(ctx context.Context, input []byte) (out []byte, err error) {
		if input == nil {
			return nil, nil
		}
		err = json.Unmarshal([]byte(input), dst)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
}

func MakeUnPackFn() (fn stream.HandlerFn) {
	return func(ctx context.Context, input []byte) (out []byte, err error) {
		return input, nil
	}
}
func MakePackFn() (fn stream.HandlerFn) {
	return func(ctx context.Context, input []byte) (out []byte, err error) {
		return input, nil
	}
}
