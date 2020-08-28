package functional

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/integr8ly/integreatly-operator/test/metadata"
	"github.com/jstemmer/go-junit-report/formatter"
	"github.com/jstemmer/go-junit-report/parser"
)

const (
	testResultsDirectory = "/test-run-results"
	jUnitOutputFilename  = "junit-integreatly-operator.xml"
	addonMetadataName    = "addon-metadata.json"
	testOutputFileName   = "test-output.txt"
	testSuiteName        = "integreatly-operator"
)

func teeOutput(f func()) string {

	var output bytes.Buffer

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	originalStdout := os.Stdout
	os.Stdout = stdoutWriter
	defer func() {
		os.Stdout = originalStdout
	}()

	originalStderr := os.Stderr
	os.Stderr = stderrWriter
	defer func() {
		os.Stderr = originalStderr
	}()

	var wg sync.WaitGroup

	// this function will keep reading
	// from the piped stdout/stderr and write
	// to the original stdout/stderr
	t := func(r, w *os.File) {
		buf := make([]byte, 4096)
		for true {
			l, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			if l == 0 {
				break
			}

			s := string(buf[:l])
			lines := strings.Split(s, "\n")
			out := ""
			for _, l := range lines {

				trimmed := strings.TrimSpace(l)
				if strings.HasPrefix(trimmed, "--- PASS") ||
					strings.HasPrefix(trimmed, "PASS") ||
					strings.HasPrefix(trimmed, "--- FAIL") ||
					strings.HasPrefix(trimmed, "--- SKIP") ||
					strings.HasPrefix(trimmed, "=== CONT") ||
					strings.HasPrefix(trimmed, "=== RUN") {
					out += l + "\n"
				} else if trimmed != "" {
					t := time.Now().UTC()
					formatted := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
						t.Year(), t.Month(), t.Day(),
						t.Hour(), t.Minute(), t.Second())
					out += fmt.Sprintf("[%v] %v\n", formatted, l)
				}

			}
			outBytes := []byte(out)

			_, err = w.Write(outBytes)
			if err != nil {
				panic(err)
			}

			_, err = output.Write(outBytes)
			if err != nil {
				break
			}
		}

		wg.Done()
	}

	wg.Add(2)

	go t(stdoutReader, originalStdout)
	go t(stderrReader, originalStderr)

	f()

	err = stdoutWriter.Close()
	if err != nil {
		panic(err)
	}

	err = stderrWriter.Close()
	if err != nil {
		panic(err)
	}

	wg.Wait()

	err = stdoutReader.Close()
	if err != nil {
		panic(err)
	}

	err = stderrReader.Close()
	if err != nil {
		panic(err)
	}

	return output.String()
}

func writeOutputToFile(output string, filepath string) error {
	return ioutil.WriteFile(filepath, []byte(output), os.FileMode(0644))
}

func writeJunitReportFile(output string, junitReportPath string) error {
	report, err := parser.Parse(strings.NewReader(output), testSuiteName)
	if err != nil {
		return err
	}

	file, err := os.Create(junitReportPath)
	if err != nil {
		return err
	}

	defer file.Close()

	err = formatter.JUnitReportXML(report, false, "", file)
	if err != nil {
		return err
	}
	return nil
}

func TestMain(t *testing.M) {
	exitCode := 0

	output := teeOutput(func() {
		exitCode = t.Run()
	})

	if _, err := os.Stat(testResultsDirectory); !os.IsNotExist(err) {
		err := writeOutputToFile(output, filepath.Join(testResultsDirectory, testOutputFileName))
		if err != nil {
			fmt.Printf("error while writing the test output: %v", err)
			os.Exit(1)
		}

		err = writeJunitReportFile(output, filepath.Join(testResultsDirectory, jUnitOutputFilename))
		if err != nil {
			fmt.Printf("error while writing the junit report file: %v", err)
			os.Exit(1)
		}

		err = metadata.Instance.WriteToJSON(filepath.Join(testResultsDirectory, addonMetadataName))
		if err != nil {
			fmt.Printf("error while writing metadata: %v", err)
			os.Exit(1)
		}
	}

	os.Exit(exitCode)
}
