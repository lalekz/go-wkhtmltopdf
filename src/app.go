package main

import (
	"crypto/tls"
	"encoding/json"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"io/ioutil"
	"os"
	"bytes"
	"time"
	"io"
	"strconv"
)

func main() {
	const bindAddress = ":80"
	http.HandleFunc("/", requestHandler)
	fmt.Println("Http server listening on", bindAddress)
	http.ListenAndServe(bindAddress, nil)
}

type documentRequest struct {
	Contents string
	Output string
	UploadUrl string `json:"upload_url"`
	// TODO: whitelist options that can be passed to avoid errors,
	// log warning when different options get passed
	Options map[string]interface{}
	Cookies map[string]string
}

func logOutput(request *http.Request, message string) {
	ip := strings.Split(request.RemoteAddr, ":")[0]
	fmt.Println(ip, request.Method, request.URL, message)
}

func getEnvInt64(name string, defVal int64) int64 {
	if valueStr, exists := os.LookupEnv(name); exists {
		value, _ := strconv.ParseInt(valueStr, 10, 64)
		return value
	}
	return defVal
}

func getEnvBool(name string, defVal bool) bool {
	if valueStr, exists := os.LookupEnv(name); exists {
		value, _ := strconv.ParseBool(valueStr)
		return value
	}
	return defVal
}

func requestHandler(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		response.WriteHeader(http.StatusNotFound)
		logOutput(request, "404 not found")
		return
	}
	if request.Method != "POST" {
		response.Header().Set("Allow", "POST")
		response.WriteHeader(http.StatusMethodNotAllowed)
		logOutput(request, "405 not allowed")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, getEnvInt64("APP_MAX_BODY_SIZE", 2000000)))
	var req documentRequest
	if err := decoder.Decode(&req); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		logOutput(request, "400 bad request (invalid JSON)")
		return
	}
	segments := make([]string, 0)
	for key, element := range req.Options {
		if element == true {
			// if it was parsed from the JSON as an actual boolean,
			// convert to command-line single argument	(--foo)
			segments = append(segments, fmt.Sprintf("--%v", key))
		} else if element != false {
			// Otherwise, use command-line argument with value (--foo bar)
			segments = append(segments, fmt.Sprintf("--%v", key), fmt.Sprintf("%v", element))
		}
	}
	for key, value := range req.Cookies {
		segments = append(segments, "--cookie", key, url.QueryEscape(value))
	}
	var programFile string
	var contentType string
	switch req.Output {
	case "jpg":
		programFile = "/bin/wkhtmltoimage"
		contentType = "image/jpeg"
		segments = append(segments, "--format", "jpg", "-q")
	case "png":
		programFile = "/bin/wkhtmltoimage"
		contentType = "image/png"
		segments = append(segments, "--format", "png", "-q")
	default:
		// defaults to pdf
		programFile = "/bin/wkhtmltopdf"
		contentType = "application/pdf"
	}
	fmt.Println("\tContents size:", len(req.Contents))
	if len(req.Contents) == 0 {
		response.WriteHeader(http.StatusBadRequest)
		logOutput(request, "400 bad request (invalid JSON)")
		return
	}
	file, _ := ioutil.TempFile("/tmp", "*.html")
	defer os.Remove(file.Name())
	b64decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(req.Contents))
	io.Copy(file, b64decoder)
	file.Close()
	segments = append(segments, file.Name(), "-")
	var buf bytes.Buffer
	fmt.Println("\tRunning:", programFile, strings.Join(segments, " "))
	cmd := exec.Command(programFile, segments...)
	cmd.Stdout = &buf
	cmd.Start()
	timer := time.AfterFunc(time.Duration(getEnvInt64("APP_PROC_TIMEOUT", 60)) * time.Second, func() {
		cmd.Process.Kill()
	})
	err := cmd.Wait()
	timer.Stop()
	if err != nil {
		fmt.Println("\tProcess error:", err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	if req.UploadUrl != "" {
		fmt.Println("\tPUT", req.UploadUrl)
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: getEnvBool("SSL_SKIP_VERIFY", false)},
		}
		client := &http.Client{Transport: tr}
		req, _ := http.NewRequest(http.MethodPut, req.UploadUrl, &buf)
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		response.WriteHeader(resp.StatusCode)
		io.Copy(response, resp.Body)
	} else {
		response.Header().Set("Content-Type", contentType)
		buf.WriteTo(response)
	}
	logOutput(request, "200 OK")
}
