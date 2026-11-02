// Command bootstrap is the AWS Lambda handler that runs colbinbench on
// Lambda's own CPUs: Graviton for the arm64 function, Intel or AMD for the
// x86_64 one. lambda/deploy.py builds it, packages it with colbinbench and
// creates the two functions; see lambda/README.md.
//
// It speaks Lambda's runtime API directly (provided.al2023). Each function
// serves one corpus, named by $CORPUS (tpch, tpcds, publicbi or schemastore), whose
// data a layer carries as one zstd-compressed tar; the first invocation of an
// instance unpacks it to /tmp/data with klauspost/compress, which needs no cgo.
// Each invocation runs colbinbench's -worker mode
// once per case and variant, one process at a time with GOMAXPROCS=1, and
// returns the output in benchmark format, one text per variant, for benchstat.
//
// The variants are builds of the same colbinbench that differ only in which
// colbin kernels they compile: the assembly (asm, the default build) and the
// scalar loops (purego). Only
// asm is what a caller of colbin gets, so it is the only one that counts and
// the only one run unless an event names the others.
package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/ivanjoz/colbin-benchmarks/matrix"
)

// request is an invocation's event. Every field has a default, so {} runs the
// quick comparison.
type request struct {
	// List returns the corpus's case names and runs nothing.
	List     bool     `json:"list"`
	Cases    []string `json:"cases"`
	Variants []string `json:"variants"`
	Formats  string   `json:"formats"`
	Ops      string   `json:"ops"`
	Count    int      `json:"count"`
	Sample   string   `json:"sample"`
}

// response is what an invocation returns.
type response struct {
	Arch     string            `json:"arch"`
	CPU      string            `json:"cpu"`
	Seconds  float64           `json:"seconds"`
	Variants map[string]string `json:"variants"`
	Cases    []string          `json:"cases,omitempty"`
}

// corpusCases is the timed cases of the corpus this function serves, in the
// matrix's report order.
func corpusCases() []string {
	corpus := os.Getenv("CORPUS")
	var names []string
	for _, c := range matrix.Cases() {
		if c.Dataset == corpus && c.Timed {
			names = append(names, c.Name)
		}
	}
	return names
}

func main() {
	api := "http://" + os.Getenv("AWS_LAMBDA_RUNTIME_API") + "/2018-06-01/runtime/invocation/"
	for {
		next, err := http.Get(api + "next")
		if err != nil {
			fatal(err)
		}
		id := next.Header.Get("Lambda-Runtime-Aws-Request-Id")
		event, err := io.ReadAll(next.Body)
		next.Body.Close()
		if err != nil {
			fatal(err)
		}
		result, err := handle(event)
		if err != nil {
			body, _ := json.Marshal(map[string]string{"errorMessage": err.Error(), "errorType": "BenchError"})
			post(api+id+"/error", body)
			continue
		}
		body, err := json.Marshal(result)
		if err != nil {
			fatal(err)
		}
		post(api+id+"/response", body)
	}
}

func handle(event []byte) (*response, error) {
	var req request
	if len(bytes.TrimSpace(event)) > 0 {
		if err := json.Unmarshal(event, &req); err != nil {
			return nil, fmt.Errorf("event: %w", err)
		}
	}
	if req.List {
		return &response{Arch: runtime.GOARCH, CPU: cpuName(), Cases: corpusCases()}, nil
	}
	if err := unpackData(); err != nil {
		return nil, fmt.Errorf("data: %w", err)
	}
	if len(req.Cases) == 0 {
		req.Cases = corpusCases()
	}
	if len(req.Variants) == 0 {
		req.Variants = []string{"asm"}
	}
	if req.Formats == "" {
		req.Formats = "colbin,msgpack,vtprotobuf"
	}
	if req.Ops == "" {
		req.Ops = "encode,decode-fresh,decode-reuse"
	}
	if req.Count == 0 {
		req.Count = 5
	}
	if req.Sample == "" {
		req.Sample = "100ms"
	}

	start := time.Now()
	cpu := cpuName()
	result := &response{Arch: runtime.GOARCH, CPU: cpu, Variants: map[string]string{}}
	outputs := map[string]*bytes.Buffer{}
	for _, variant := range req.Variants {
		outputs[variant] = bytes.NewBufferString(fmt.Sprintf(
			"goos: linux\ngoarch: %s\npkg: colbinbench\ncpu: %s\n", runtime.GOARCH, cpu))
	}
	// Case by case, every variant in turn, so that whatever the host does
	// meanwhile falls on all of them alike.
	for _, name := range req.Cases {
		for _, variant := range req.Variants {
			binary := filepath.Join(taskRoot(), "colbinbench-"+variant)
			command := exec.Command(binary, "-worker", name,
				"-formats", req.Formats, "-ops", req.Ops,
				"-count", strconv.Itoa(req.Count), "-sample", req.Sample,
				"-data", dataDir())
			command.Env = append(os.Environ(), "GOMAXPROCS=1")
			var stderr bytes.Buffer
			command.Stdout, command.Stderr = outputs[variant], &stderr
			if err := command.Run(); err != nil {
				return nil, fmt.Errorf("%s %s: %w: %s", variant, name, err, stderr.String())
			}
		}
	}
	for variant, out := range outputs {
		result.Variants[variant] = out.String()
	}
	result.Seconds = time.Since(start).Seconds()
	return result, nil
}

// taskRoot is where Lambda unpacked the function's code.
func taskRoot() string {
	if root := os.Getenv("LAMBDA_TASK_ROOT"); root != "" {
		return root
	}
	return "/var/task"
}

// dataDir is where the corpus is unpacked, or $COLBIN_DATA when bootstrap runs
// outside Lambda.
func dataDir() string {
	if dir := os.Getenv("COLBIN_DATA"); dir != "" {
		return dir
	}
	return "/tmp/data"
}

var (
	unpackOnce sync.Once
	unpackErr  error
)

// unpackData unpacks the layer's /opt/data.tar.zst into dataDir once per
// instance. Outside Lambda, with $COLBIN_DATA set, the data is already there.
func unpackData() error {
	unpackOnce.Do(func() {
		if os.Getenv("COLBIN_DATA") != "" {
			return
		}
		unpackErr = untarZstd("/opt/data.tar.zst", dataDir())
	})
	return unpackErr
}

func untarZstd(archive, dest string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder, err := zstd.NewReader(file)
	if err != nil {
		return err
	}
	defer decoder.Close()
	reader := tar.NewReader(decoder)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		path := filepath.Join(dest, filepath.Clean("/"+header.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, reader); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
}

// cpuName is the host's CPU as /proc/cpuinfo names it: a model name on x86, a
// part number on arm64 (0xd0c is Neoverse N1, Graviton2's core).
func cpuName() string {
	info, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	var implementer, part string
	for line := range strings.Lines(string(info)) {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "model name":
			return value
		case "CPU implementer":
			implementer = value
		case "CPU part":
			part = value
		}
	}
	return "implementer " + implementer + " part " + part
}

func post(url string, body []byte) {
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fatal(err)
	}
	resp.Body.Close()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "bootstrap:", err)
	os.Exit(1)
}
