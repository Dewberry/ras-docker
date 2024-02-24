package runners

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"bytes"

	plug "github.com/Dewberry/papigoplug/papigoplug"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-errors/errors"
)

type OGCRunner struct {
	Bucket      string  `json:"bucket"`
	PayloadFile string  `json:"payload_file"`
	LocalDir    string  `json:"local_dir"`
	Payload     Payload `json:"payload"`
}

type Payload struct {
	Inputs  []Inputs  `json:"inputs"`
	Outputs []Outputs `json:"outputs"`
}

type Inputs struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

type Outputs struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

func (r *OGCRunner) ModelName() (modelName string, err error) {
	// Ignoring extensions, ensure input file names all start with the same string (up to first period), then return that.
	var candidate string
	for i, link := range r.Payload.Inputs {
		candidate = strings.Split(filepath.Base(link.Href), ".")[0]
		if i == 0 {
			modelName = candidate
		} else if modelName != candidate {
			err = fmt.Errorf("inputs do not all resolve to same modelName (%s vs %s)", modelName, candidate)
			return
		}
	}
	return
}


// pluginResults is used to format the JSON string printed at the end of the program.
type pluginResults struct {
	Results []map[string]interface{} `json:"plugin_results"`
}

// printResults encodes the provided map into a JSON string within key "plugin_results" and prints that.
// This function must be called at the end of the program.
func printResults(results []map[string]interface{}) (err error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // Do not escape HTML characters

	if err = encoder.Encode(pluginResults{Results: results}); err != nil {
		return
	}

	// Use strings.TrimSpace to remove the trailing newline added by Encode.
	fmt.Println(buf.String())
	return
}

func (r *OGCRunner) PrintResults() error {
    var outputsMap []map[string]interface{}
    for _, output := range r.Payload.Outputs {
        outputsMap = append(outputsMap, map[string]interface{}{
            "href": output.Href,
            "rel":  output.Rel,
        })
    }
    printResults(outputsMap)
	return nil
}

func (r *OGCRunner) GeomID() (geomID string, err error) {
	// Scan the Payload's input file list, and return the 2-digit suffix associated with the .cNN file
	var pattern string = `\.c(\d{2})$` // e.g. extract "03" from "foobar.c03"
	var rgx = regexp.MustCompile(pattern)
	for _, link := range r.Payload.Inputs {
		matches := rgx.FindStringSubmatch(link.Href)
		if len(matches) != 2 {
			continue
		}
		if geomID == "" {
			// first .cNN file found
			geomID = matches[1]
		} else {
			// found another .cNN file
			err = fmt.Errorf("multiple files in payload inputs matched pattern %q", pattern)
			return
		}
	}
	if geomID == "" {
		err = fmt.Errorf("no file in payload inputs matched pattern %q", pattern)
	}
	return
}

func (r *OGCRunner) UnsteadyID() (unsteadyID string, err error) {
	// Scan the Payload's input file list, and return the 2-digit suffix associated with the .bNN file
	var pattern string = `\.b(\d{2})$` // e.g. extract "03" from "foobar.b03"
	var rgx = regexp.MustCompile(pattern)
	for _, link := range r.Payload.Inputs {
		matches := rgx.FindStringSubmatch(link.Href)
		if len(matches) != 2 {
			continue
		}
		if unsteadyID == "" {
			// first .bNN file found
			unsteadyID = matches[1]
		} else {
			// found another .bNN file
			err = fmt.Errorf("multiple files in payload inputs matched pattern %q", pattern)
			return
		}
	}
	if unsteadyID == "" {
		err = fmt.Errorf("no file in payload inputs matched pattern %q", pattern)
	}
	return
}

func (r *OGCRunner) PrepRun() error {

	err := r.fetchPayload()
	if err != nil {
		return err
	}

	localFiles := make([]string, len(r.Payload.Inputs))
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})
	if err != nil {
		return err
	}

	svc := s3.New(sess)

	for i, link := range r.Payload.Inputs {
		input := &s3.GetObjectInput{
			Bucket: aws.String(r.Bucket),
			Key:    aws.String(link.Href),
		}

		obj, err := svc.GetObject(input)
		if err != nil {
			plug.Log.Infof("S3 Fetch Error |  %s %s", link.Href, err)
			return err
		}
		defer obj.Body.Close()

		fileName := filepath.Base(link.Href)
		localFile := filepath.Join(r.LocalDir, fileName)

		f, err := os.OpenFile(localFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			plug.Log.Errorf("Open File Error %s", err)
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, obj.Body)
		if err != nil {
			plug.Log.Errorf("Write File Error %s", err)
			return err
		}

		localFiles[i] = localFile

		msg := fmt.Sprintf("downloaded s3://%s/%s to %s", r.Bucket, link.Href, localFile)
		plug.Log.Infof(msg)

	}

	return nil
}

