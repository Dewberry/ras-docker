package main

import (
	"app/runners"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	plug "github.com/Dewberry/papigoplug/papigoplug"
	// "github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
)

const (
	RAS_LIB_PATH = "/ras/libs:/ras/libs/mkl:/ras/libs/rhel_8"
	RAS_EXE      = "/ras/v61"
	MODEL_DIR    = "/sim/model"
	SCRIPT       = "/app/run-model.sh"
)

type Runner interface {
	ModelName() (string, error)
	PrepRun() error
	Run() error
	CopyOutputs() error
}

func main() {
	// var runnerType string
	var r Runner
	var err error

	// Set log level
	plug.InitLog("info")
	allowedParams := plug.PluginParams{
		Required: []string{"s3key"},
	}
	params, err := plug.ParseInput(os.Args, allowedParams)
	if err != nil {
		plug.Log.Fatal(err)
	}
	plug.Log.Infof("Params provided: %s", params)

	// // Local dev only
	// err = godotenv.Load(".env")
	// if err != nil {
	// 	plug.Log.Fatal("Error loading .env file")
	// }

	// Fetch inputs
	p, err := FetchParams(os.Args[1])
	if err != nil {
		plug.Log.Fatal(err)
	}

	r = &runners.OGCRunner{PayloadFile: p.S3key, LocalDir: MODEL_DIR, Bucket: os.Getenv("AWS_BUCKET")}

	err = r.PrepRun()
	if err != nil {
		plug.Log.Fatal(err.Error())
	}

	modelName, err := r.ModelName()
	if err != nil {
		plug.Log.Fatal(err.Error())
	}
	plug.Log.Debugf("Model name: %s", modelName)

	logFile := filepath.Join(MODEL_DIR, modelName+".log")
	logOutput, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer logOutput.Close()

	// Print to terminal and log file for dev
	mw := io.MultiWriter(os.Stdout, logOutput)
	log.SetOutput(mw)

	plug.Log.Info("initiating model simulation")
	err = r.Run()
	if err != nil {
		log.Fatal(err.Error())
	}

	plug.Log.Info("pushing model outputs")
	err = r.CopyOutputs()
	if err != nil {
		log.Fatal(err.Error())
	}

	// plug.PrintResults(results)
}

type Inputs struct {
	Inputs Params `json:"inputs"`
}

type Params struct {
	S3key string `json:"s3key"`
}

func FetchParams(inputString string) (Params, error) {
	var params Params
	err := json.Unmarshal([]byte(inputString), &params)
	if err != nil {
		fmt.Println("error unmarshaling input params:", err)
		return params, err
	}

	return params, nil
}

// TODO
func (params Params) Validate() error {
	return nil
}
