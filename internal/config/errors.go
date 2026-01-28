package config

import "errors"

var ErrFileDoesNotExist = errors.New("config file does not exist")
var ErrReadConfigFail = errors.New("failed to read config file")
var ErrConfigParsingFail = errors.New("failed to parse config file")
var ErrInvalidConfig = errors.New("Invalid config file")