func (r *OGCRunner) Run() error {
	modelName, err := r.ModelName()
	if err != nil {
		return err
	}

	geomID, err := r.GeomID()
	if err != nil {
		return err
	}

	unsteadyID, err := r.UnsteadyID()
	if err != nil {
		return err
	}

	cmd := exec.Command("/app/run-model.sh", modelName, geomID, unsteadyID)
	cmd.Dir = r.LocalDir
	msg := fmt.Sprintf("running model from directory '%s' with args: [ %s ]", cmd.Dir, strings.Join(cmd.Args, ", "))
	plug.Log.Debug(msg)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	in := bufio.NewScanner(stdout)

	// Logging placeholder
	startLogging := 0
	checkValues := map[string]float64{
		"10%":  0.1,
		"20%":  0.2,
		"30%":  0.3,
		"40%":  0.4,
		"50%":  0.5,
		"60%":  0.6,
		"70%":  0.7,
		"80%":  0.8,
		"90%":  0.9,
		"100%": 1.0,
	}

	computeLog := filepath.Join(r.LocalDir, modelName+".log")
	f, err := os.OpenFile(computeLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	for in.Scan() {
		message := in.Text()
		rasPctLog(&startLogging, message, &checkValues)
		_, err := f.WriteString(message + "\n")
		if err != nil {
			return err
		}
	}

	if err := in.Err(); err != nil {
		return err
	}

	// extract stderr messages
	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return err
	}
	// check if the command failed
	err = cmd.Wait()
	if err != nil {
		// exitError := err.(*exec.ExitError)
		// fmt.Printf("command exited with non-zero code: %d\n", exitError.ExitCode())
		plug.Log.Errorf("vvvvv below is stderr from failing command\n%s\n^^^^^ above is stderr from failing command\n", stderrBytes)
		return err
	}

	// Rename the output p*/hdf following successful sim
	err = filepath.Walk(r.LocalDir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".tmp") {
			err := os.Rename(path, strings.Replace(path, ".tmp", "", 1))
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *OGCRunner) CopyOutputs() error {

	sess, err := session.NewSession()
	if err != nil {
		return err
	}
	svc := s3.New(sess)

	for _, link := range r.Payload.Outputs {

		fileName := filepath.Base(link.Href)
		localFile := filepath.Join(r.LocalDir, fileName)

		file, err := os.Open(localFile)
		if err != nil {
			return err
		}

		fileInfo, err := file.Stat()
		if err != nil {
			return err
		}
		size := fileInfo.Size()

		contentType := mime.TypeByExtension(filepath.Ext(localFile))
		if contentType == "" {
			if filepath.Ext(localFile) == ".log" { // this one is not caught for some reason
				contentType = "text/plain"
			} else {
				contentType = "application/octet-stream"
			}
		}

		_, err = svc.PutObject(&s3.PutObjectInput{
			Bucket:        aws.String(r.Bucket),
			Key:           aws.String(link.Href),
			Body:          file,
			ContentLength: aws.Int64(size),
			ContentType:   aws.String(contentType),
		})
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("uploaded %s to s3://%s/%s", localFile, r.Bucket, link.Href)
		plug.Log.Infof(msg)

	}
	return nil
}

func (r *OGCRunner) fetchPayload() error {
	var payload Payload

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})
	if err != nil {
		return err
	}

	svc := s3.New(sess)
	input := &s3.GetObjectInput{
		Bucket: aws.String(r.Bucket),
		Key:    aws.String(r.PayloadFile),
	}

	obj, err := svc.GetObject(input)
	if err != nil {
		return errors.Errorf("input error for bucket %s payloadFile %s", r.Bucket, r.PayloadFile)
	}
	defer obj.Body.Close()

	body, err := io.ReadAll(obj.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &payload)
	if err != nil {
		return err
	}

	r.Payload = payload

	return nil
}
