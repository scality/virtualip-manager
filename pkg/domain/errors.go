package domain

import (
	"github.com/scality/go-errors"
)

var (
	ErrConfigurationLoading  error = errors.New("Configuration Loading Error")
	ErrTemplating            error = errors.New("Templating Error")
	ErrInputFileReading      error = errors.New("Input File Reading Error")
	ErrInputFileParsing      error = errors.New("Input File Parsing Error")
	ErrMissingInputParameter error = errors.New("Missing Input Parameter Error")
	ErrInvalidInputParameter error = errors.New("Invalid Input Parameter Error")
	ErrOutputFileWriting     error = errors.New("Output File Writing Error")
	ErrInvalidIPAddress      error = errors.New("Invalid IP Address Error")
	ErrInterfaceListing      error = errors.New("Interface Listing Error")
	ErrInterfaceNotFound     error = errors.New("Interface Not Found Error")
)
