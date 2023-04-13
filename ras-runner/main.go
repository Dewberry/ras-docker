package main

import (
	"app/runners"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

	// // Local dev only
	// err = godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

	if len(os.Args) != 2 {
		log.Fatal("no inputs provided, program requires `inputs: {Params}`")
	}

	// fmt.Println(os.Args[1])
	// Fetch inputs
	p, err := FetchParams(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	r = &runners.OGCRunner{PayloadFile: p.S3key, LocalDir: MODEL_DIR, Bucket: os.Getenv("AWS_BUCKET")}

	err = r.PrepRun()
	if err != nil {
		log.Fatal(err.Error())
	}

	modelName, err := r.ModelName()
	if err != nil {
		log.Fatal(err.Error())
	}
	logFile := filepath.Join(MODEL_DIR, modelName+".log")
	logOutput, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer logOutput.Close()

	// Print to terminal and log file for dev
	mw := io.MultiWriter(os.Stdout, logOutput)
	log.SetOutput(mw)

	fmt.Println("Running model....")
	err = r.Run()
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println("Pushing results......")
	err = r.CopyOutputs()
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println("Done")
}

type Inputs struct {
	Inputs Params `json:"inputs"`
}

type Params struct {
	S3key string `json:"s3key"`
}

func FetchParams(inputString string) (Params, error) {
	var params Params
	if inputString == "" {
		return params, fmt.Errorf("WaterhsedID and additional params required")
	}

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
