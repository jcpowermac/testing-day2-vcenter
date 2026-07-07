package main

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type JUnitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure"`
	Skipped   *JUnitSkipped `xml:"skipped"`
	Error     *JUnitError   `xml:"error"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type JUnitSkipped struct {
	Message string `xml:"message,attr"`
}

type JUnitError struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type Result int

const (
	ResultMissing Result = iota
	ResultPass
	ResultFail
	ResultSkip
	ResultError
)

func (r Result) String() string {
	switch r {
	case ResultPass:
		return "PASS"
	case ResultFail:
		return "FAIL"
	case ResultSkip:
		return "SKIP"
	case ResultError:
		return "ERR"
	default:
		return "-"
	}
}

func (r Result) Emoji() string {
	switch r {
	case ResultPass:
		return "&#x2705;"
	case ResultFail:
		return "&#x274C;"
	case ResultSkip:
		return "&#x23ED;"
	case ResultError:
		return "&#x26A0;"
	default:
		return "&#x2796;"
	}
}

func (r Result) CSSClass() string {
	switch r {
	case ResultPass:
		return "pass"
	case ResultFail:
		return "fail"
	case ResultSkip:
		return "skip"
	case ResultError:
		return "error"
	default:
		return "missing"
	}
}

type GridData struct {
	Runs      []string
	Tests     []string
	Matrix    map[string]map[string]Result
	PassRates map[string]float64
	RunTotals map[string]RunSummary
}

type RunSummary struct {
	Pass    int
	Fail    int
	Skip    int
	Error   int
	Missing int
	Total   int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <results-dir>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Reads results/run-*/*.xml JUnit files and generates a test grid.\n")
		os.Exit(1)
	}
	resultsDir := os.Args[1]

	grid, err := buildGrid(resultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(grid.Tests) == 0 {
		fmt.Fprintf(os.Stderr, "No test results found in %s\n", resultsDir)
		os.Exit(1)
	}

	printMarkdownGrid(grid)

	htmlPath := filepath.Join(resultsDir, "grid.html")
	if err := writeHTMLGrid(grid, htmlPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write HTML grid: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "\nHTML grid written to %s\n", htmlPath)
	}
}

func buildGrid(resultsDir string) (*GridData, error) {
	runDirs, err := filepath.Glob(filepath.Join(resultsDir, "run-*"))
	if err != nil {
		return nil, fmt.Errorf("glob run dirs: %w", err)
	}
	if len(runDirs) == 0 {
		return nil, fmt.Errorf("no run-* directories found in %s", resultsDir)
	}
	sort.Strings(runDirs)

	grid := &GridData{
		Matrix:    make(map[string]map[string]Result),
		PassRates: make(map[string]float64),
		RunTotals: make(map[string]RunSummary),
	}

	testSet := make(map[string]bool)

	for _, runDir := range runDirs {
		runName := filepath.Base(runDir)
		grid.Runs = append(grid.Runs, runName)

		xmlFiles, err := filepath.Glob(filepath.Join(runDir, "*.xml"))
		if err != nil {
			continue
		}

		for _, xmlFile := range xmlFiles {
			cases, err := parseJUnitFile(xmlFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", xmlFile, err)
				continue
			}

			for _, tc := range cases {
				testName := tc.Name
				testSet[testName] = true

				if grid.Matrix[testName] == nil {
					grid.Matrix[testName] = make(map[string]Result)
				}

				result := classifyResult(tc)
				existing, exists := grid.Matrix[testName][runName]
				if !exists || result > existing {
					grid.Matrix[testName][runName] = result
				}
			}
		}
	}

	for name := range testSet {
		grid.Tests = append(grid.Tests, name)
	}
	sort.Strings(grid.Tests)

	for _, test := range grid.Tests {
		var passed, total int
		for _, run := range grid.Runs {
			r := grid.Matrix[test][run]
			if r == ResultPass || r == ResultFail || r == ResultError {
				total++
				if r == ResultPass {
					passed++
				}
			}
		}
		if total > 0 {
			grid.PassRates[test] = float64(passed) / float64(total) * 100
		}
	}

	for _, run := range grid.Runs {
		summary := RunSummary{}
		for _, test := range grid.Tests {
			summary.Total++
			switch grid.Matrix[test][run] {
			case ResultPass:
				summary.Pass++
			case ResultFail:
				summary.Fail++
			case ResultSkip:
				summary.Skip++
			case ResultError:
				summary.Error++
			default:
				summary.Missing++
			}
		}
		grid.RunTotals[run] = summary
	}

	return grid, nil
}

func parseJUnitFile(path string) ([]JUnitTestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		var suite JUnitTestSuite
		if err2 := xml.Unmarshal(data, &suite); err2 != nil {
			return nil, fmt.Errorf("parse XML: %w", err)
		}
		suites.Suites = []JUnitTestSuite{suite}
	}

	var cases []JUnitTestCase
	for _, suite := range suites.Suites {
		cases = append(cases, suite.TestCases...)
	}
	return cases, nil
}

func classifyResult(tc JUnitTestCase) Result {
	if tc.Error != nil {
		return ResultError
	}
	if tc.Failure != nil {
		return ResultFail
	}
	if tc.Skipped != nil {
		return ResultSkip
	}
	return ResultPass
}

func printMarkdownGrid(grid *GridData) {
	maxNameLen := 40
	for _, t := range grid.Tests {
		name := shortName(t)
		if len(name) > maxNameLen {
			maxNameLen = len(name)
		}
	}
	if maxNameLen > 70 {
		maxNameLen = 70
	}

	fmt.Printf("| %-*s |", maxNameLen, "Test Case")
	for _, run := range grid.Runs {
		fmt.Printf(" %-6s |", run[len("run-"):])
	}
	fmt.Printf(" Rate   |\n")

	fmt.Printf("|-%s-|", strings.Repeat("-", maxNameLen))
	for range grid.Runs {
		fmt.Print("--------|")
	}
	fmt.Print("--------|\n")

	for _, test := range grid.Tests {
		name := shortName(test)
		if len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}
		fmt.Printf("| %-*s |", maxNameLen, name)
		for _, run := range grid.Runs {
			r := grid.Matrix[test][run]
			fmt.Printf(" %-6s |", r.String())
		}
		fmt.Printf(" %5.1f%% |\n", grid.PassRates[test])
	}

	fmt.Printf("|-%s-|", strings.Repeat("-", maxNameLen))
	for range grid.Runs {
		fmt.Print("--------|")
	}
	fmt.Print("--------|\n")

	fmt.Printf("| %-*s |", maxNameLen, "TOTALS (pass/fail/skip)")
	for _, run := range grid.Runs {
		s := grid.RunTotals[run]
		fmt.Printf(" %d/%d/%d |", s.Pass, s.Fail, s.Skip)
	}
	fmt.Printf("        |\n")
}

func shortName(fullName string) string {
	parts := strings.SplitN(fullName, " ", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[0], "[") {
		return parts[1]
	}
	return fullName
}

func rateClass(rate float64) string {
	switch {
	case rate >= 100:
		return "rate-100"
	case rate >= 80:
		return "rate-high"
	case rate >= 50:
		return "rate-med"
	default:
		return "rate-low"
	}
}

func writeHTMLGrid(grid *GridData, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl := template.Must(template.New("grid").Funcs(template.FuncMap{
		"shortName": shortName,
		"rateClass": rateClass,
	}).Parse(htmlSource))

	return tmpl.Execute(f, grid)
}

const htmlSource = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Test Grid Results</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; margin: 2rem; background: #fff; color: #1a1a1a; }
  h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
  .meta { color: #666; margin-bottom: 1.5rem; }
  table { border-collapse: collapse; font-size: 0.85rem; }
  th, td { border: 1px solid #d0d7de; padding: 6px 10px; text-align: center; white-space: nowrap; }
  th { background: #f6f8fa; font-weight: 600; }
  th.test-name, td.test-name { text-align: left; max-width: 500px; overflow: hidden; text-overflow: ellipsis; }
  td.pass { background: #dafbe1; }
  td.fail { background: #ffcecb; }
  td.skip { background: #fff8c5; }
  td.error { background: #ffcecb; }
  td.missing { background: #f6f8fa; color: #999; }
  .rate-100 { font-weight: 600; color: #1a7f37; }
  .rate-high { color: #1a7f37; }
  .rate-med { color: #9a6700; }
  .rate-low { color: #cf222e; }
  .summary { margin-top: 1.5rem; }
  .summary td { font-weight: 600; }
</style>
</head>
<body>
<h1>vSphere Multi-vCenter Day 2 — Test Grid</h1>
<p class="meta">{{len .Runs}} runs, {{len .Tests}} test cases</p>
<table>
<thead>
  <tr>
    <th class="test-name">Test Case</th>
    {{range .Runs}}<th>{{slice . 4}}</th>{{end}}
    <th>Pass Rate</th>
  </tr>
</thead>
<tbody>
  {{range $test := .Tests}}
  <tr>
    <td class="test-name" title="{{$test}}">{{shortName $test}}</td>
    {{range $run := $.Runs -}}
    {{- $results := index $.Matrix $test -}}
    {{- $r := index $results $run -}}
    <td class="{{$r.CSSClass}}">{{$r.Emoji}}</td>
    {{end}}
    <td class="{{rateClass (index $.PassRates $test)}}">{{printf "%.0f%%" (index $.PassRates $test)}}</td>
  </tr>
  {{end}}
</tbody>
<tfoot class="summary">
  <tr>
    <td class="test-name">Totals</td>
    {{range $run := .Runs -}}
    {{- $s := index $.RunTotals $run -}}
    <td>{{$s.Pass}}/{{$s.Fail}}/{{$s.Skip}}</td>
    {{end}}
    <td></td>
  </tr>
</tfoot>
</table>
</body>
</html>
`
