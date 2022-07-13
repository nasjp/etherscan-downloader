package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type Config struct {
	Target      string                    `json:"target"`
	ContractDir string                    `json:"contractDir"`
	Contracts   map[string]ConfigContract `json:"contracts"`
}

type ConfigContract struct {
	Chain   chain  `json:"chain"`
	Address string `json:"address"`
}

const (
	ethereum chain = 1
	polygon  chain = 137
)

var blockExploers = map[chain]blockExplorer{
	ethereum: {endpoint: "https://api.etherscan.io/", apiKey: os.Getenv("ETHERSCAN_APIKEY")},
	polygon:  {endpoint: "https://api.polygonscan.com/", apiKey: os.Getenv("POLYGONSCAN_APIKEY")},
}

type chain uint

type blockExplorer struct {
	endpoint string
	apiKey   string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	c, err := loadConfig()
	if err != nil {
		return err
	}

	targetAddress := c.Contracts[c.Target]
	explorer := blockExploers[targetAddress.Chain]

	rawCodes, err := getRawContractCode(explorer.endpoint, targetAddress.Address, explorer.apiKey)
	if err != nil {
		return err
	}

	sourceCodes, err := parseContractCode(rawCodes)
	if err != nil {
		return err
	}

	for _, sourceCode := range sourceCodes {
		for path, source := range sourceCode.Sources {
			if err := os.MkdirAll(targetDir(c.ContractDir, c.Target, path), os.ModePerm); err != nil {
				return err
			}

			f, err := os.Create(targetPath(c.ContractDir, c.Target, path))
			if err != nil {
				return err
			}
			defer f.Close()

			f.Write([]byte(source.Content))
		}
	}

	return nil
}

func loadConfig() (*Config, error) {
	bs, err := os.ReadFile("config.json")
	if err != nil {
		return nil, err
	}

	c := &Config{}

	if err := json.NewDecoder(bytes.NewBuffer(bs)).Decode(c); err != nil {
		return nil, err
	}

	return c, err
}

func getContractURL(endpoint string, address string, apikey string) string {
	const url = "%s/api?module=contract&action=getsourcecode&address=%s&apikey=%s"
	return fmt.Sprintf(url, endpoint, address, apikey)
}

func targetDir(rootDir string, dir string, path string) string {
	return filepath.Dir(targetPath(rootDir, dir, path))
}

func targetPath(rootDir string, dir string, path string) string {
	return filepath.Join(rootDir, dir, path)
}

func getRawContractCode(endpoint, address string, apiKey string) ([]*RawCode, error) {
	url := getContractURL(endpoint, address, apiKey)
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, err
	}

	contractCodeResponse := &Response{}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.NewDecoder(bytes.NewBuffer(bs)).Decode(contractCodeResponse); err != nil {
		return []*RawCode{{SourceCode: string(bs), IsOneSource: true}}, nil
	}

	if contractCodeResponse.Status != "1" {
		return nil, fmt.Errorf("bad status: %s, message: %s", contractCodeResponse.Status, contractCodeResponse.Status)
	}

	return contractCodeResponse.Codes, nil
}

func parseContractCode(rawCodes []*RawCode) ([]*SourceCode, error) {
	sourceCodes := make([]*SourceCode, 0, len(rawCodes))
	if len(rawCodes) == 1 && rawCodes[0].IsOneSource {
		return []*SourceCode{{
			Sources: Sources{"main.sol": &Contract{Content: rawCodes[0].SourceCode}},
		}}, nil
	}

	for _, rawCode := range rawCodes {
		sourceCode := &SourceCode{}
		if err := json.Unmarshal([]byte(rawCode.SourceCode[1:len(rawCode.SourceCode)-1]), sourceCode); err != nil {
			return []*SourceCode{{
				Sources: Sources{"main.sol": &Contract{Content: rawCodes[0].SourceCode}},
			}}, nil
		}

		sourceCodes = append(sourceCodes, sourceCode)
	}

	return sourceCodes, nil
}

type Response struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Codes   []*RawCode `json:"result"`
}

type RawCode struct {
	SourceCode           string `json:"SourceCode"`
	Abi                  string `json:"ABI"`
	ContractName         string `json:"ContractName"`
	CompilerVersion      string `json:"CompilerVersion"`
	OptimizationUsed     string `json:"OptimizationUsed"`
	Runs                 string `json:"Runs"`
	ConstructorArguments string `json:"ConstructorArguments"`
	EVMVersion           string `json:"EVMVersion"`
	Library              string `json:"Library"`
	LicenseType          string `json:"LicenseType"`
	Proxy                string `json:"Proxy"`
	Implementation       string `json:"Implementation"`
	SwarmSource          string `json:"SwarmSource"`
	IsOneSource          bool
}

// SourceCodeFields
type SourceCode struct {
	Language string   `json:"language"`
	Sources  Sources  `json:"sources"`
	Settings Settings `json:"settings"`
}

type Sources map[string]*Contract

type Contract struct {
	Content string `json:"content"`
}

type Settings struct {
	Optimizer       *Optimizer      `json:"optimizer"`
	OutputSelection OutputSelection `json:"outputSelection"`
	Libraries       Libraries       `json:"libraries"`
}

type Optimizer struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

type OutputSelection map[string]map[string][]string

type Libraries interface{}
